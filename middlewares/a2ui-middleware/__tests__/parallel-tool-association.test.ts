import { describe, expect, it } from "vitest";
import { ActivitySnapshotEvent, BaseEvent, EventType } from "@ag-ui/client";
import { A2UIMiddleware } from "../src";
import { MockAgent, collectEvents, createRunAgentInput } from "./test-utils";
import langgraphImport from "./fixtures/langgraph-import.json";
import langgraphLive from "./fixtures/langgraph-live.json";
import strandsImport from "./fixtures/strands-ts-import.json";
import strandsLive from "./fixtures/strands-ts-live.json";

async function paint(events: BaseEvent[]) {
  const output = await collectEvents(
    new A2UIMiddleware({
      defaultCatalogId: "copilotkit://app-dashboard-catalog",
    }).run(createRunAgentInput(), new MockAgent(events)),
  );
  return output.filter(
    (event): event is ActivitySnapshotEvent =>
      event.type === EventType.ACTIVITY_SNAPSHOT,
  );
}

describe("parallel A2UI tool association", () => {
  it.each([
    ["LangGraph native import", langgraphImport],
    ["LangGraph live stream", langgraphLive],
    ["Strands TypeScript native import", strandsImport],
    ["Strands TypeScript live stream with distinct message IDs", strandsLive],
  ])(
    "keeps sibling flight and dashboard surfaces separate: %s",
    async (_name, events) => {
      const flight = events.find(
        (event) =>
          "toolCallName" in event && event.toolCallName === "search_flights",
      );
      const dashboard = events.find(
        (event) =>
          "toolCallName" in event && event.toolCallName === "render_a2ui",
      );
      if (
        !flight ||
        !dashboard ||
        !("toolCallId" in flight) ||
        !("toolCallId" in dashboard)
      )
        throw new Error("Missing native tool calls");
      const typedEvents = events.map((event) => {
        const type = Object.values(EventType).find(
          (type) => type === event.type,
        );
        if (!type)
          throw new Error(`Unknown recorded event type: ${event.type}`);
        return { ...event, type };
      });
      const snapshots = await paint(typedEvents);
      const dashboardPaint = snapshots.filter((event) =>
        JSON.stringify(event.content).includes('"canary-dashboard"'),
      );
      const flightPaint = snapshots.filter((event) =>
        JSON.stringify(event.content).includes('"flight-search-results"'),
      );
      expect(dashboardPaint.length).toBeGreaterThan(0);
      expect(
        dashboardPaint.every(
          (event) => event.messageId === `a2ui-surface-${dashboard.toolCallId}`,
        ),
      ).toBe(true);
      expect(flightPaint.length).toBeGreaterThan(0);
      if (events === strandsLive) {
        const content = JSON.stringify(
          flightPaint.map((event) => event.content),
        );
        expect(content).toContain("DL123");
        expect(content).toContain("AA456");
      }
      expect(
        flightPaint.every((event) =>
          event.messageId.endsWith(`-${flight.toolCallId}`),
        ),
      ).toBe(true);
      expect(
        snapshots
          .filter((event) => event.content.status)
          .every(
            (event) =>
              event.messageId === `a2ui-surface-${dashboard.toolCallId}`,
          ),
      ).toBe(true);
    },
  );

  it("preserves legacy nested renders after the outer arguments end", async () => {
    const snapshots = await paint([
      { type: EventType.RUN_STARTED, threadId: "test", runId: "test" },
      {
        type: EventType.TOOL_CALL_START,
        toolCallId: "outer",
        toolCallName: "generate_a2ui",
        parentMessageId: "outer-message",
      },
      { type: EventType.TOOL_CALL_END, toolCallId: "outer" },
      {
        type: EventType.TOOL_CALL_START,
        toolCallId: "inner",
        toolCallName: "render_a2ui",
      },
      {
        type: EventType.TOOL_CALL_ARGS,
        toolCallId: "inner",
        delta: JSON.stringify({
          surfaceId: "nested",
          components: [{ id: "root", component: "Text", text: "Nested" }],
        }),
      },
      { type: EventType.TOOL_CALL_END, toolCallId: "inner" },
      {
        type: EventType.TOOL_CALL_RESULT,
        toolCallId: "outer",
        messageId: "outer-result",
        content: JSON.stringify({
          a2ui_operations: [
            {
              version: "v0.9",
              createSurface: {
                surfaceId: "nested",
                catalogId: "copilotkit://app-dashboard-catalog",
              },
            },
          ],
        }),
      },
      { type: EventType.RUN_FINISHED, threadId: "test", runId: "test" },
    ]);
    expect(snapshots.length).toBeGreaterThan(0);
    expect(
      snapshots.every((event) => event.messageId === "a2ui-surface-outer"),
    ).toBe(true);
    expect(
      snapshots.filter((event) => event.content.a2ui_operations),
    ).toHaveLength(1);
  });

  it("attributes out-of-order results and failures to their own calls", async () => {
    const envelope = JSON.stringify({
      a2ui_operations: [
        {
          version: "v0.9",
          createSurface: {
            surfaceId: "flights",
            catalogId: "copilotkit://app-dashboard-catalog",
          },
        },
      ],
    });
    const snapshots = await paint([
      { type: EventType.RUN_STARTED, threadId: "test", runId: "test" },
      {
        type: EventType.TOOL_CALL_START,
        toolCallId: "flights",
        toolCallName: "search_flights",
        parentMessageId: "siblings",
      },
      {
        type: EventType.TOOL_CALL_START,
        toolCallId: "failed",
        toolCallName: "generate_a2ui",
        parentMessageId: "siblings",
      },
      {
        type: EventType.TOOL_CALL_START,
        toolCallId: "unrelated",
        toolCallName: "query_data",
        parentMessageId: "siblings",
      },
      {
        type: EventType.TOOL_CALL_RESULT,
        toolCallId: "flights",
        messageId: "result-flights",
        role: "tool",
        content: envelope,
      },
      {
        type: EventType.TOOL_CALL_RESULT,
        toolCallId: "failed",
        messageId: "result-failed",
        role: "tool",
        content: JSON.stringify({
          code: "a2ui_recovery_exhausted",
          error: "invalid components",
          attempts: [1, 2],
        }),
      },
      { type: EventType.RUN_FINISHED, threadId: "test", runId: "test" },
    ]);
    expect(
      snapshots.find((event) => event.content.a2ui_operations)?.messageId,
    ).toBe("a2ui-surface-flights");
    expect(
      snapshots.find((event) => event.content.status === "failed")?.messageId,
    ).toBe("a2ui-surface-failed");
  });
});
