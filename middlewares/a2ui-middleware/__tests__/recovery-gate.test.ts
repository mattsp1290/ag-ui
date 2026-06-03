import { describe, it, expect } from "vitest";
import { BaseEvent, EventType, RunAgentInput } from "@ag-ui/client";
import { Observable, firstValueFrom, toArray } from "rxjs";
import { A2UIMiddleware, A2UIActivityType } from "../src/index";
import { AbstractAgent } from "@ag-ui/client";

// Minimal mock agent that replays a fixed event sequence.
class MockAgent extends AbstractAgent {
  constructor(private events: BaseEvent[]) {
    super();
  }
  run(): Observable<BaseEvent> {
    return new Observable((s) => {
      for (const e of this.events) s.next(e);
      s.complete();
    });
  }
}

function input(): RunAgentInput {
  return { threadId: "t", runId: "r", tools: [], context: [], forwardedProps: {}, state: {}, messages: [] };
}
const collect = (o: Observable<BaseEvent>) => firstValueFrom(o.pipe(toArray()));

// Inline JSON-Schema catalog (A2UIInlineCatalogSchema): Row requires children;
// HotelCard requires name + rating.
const CATALOG = {
  catalogId: "https://a2ui.org/demos/dojo/dynamic_catalog.json",
  components: {
    Row: { type: "object", required: ["children"] },
    HotelCard: { type: "object", required: ["name", "rating"] },
  },
};

const ROOT = { id: "root", component: "Row", children: { componentId: "card", path: "/items" } };
const GOOD_CARD = { id: "card", component: "HotelCard", name: { path: "name" }, rating: { path: "rating" } };
const BAD_CARD = { id: "card", component: "HotelCard", name: { path: "name" } }; // missing required `rating`
const DATA = { items: [{ name: "Ritz", rating: 4.8 }] };

function streamRender(components: unknown[]) {
  const args = JSON.stringify({ surfaceId: "hotels", components, data: DATA });
  return [
    { type: EventType.RUN_STARTED, runId: "r", threadId: "t" },
    { type: EventType.TOOL_CALL_START, toolCallId: "tc1", toolCallName: "render_a2ui" },
    { type: EventType.TOOL_CALL_ARGS, toolCallId: "tc1", delta: args },
    { type: EventType.TOOL_CALL_END, toolCallId: "tc1" },
    { type: EventType.RUN_FINISHED, runId: "r", threadId: "t" },
  ] as BaseEvent[];
}

const surfaceSnapshots = (events: BaseEvent[]) =>
  events.filter((e) => e.type === EventType.ACTIVITY_SNAPSHOT && (e as any).activityType === A2UIActivityType);
const recoveryActivities = (events: BaseEvent[]) =>
  events.filter((e) => e.type === EventType.ACTIVITY_SNAPSHOT && (e as any).activityType === "a2ui_recovery");

describe("A2UI middleware — semantic-validation gate (OSS-162)", () => {
  it("suppresses a semantically-invalid streamed component tree (no faulty paint)", async () => {
    const mw = new A2UIMiddleware({ schema: CATALOG });
    const events = await collect(mw.run(input(), new MockAgent(streamRender([ROOT, BAD_CARD]))));
    // No surface painted for the invalid attempt...
    expect(surfaceSnapshots(events)).toHaveLength(0);
    // ...and a recovery "retrying" status is surfaced (client decides when to show it).
    const recovery = recoveryActivities(events);
    expect(recovery.length).toBeGreaterThanOrEqual(1);
    expect((recovery[0] as any).content.status).toBe("retrying");
  });

  it("emits a surface for a valid streamed tree (existing behavior preserved)", async () => {
    const mw = new A2UIMiddleware({ schema: CATALOG });
    const events = await collect(mw.run(input(), new MockAgent(streamRender([ROOT, GOOD_CARD]))));
    const snaps = surfaceSnapshots(events);
    expect(snaps.length).toBeGreaterThanOrEqual(1);
    expect((snaps[0] as any).content.a2ui_operations.length).toBeGreaterThanOrEqual(2);
    expect(recoveryActivities(events)).toHaveLength(0);
  });

  it("does NOT over-suppress when no catalog is configured (structural-only)", async () => {
    // No `schema` → catalog checks skipped; an unknown component type still paints.
    const mw = new A2UIMiddleware();
    const unknown = [{ id: "root", component: "MysteryCard", children: { componentId: "card", path: "/items" } }, { id: "card", component: "MysteryCard", name: { path: "name" } }];
    const events = await collect(mw.run(input(), new MockAgent(streamRender(unknown))));
    expect(surfaceSnapshots(events).length).toBeGreaterThanOrEqual(1);
  });

  it("emits a hard-failure recovery activity when the tool result is an exhausted envelope", async () => {
    const mw = new A2UIMiddleware({ schema: CATALOG });
    const errorEnvelope = JSON.stringify({ error: "Failed to generate valid A2UI after 3 attempt(s)", code: "a2ui_recovery_exhausted", attempts: [{ attempt: 1, ok: false }] });
    const events = await collect(
      mw.run(
        input(),
        new MockAgent([
          { type: EventType.RUN_STARTED, runId: "r", threadId: "t" },
          { type: EventType.TOOL_CALL_START, toolCallId: "outer1", toolCallName: "generate_a2ui" },
          { type: EventType.TOOL_CALL_ARGS, toolCallId: "outer1", delta: '{"intent":"create"}' },
          { type: EventType.TOOL_CALL_END, toolCallId: "outer1" },
          { type: EventType.TOOL_CALL_RESULT, messageId: "m1", toolCallId: "outer1", content: errorEnvelope } as BaseEvent,
          { type: EventType.RUN_FINISHED, runId: "r", threadId: "t" },
        ]),
      ),
    );
    expect(surfaceSnapshots(events)).toHaveLength(0);
    const recovery = recoveryActivities(events);
    expect(recovery.length).toBe(1);
    expect((recovery[0] as any).content.status).toBe("failed");
    expect((recovery[0] as any).content.error).toContain("Failed to generate");
  });
});
