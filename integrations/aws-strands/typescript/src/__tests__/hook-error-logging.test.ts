/**
 * Hook exceptions must be logged with the raw Error object so Node prints
 * the stack trace, not `String(e)` which produces "Error: boom" with no
 * context.
 */

import { describe, it, expect, vi, afterEach } from "vitest";
import { EventType } from "@ag-ui/core";
import { collect, minimalRunInput, scriptedStrandsAgent } from "./helpers";

describe("hook error logging", () => {
  let spy: ReturnType<typeof vi.spyOn>;

  afterEach(() => {
    spy?.mockRestore();
  });

  it("stateContextBuilder exception logs the Error object", async () => {
    spy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const agent = scriptedStrandsAgent([]);
    (agent as unknown as { config: Record<string, unknown> }).config = {
      stateContextBuilder: () => {
        throw new Error("builder bombed");
      },
    };
    await collect(
      agent,
      minimalRunInput({
        messages: [{ id: "u1", role: "user", content: "hi" }],
      }),
    );
    // First arg is the prefix string, second is the Error itself.
    expect(spy).toHaveBeenCalled();
    const lastCall = spy.mock.calls.find((c: unknown[]) =>
      String(c[0] ?? "").includes("stateContextBuilder"),
    );
    expect(lastCall).toBeTruthy();
    expect(lastCall?.[1]).toBeInstanceOf(Error);
    expect((lastCall?.[1] as Error).message).toBe("builder bombed");
  });

  it("stateFromArgs exception logs the Error object", async () => {
    spy = vi.spyOn(console, "warn").mockImplementation(() => {});
    // Emit a tool-use block so the hook site fires.
    const { ToolUseBlock } = await import("@strands-agents/sdk");
    const block = new ToolUseBlock({
      name: "Multiply",
      toolUseId: "u1",
      input: { a: 1, b: 2 },
    });
    const agent = scriptedStrandsAgent([block]);
    (agent as unknown as { config: Record<string, unknown> }).config = {
      toolBehaviors: {
        Multiply: {
          stateFromArgs: () => {
            throw new Error("args hook bombed");
          },
        },
      },
    };
    await collect(agent);
    const lastCall = spy.mock.calls.find((c: unknown[]) =>
      String(c[0] ?? "").includes("stateFromArgs"),
    );
    expect(lastCall).toBeTruthy();
    expect(lastCall?.[1]).toBeInstanceOf(Error);
    expect((lastCall?.[1] as Error).message).toBe("args hook bombed");
  });

  it("argsStreamer exception logs the Error and still emits fallback args", async () => {
    spy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { ToolUseBlock } = await import("@strands-agents/sdk");
    const block = new ToolUseBlock({
      name: "Multiply",
      toolUseId: "u1",
      input: { a: 1, b: 2 },
    });
    const agent = scriptedStrandsAgent([block]);
    // eslint-disable-next-line require-yield
    (agent as unknown as { config: Record<string, unknown> }).config = {
      toolBehaviors: {
        Multiply: {
          argsStreamer: async function* () {
            throw new Error("streamer bombed");
          },
        },
      },
    };
    const events = await collect(agent);
    const lastCall = spy.mock.calls.find((c: unknown[]) =>
      String(c[0] ?? "").includes("argsStreamer"),
    );
    expect(lastCall).toBeTruthy();
    expect(lastCall?.[1]).toBeInstanceOf(Error);
    expect((lastCall?.[1] as Error).message).toBe("streamer bombed");
    // Fallback TOOL_CALL_ARGS should still fire with the full args blob.
    const args = events.find(
      (e) => e.type === EventType.TOOL_CALL_ARGS,
    ) as unknown as { delta: string };
    expect(JSON.parse(args.delta)).toEqual({ a: 1, b: 2 });
  });
});
