import { beforeEach, expect, it, vi } from "vitest";
import { lastValueFrom, toArray } from "rxjs";
import type { RunAgentInput, BaseEvent } from "@ag-ui/core";
const sdk = vi.hoisted(() => ({ query: vi.fn(), getSessionInfo: vi.fn() }));
vi.mock("@anthropic-ai/claude-agent-sdk", async (original) => ({
  ...(await original<object>()),
  ...sdk,
}));
import { ClaudeAgentAdapter } from "./adapter";
import type { SessionBinding } from "./recovery";
const pending = {
  id: "original-call",
  name: "Bash",
  input: { command: "echo test" },
};
const request = (behavior?: string): RunAgentInput => ({
  threadId: "public",
  runId: "run",
  messages: [],
  tools: [],
  state: {},
  context: [],
  forwardedProps: {},
  ...(behavior
    ? {
        resume: [
          {
            interruptId: pending.id,
            status: "resolved",
            payload: { behavior },
          },
        ],
      }
    : {}),
});
const collect = (
  adapter: ClaudeAgentAdapter,
  input: RunAgentInput,
): Promise<BaseEvent[]> => lastValueFrom(adapter.run(input).pipe(toArray()));
beforeEach(() => {
  vi.clearAllMocks();
  sdk.getSessionInfo.mockResolvedValue({ sessionId: "original-session" });
});
for (const behavior of ["allow", "deny"])
  it(`restores original session/profile after restart and ${behavior}s original call`, async () => {
    let saved: SessionBinding | undefined;
    const decisions: string[] = [];
    const store = {
      load: async () => structuredClone(saved),
      save: async (_id: string, value: SessionBinding) => {
        saved = structuredClone(value);
      },
    };
    sdk.query.mockImplementation(({ options, prompt }) =>
      (async function* () {
        yield {
          type: "system",
          subtype: "init",
          session_id: "original-session",
        };
        if (options.resume) {
          expect(options.resume).toBe("original-session");
          expect(options.forkSession).toBe(false);
          expect(options.cwd).toBe("/original");
          expect(typeof prompt[Symbol.asyncIterator]).toBe("function");
          const result = await options.hooks.PreToolUse[
            options.hooks.PreToolUse.length - 1
          ].hooks[0]({
            hook_event_name: "PreToolUse",
            tool_use_id: pending.id,
            tool_name: pending.name,
            tool_input: pending.input,
          });
          decisions.push(result.hookSpecificOutput.permissionDecision);
        }
        yield {
          type: "result",
          subtype: "success",
          session_id: "original-session",
          is_error: false,
          ...(options.resume
            ? {}
            : { stop_reason: "tool_deferred", deferred_tool_use: pending }),
        };
      })(),
    );
    const config = {
      sessionRecovery: {
        store,
        profile: "original-profile",
        resolveProfile: vi.fn(async () => ({ cwd: "/original" })),
      },
    };
    const first = await collect(new ClaudeAgentAdapter(config), request());
    expect(first[first.length - 1]).toMatchObject({
      type: "RUN_FINISHED",
      outcome: { type: "interrupt", interrupts: [{ id: pending.id }] },
    });
    const second = await collect(
      new ClaudeAgentAdapter(config),
      request(behavior),
    );
    expect(second[second.length - 1]).toMatchObject({ type: "RUN_FINISHED" });
    expect(decisions).toEqual([behavior]);
    expect(saved).toEqual({
      sessionId: "original-session",
      profile: "original-profile",
    });
    expect(config.sessionRecovery.resolveProfile).toHaveBeenCalledWith(
      "original-profile",
    );
  });
it("does not spawn a replacement for a missing imported session", async () => {
  sdk.getSessionInfo.mockResolvedValue(undefined);
  const events = await collect(new ClaudeAgentAdapter(), {
    ...request(),
    forwardedProps: { resume: "missing-session" },
  });
  expect(sdk.query).not.toHaveBeenCalled();
  expect(events[events.length - 1]).toMatchObject({
    type: "RUN_ERROR",
    message: expect.stringContaining("unavailable"),
  });
});
it("keeps pending recovery actionable without re-running the SDK", async () => {
  const config = {
    sessionRecovery: {
      profile: "profile",
      resolveProfile: async () => ({}),
      store: {
        load: async () => ({
          sessionId: "original-session",
          profile: "profile",
          pending,
        }),
        save: vi.fn(),
      },
    },
  };
  const events = await collect(new ClaudeAgentAdapter(config), request());
  expect(sdk.query).not.toHaveBeenCalled();
  expect(events[events.length - 1]).toMatchObject({
    type: "RUN_FINISHED",
    outcome: { type: "interrupt" },
  });
});
it("keeps frontend deferral pending with a ready HTTP proxy", async () => {
  const deferred = {
    id: "frontend-original",
    name: "mcp__ag_ui__confirm",
    input: {},
  };
  const adapter = new ClaudeAgentAdapter({
    sessionRecovery: {
      profile: "profile",
      resolveProfile: async () => ({}),
      store: {
        load: async () => ({
          sessionId: "original-session",
          profile: "profile",
          pending: deferred,
        }),
        save: vi.fn(),
      },
    },
  });
  const events = await collect(adapter, {
    ...request(),
    tools: [
      { name: "confirm", description: "", parameters: { type: "object" } },
    ],
  });
  expect(sdk.query).not.toHaveBeenCalled();
  expect(events[events.length - 1]).toMatchObject({
    type: "RUN_FINISHED",
    outcome: { type: "success", pendingToolCallIds: [deferred.id] },
  });
});
