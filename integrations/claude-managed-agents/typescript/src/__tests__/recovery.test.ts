import { lastValueFrom, toArray } from "rxjs";
import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import type { RunAgentInput } from "@ag-ui/client";
import { ManagedAgentsAgent } from "../agent";
import { InMemorySessionStore } from "../sessions";
import { createFakeClient } from "./fake-client";

const custom = (id: string) => ({
  type: "agent.custom_tool_use",
  id,
  name: "browser_tool",
  input: { id },
});
const approval = (id: string) => ({
  type: "agent.tool_use",
  id,
  name: "bash",
  input: { command: "echo ok" },
  evaluated_permission: "ask",
});
const idle = {
  type: "session.status_idle",
  id: "idle_end",
  stop_reason: { type: "end_turn" },
};
const parked = (ids: string[]) => ({
  type: "session.status_idle",
  id: "idle_pending",
  stop_reason: { type: "requires_action", event_ids: ids },
});
const input = (overrides: Partial<RunAgentInput> = {}): RunAgentInput => ({
  threadId: "imported",
  runId: "run",
  messages: [{ role: "user", id: "old", content: "already sent" }],
  tools: [],
  context: [],
  forwardedProps: {},
  ...overrides,
});
const collect = (agent: ManagedAgentsAgent, value = input()) =>
  lastValueFrom(agent.run(value).pipe(toArray()));
const agent = (
  fake: ReturnType<typeof createFakeClient>,
  store = new InMemorySessionStore(),
) =>
  new ManagedAgentsAgent({
    managedAgentId: "agent",
    environmentId: "env",
    client: fake.client,
    sessionStore: store,
    resolveSession: () => ({ sessionId: "original", lastUserMessageId: "old" }),
  });

describe("native session recovery", () => {
  it("does not duplicate saved tool arguments and completes partial text from history", async () => {
    const store = new InMemorySessionStore();
    store.set("5:agent|0:|3:env|0:|imported", {
      sessionId: "original",
      preserveTools: true,
      toolNames: [],
      pendingClientToolUseIds: [],
      lastUserMessageId: "old",
      lastEventId: "cursor",
    });
    const fake = createFakeClient({
      histories: [
        [
          { ...idle, id: "cursor" },
          custom("c1"),
          {
            type: "agent.message",
            id: "reply",
            content: [{ type: "text", text: "hello world" }],
          },
        ],
      ],
    });
    const events = await collect(
      agent(fake, store),
      input({
        messages: [
          {
            role: "assistant",
            id: "reply",
            content: "hello",
            toolCalls: [
              {
                id: "c1",
                type: "function",
                function: { name: "browser tool", arguments: '{"id":"c1"}' },
              },
            ],
          },
        ],
      }),
    );
    expect(events.filter((event) => event.type === "TOOL_CALL_ARGS")).toEqual(
      [],
    );
    expect(events).toContainEqual(
      expect.objectContaining({
        type: "TEXT_MESSAGE_CONTENT",
        delta: " world",
      }),
    );
  });

  it("restores the configured frontend name for a recovered custom call", async () => {
    const fake = createFakeClient({ histories: [[custom("c1")]] });
    const events = await collect(
      agent(fake),
      input({
        tools: [{ name: "browser tool", description: "", parameters: {} }],
      }),
    );
    expect(events).toContainEqual(
      expect.objectContaining({
        type: "TOOL_CALL_START",
        toolCallName: "browser tool",
      }),
    );
  });

  it("preserves unknown live tools and permissions arriving during reconnect", async () => {
    const fake = createFakeClient({
      sessionStatus: "running",
      histories: [[]],
      streams: [
        [
          custom("new_custom"),
          approval("new_approval"),
          parked(["new_custom", "new_approval"]),
        ],
      ],
    });
    const subject = new ManagedAgentsAgent({
      managedAgentId: "agent",
      environmentId: "env",
      client: fake.client,
      toolConfirmation: "deny",
      resolveSession: () => ({
        sessionId: "original",
        lastUserMessageId: "old",
      }),
    });
    const events = await collect(subject);
    expect(fake.sent).toEqual([]);
    expect(events.at(-1)).toMatchObject({
      type: "RUN_FINISHED",
      outcome: { interrupts: [{ id: "new_approval" }] },
    });
  });

  it("reopens without creating, replaying user text, replacing tools or rejecting pending calls", async () => {
    const fake = createFakeClient({
      histories: [[custom("c1"), approval("a1"), parked(["c1", "a1"])]],
    });
    const events = await collect(agent(fake));
    expect(fake.spies.create).not.toHaveBeenCalled();
    expect(fake.spies.update).not.toHaveBeenCalled();
    expect(fake.sent).toEqual([]);
    expect(fake.spies.stream.mock.invocationCallOrder[0]).toBeLessThan(
      fake.spies.list.mock.invocationCallOrder[0],
    );
    expect(events.at(-1)).toMatchObject({
      type: "RUN_FINISHED",
      outcome: {
        type: "interrupt",
        interrupts: [{ id: "a1", reason: "tool-approval" }],
      },
    });
    expect(events).toContainEqual(
      expect.objectContaining({
        name: "managed_agents.session",
        value: { sessionId: "original", threadId: "imported" },
      }),
    );
  });

  it("recovers several requests after restart and sends only matching real answers", async () => {
    const store = new InMemorySessionStore();
    const history = [
      custom("c1"),
      custom("c2"),
      approval("a1"),
      parked(["c1", "c2", "a1"]),
    ];
    await collect(agent(createFakeClient({ histories: [history] }), store));
    // Serialized storage plus a new client/agent: no in-process pending callbacks survive.
    const snapshot = JSON.stringify(store.get("5:agent|0:|3:env|0:|imported"));
    const restored = new InMemorySessionStore();
    restored.set("5:agent|0:|3:env|0:|imported", JSON.parse(snapshot));
    const fake = createFakeClient({ histories: [history], streams: [[idle]] });
    await collect(
      agent(fake, restored),
      input({
        messages: [
          { role: "user", id: "old", content: "already sent" },
          {
            role: "tool",
            id: "r1",
            toolCallId: "c1",
            content: "actual first result",
          },
          {
            role: "tool",
            id: "r2",
            toolCallId: "c2",
            content: "actual second result",
          },
        ],
        resume: [{ interruptId: "a1", status: "resolved", payload: "allow" }],
      }),
    );
    expect(fake.sent).toEqual([
      {
        sessionId: "original",
        events: [
          {
            type: "user.tool_confirmation",
            tool_use_id: "a1",
            result: "allow",
          },
          {
            type: "user.custom_tool_result",
            custom_tool_use_id: "c1",
            content: [{ type: "text", text: "actual first result" }],
            is_error: false,
          },
          {
            type: "user.custom_tool_result",
            custom_tool_use_id: "c2",
            content: [{ type: "text", text: "actual second result" }],
            is_error: false,
          },
        ],
      },
    ]);
  });

  it("holds a new message while one request remains unanswered", async () => {
    const fake = createFakeClient({
      histories: [[custom("c1"), approval("a1"), parked(["c1", "a1"])]],
    });
    const events = await collect(
      agent(fake),
      input({
        messages: [
          { role: "user", id: "old", content: "old" },
          { role: "user", id: "new", content: "move on" },
          { role: "tool", id: "r1", toolCallId: "c1", content: "real" },
        ],
      }),
    );
    expect(fake.sent[0].events).toHaveLength(1);
    expect(events.at(-1)).toMatchObject({
      outcome: { interrupts: [{ id: "a1" }] },
    });
  });

  it.each(["yes", {}, null])(
    "rejects malformed confirmation %j without sending",
    async (payload) => {
      const fake = createFakeClient({ histories: [[approval("a1")]] });
      const events = await collect(
        agent(fake),
        input({ resume: [{ interruptId: "a1", status: "resolved", payload }] }),
      );
      expect(events.at(-1)).toMatchObject({
        type: "RUN_ERROR",
        code: "invalid_resume",
      });
      expect(fake.sent).toEqual([]);
    },
  );

  it("denies only the explicitly cancelled pending permission", async () => {
    const fake = createFakeClient({
      histories: [[approval("a1"), approval("a2")]],
    });
    const events = await collect(
      agent(fake),
      input({ resume: [{ interruptId: "a1", status: "cancelled" }] }),
    );
    expect(fake.sent[0].events).toEqual([
      { type: "user.tool_confirmation", tool_use_id: "a1", result: "deny" },
    ]);
    expect(events.at(-1)).toMatchObject({
      outcome: { interrupts: [{ id: "a2" }] },
    });
  });

  it("does not recover already answered or duplicate cross-posted calls", async () => {
    const fake = createFakeClient({
      histories: [
        [
          custom("done"),
          custom("done"),
          {
            type: "user.custom_tool_result",
            id: "result",
            custom_tool_use_id: "done",
            content: [],
          },
          approval("a1"),
        ],
      ],
    });
    const events = await collect(agent(fake));
    expect(events.filter((e) => e.type === "TOOL_CALL_START")).toEqual([]);
    expect(fake.sent).toEqual([]);
  });

  it("deduplicates history overlap while streaming the resumed response", async () => {
    const history = [approval("a1"), parked(["a1"])];
    const fake = createFakeClient({
      histories: [history],
      streams: [
        [
          ...history,
          {
            type: "agent.message",
            id: "answer",
            content: [{ type: "text", text: "continued" }],
          },
          idle,
        ],
      ],
    });
    const events = await collect(
      agent(fake),
      input({
        resume: [{ interruptId: "a1", status: "resolved", payload: "allow" }],
      }),
    );
    expect(events.filter((e) => e.type === "TOOL_CALL_START")).toEqual([]);
    expect(events).toContainEqual(
      expect.objectContaining({
        type: "TEXT_MESSAGE_CONTENT",
        messageId: "answer",
        delta: "continued",
      }),
    );
  });

  it("keeps terminated and inaccessible sessions bound without replacement", async () => {
    for (const options of [
      { sessionStatus: "terminated" },
      {
        sessionError: Object.assign(new Error("private details"), {
          status: 404,
        }),
      },
    ]) {
      const fake = createFakeClient(options);
      const subject = agent(fake);
      const first = await collect(subject);
      const second = await collect(subject);
      expect(first.at(-1)).toMatchObject({
        type: "RUN_ERROR",
        code: options.sessionStatus ? "session_ended" : "session_unavailable",
      });
      expect(second.at(-1)).toMatchObject({ type: "RUN_ERROR" });
      expect(fake.spies.create).not.toHaveBeenCalled();
      expect(fake.sent).toEqual([]);
    }
  });

  it("replays activity missed during a backend restart from a disk-persisted cursor without executing tools", async () => {
    const dir = await mkdtemp(join(tmpdir(), "managed-recovery-"));
    try {
      const key = "5:agent|0:|3:env|0:|imported";
      await writeFile(
        join(dir, "record.json"),
        JSON.stringify({
          sessionId: "original",
          preserveTools: true,
          toolNames: [],
          pendingClientToolUseIds: [],
          lastUserMessageId: "old",
          lastEventId: "before_restart",
        }),
      );
      const store = new InMemorySessionStore();
      store.set(
        key,
        JSON.parse(await readFile(join(dir, "record.json"), "utf8")),
      );
      const history = [
        { ...idle, id: "before_restart" },
        custom("offline_tool"),
        {
          type: "user.custom_tool_result",
          id: "offline_result",
          custom_tool_use_id: "offline_tool",
          content: [{ type: "text", text: "saved output" }],
        },
        {
          type: "agent.message",
          id: "offline_reply",
          content: [{ type: "text", text: "done while offline" }],
        },
        idle,
      ];
      const fake = createFakeClient({ histories: [history, history] });
      const events = await collect(agent(fake, store));
      expect(events).toContainEqual(
        expect.objectContaining({
          type: "TEXT_MESSAGE_CONTENT",
          messageId: "offline_reply",
          delta: "done while offline",
        }),
      );
      expect(events).toContainEqual(
        expect.objectContaining({
          type: "TOOL_CALL_RESULT",
          messageId: "offline_result",
          toolCallId: "offline_tool",
          content: "saved output",
        }),
      );
      expect(fake.sent).toEqual([]);
      expect(store.get(key)?.lastEventId).toBe("idle_end");
      const next = await collect(agent(fake, store));
      expect(next.some((event) => event.type === "TEXT_MESSAGE_CONTENT")).toBe(
        false,
      );
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  });

  it("preserves child identity and routes its pending result by the original call ID", async () => {
    const fake = createFakeClient({
      histories: [[{ ...custom("child_call"), session_thread_id: "child" }]],
      streams: [
        [
          {
            type: "session.thread_status_idle",
            id: "child_end",
            session_thread_id: "child",
            stop_reason: { type: "end_turn" },
          },
        ],
      ],
    });
    const subject = new ManagedAgentsAgent({
      managedAgentId: "agent",
      environmentId: "env",
      client: fake.client,
      resolveSession: () => ({
        sessionId: "original",
        sessionThreadId: "child",
        lastUserMessageId: "old",
      }),
    });
    const events = await collect(
      subject,
      input({
        messages: [
          {
            role: "tool",
            id: "r",
            toolCallId: "child_call",
            content: "actual",
          },
        ],
      }),
    );
    expect(fake.spies.list).toHaveBeenCalledWith(
      "child",
      { session_id: "original" },
      expect.any(Object),
    );
    expect(fake.sent[0]).toMatchObject({
      sessionId: "original",
      events: [{ custom_tool_use_id: "child_call" }],
    });
    expect(events.at(-1)).toMatchObject({ type: "RUN_FINISHED" });
  });

  it("fails clearly when a blocking request is missing from history", async () => {
    const fake = createFakeClient({ histories: [[parked(["missing"])]] });
    const events = await collect(agent(fake));
    expect(events.at(-1)).toMatchObject({
      type: "RUN_ERROR",
      code: "pending_request_unavailable",
    });
    expect(fake.sent).toEqual([]);
  });

  it("finishes when the session completed between retrieval and history enumeration", async () => {
    const fake = createFakeClient({
      sessionStatus: "running",
      histories: [[idle]],
    });
    const events = await collect(agent(fake));
    expect(events.at(-1)).toMatchObject({ type: "RUN_FINISHED" });
    expect(fake.sent).toEqual([]);
  });

  it("replays an owned session's response after a crash before its first received event", async () => {
    const store = new InMemorySessionStore();
    store.set("5:agent|0:|3:env|0:|imported", {
      sessionId: "original",
      toolNames: [],
      pendingClientToolUseIds: [],
      lastUserMessageId: "old",
    });
    const fake = createFakeClient({
      histories: [
        [
          {
            type: "agent.message",
            id: "missed",
            content: [{ type: "text", text: "completed" }],
          },
          idle,
        ],
      ],
    });
    const events = await collect(agent(fake, store));
    expect(events).toContainEqual(
      expect.objectContaining({
        type: "TEXT_MESSAGE_CONTENT",
        messageId: "missed",
        delta: "completed",
      }),
    );
    expect(fake.sent).toEqual([]);
  });

  it("reports a child terminating after history enumeration as ended", async () => {
    const fake = createFakeClient({
      sessionStatus: "running",
      histories: [[]],
      streams: [
        [
          {
            type: "session.thread_status_terminated",
            id: "ended",
            session_thread_id: "child",
          },
        ],
      ],
    });
    const subject = new ManagedAgentsAgent({
      managedAgentId: "agent",
      environmentId: "env",
      client: fake.client,
      resolveSession: () => ({
        sessionId: "original",
        sessionThreadId: "child",
        lastUserMessageId: "old",
      }),
    });
    const events = await collect(subject);
    expect(events.at(-1)).toMatchObject({
      type: "RUN_ERROR",
      code: "session_ended",
    });
  });

  it("does not expose an approval already resolved externally before end_turn", async () => {
    const fake = createFakeClient({
      streams: [
        [
          approval("a1"),
          {
            type: "user.tool_confirmation",
            id: "external",
            tool_use_id: "a1",
            result: "allow",
          },
          idle,
        ],
      ],
    });
    const subject = new ManagedAgentsAgent({
      managedAgentId: "agent",
      environmentId: "env",
      client: fake.client,
    });
    const events = await collect(subject);
    expect(events.at(-1)).toMatchObject({ type: "RUN_FINISHED" });
    expect(events.at(-1)).not.toHaveProperty("outcome");
  });
});
