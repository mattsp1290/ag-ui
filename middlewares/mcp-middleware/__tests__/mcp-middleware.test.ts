import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  AbstractAgent,
  BaseEvent,
  EventType,
  RunAgentInput,
  Tool,
} from "@ag-ui/client";
import { Observable, firstValueFrom, toArray } from "rxjs";

// --- Mock the MCP SDK ---------------------------------------------------------
const mockConnect = vi.fn();
const mockClose = vi.fn();
const mockListTools = vi.fn();
const mockCallTool = vi.fn();

vi.mock("@modelcontextprotocol/sdk/client/index.js", () => ({
  Client: class MockClient {
    connect = mockConnect;
    close = mockClose;
    listTools = mockListTools;
    callTool = mockCallTool;
  },
}));
vi.mock("@modelcontextprotocol/sdk/client/sse.js", () => ({
  SSEClientTransport: class {
    constructor(public url: URL) {}
  },
}));
vi.mock("@modelcontextprotocol/sdk/client/streamableHttp.js", () => ({
  StreamableHTTPClientTransport: class {
    constructor(public url: URL) {}
  },
}));

import { MCPMiddleware } from "../src/index";

// --- Event builders (real streaming events; no MESSAGES_SNAPSHOT) -------------
const THREAD = "t";

function runStarted(runId = "r"): BaseEvent {
  return { type: EventType.RUN_STARTED, threadId: THREAD, runId } as BaseEvent;
}
function runFinished(runId = "r"): BaseEvent {
  return { type: EventType.RUN_FINISHED, threadId: THREAD, runId } as BaseEvent;
}
function runError(message = "boom"): BaseEvent {
  return { type: EventType.RUN_ERROR, message } as BaseEvent;
}

/** Streaming events for one assistant tool call. `args` may be split into
 *  multiple deltas to simulate chunked argument streaming. */
function toolCall(
  toolCallId: string,
  toolCallName: string,
  args: string | string[] = "{}",
): BaseEvent[] {
  const deltas = Array.isArray(args) ? args : [args];
  return [
    { type: EventType.TOOL_CALL_START, toolCallId, toolCallName } as BaseEvent,
    ...deltas.map(
      (delta) =>
        ({ type: EventType.TOOL_CALL_ARGS, toolCallId, delta }) as BaseEvent,
    ),
    { type: EventType.TOOL_CALL_END, toolCallId } as BaseEvent,
  ];
}

function textMessage(messageId: string, text: string): BaseEvent[] {
  return [
    { type: EventType.TEXT_MESSAGE_START, messageId, role: "assistant" } as BaseEvent,
    { type: EventType.TEXT_MESSAGE_CONTENT, messageId, delta: text } as BaseEvent,
    { type: EventType.TEXT_MESSAGE_END, messageId } as BaseEvent,
  ];
}

// --- Mock agents --------------------------------------------------------------
/** Replays a different batch of events on each successive run() call. */
class BatchMockAgent extends AbstractAgent {
  public runCalls: RunAgentInput[] = [];
  private call = 0;
  constructor(private batches: BaseEvent[][]) {
    super();
  }
  run(input: RunAgentInput): Observable<BaseEvent> {
    this.runCalls.push(input);
    const events = this.batches[this.call] ?? [runStarted(), runFinished()];
    this.call++;
    return new Observable((subscriber) => {
      for (const event of events) subscriber.next(event);
      subscriber.complete();
    });
  }
}

/** Always replays the same batch — used to exercise the runaway guard. */
class LoopingMockAgent extends AbstractAgent {
  public runCount = 0;
  constructor(private events: BaseEvent[]) {
    super();
  }
  run(): Observable<BaseEvent> {
    this.runCount++;
    return new Observable((subscriber) => {
      for (const event of this.events) subscriber.next(event);
      subscriber.complete();
    });
  }
}

function createRunAgentInput(
  overrides: Partial<RunAgentInput> = {},
): RunAgentInput {
  return {
    threadId: THREAD,
    runId: "r",
    tools: [],
    context: [],
    forwardedProps: {},
    state: {},
    messages: [],
    ...overrides,
  };
}

async function collectEvents(o: Observable<BaseEvent>): Promise<BaseEvent[]> {
  return firstValueFrom(o.pipe(toArray()));
}

const weatherServer = (): { type: "http"; url: string; serverId: string } => ({
  type: "http",
  url: "https://example.com/mcp",
  serverId: "s",
});

beforeEach(() => {
  mockConnect.mockReset().mockResolvedValue(undefined);
  mockClose.mockReset().mockResolvedValue(undefined);
  mockListTools.mockReset().mockResolvedValue({ tools: [] });
  mockCallTool
    .mockReset()
    .mockResolvedValue({ content: [{ type: "text", text: "ok" }] });
});

// --- Tool injection -----------------------------------------------------------
describe("MCPMiddleware — tool injection", () => {
  async function injectedNames(
    middleware: MCPMiddleware,
    input: RunAgentInput,
  ): Promise<string[]> {
    const next = new BatchMockAgent([[runStarted(), runFinished()]]);
    await collectEvents(middleware.run(input, next));
    return next.runCalls[0].tools.map((t) => t.name);
  }

  it("passes through untouched with no servers", async () => {
    const names = await injectedNames(new MCPMiddleware(), createRunAgentInput());
    expect(names).toEqual([]);
    expect(mockConnect).not.toHaveBeenCalled();
  });

  it("prefixes injected tools as mcp__{server}__{tool}", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "list_issues", inputSchema: {} }] });
    const names = await injectedNames(
      new MCPMiddleware([{ ...weatherServer(), serverId: "github" }]),
      createRunAgentInput(),
    );
    expect(names).toEqual(["mcp__github__list_issues"]);
  });

  it("falls back to server{index} without serverId", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "ping", inputSchema: {} }] });
    const names = await injectedNames(
      new MCPMiddleware([{ type: "http", url: "https://example.com/mcp" }]),
      createRunAgentInput(),
    );
    expect(names).toEqual(["mcp__server0__ping"]);
  });

  it("merges MCP tools after existing input tools", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "ping", inputSchema: {} }] });
    const existing: Tool = { name: "existing", description: "", parameters: {} };
    const names = await injectedNames(
      new MCPMiddleware([weatherServer()]),
      createRunAgentInput({ tools: [existing] }),
    );
    expect(names).toEqual(["existing", "mcp__s__ping"]);
  });

  it("dedupes colliding names", async () => {
    mockListTools.mockResolvedValue({
      tools: [{ name: "dup", inputSchema: {} }, { name: "dup", inputSchema: {} }],
    });
    const names = await injectedNames(
      new MCPMiddleware([weatherServer()]),
      createRunAgentInput(),
    );
    expect(names).toEqual(["mcp__s__dup", "mcp__s__dup_1"]);
  });

  it("truncates names to 64 characters", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "t".repeat(80), inputSchema: {} }] });
    const names = await injectedNames(
      new MCPMiddleware([weatherServer()]),
      createRunAgentInput(),
    );
    expect(names[0].length).toBe(64);
  });

  it("skips a server that fails to list, keeping the others", async () => {
    mockListTools
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValueOnce({ tools: [{ name: "ok", inputSchema: {} }] });
    const names = await injectedNames(
      new MCPMiddleware([
        { type: "http", url: "https://bad/mcp", serverId: "bad" },
        { type: "http", url: "https://good/mcp", serverId: "good" },
      ]),
      createRunAgentInput(),
    );
    expect(names).toEqual(["mcp__good__ok"]);
  });
});

// --- Execution loop -----------------------------------------------------------
describe("MCPMiddleware — execution loop", () => {
  it("does not interfere when no MCP tool calls are open", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [runStarted(), ...textMessage("m1", "hi"), runFinished()],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).not.toHaveBeenCalled();
    expect(next.runCalls).toHaveLength(1);
    expect(received.map((e) => e.type)).toEqual([
      EventType.RUN_STARTED,
      EventType.TEXT_MESSAGE_START,
      EventType.TEXT_MESSAGE_CONTENT,
      EventType.TEXT_MESSAGE_END,
      EventType.RUN_FINISHED,
    ]);
  });

  it("ignores a call that matches the prefix but is not a known MCP tool", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__ghost"), runFinished()],
    ]);
    await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).not.toHaveBeenCalled();
    expect(next.runCalls).toHaveLength(1);
  });

  it("scenario 1: executes our tool, emits result, then runs again", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    mockCallTool.mockResolvedValue({ content: [{ type: "text", text: "sunny" }] });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather", '{"city":"sf"}'), runFinished()],
      [runStarted("r2"), ...textMessage("m2", "It is sunny."), runFinished("r2")],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).toHaveBeenCalledTimes(1);
    expect(mockCallTool).toHaveBeenCalledWith({
      name: "weather",
      arguments: { city: "sf" },
    });
    const result = received.find((e) => e.type === EventType.TOOL_CALL_RESULT);
    expect((result as unknown as { content: string }).content).toBe("sunny");
    expect(next.runCalls).toHaveLength(2);
    expect(next.runCalls[1].messages.some((m) => m.role === "tool")).toBe(true);
  });

  it("scenario 2: stops when a non-MCP tool call is still open", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [
        runStarted(),
        ...toolCall("c1", "mcp__s__weather"),
        ...toolCall("c2", "frontendTool"),
        runFinished(),
      ],
      [runStarted("r2"), runFinished("r2")],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).toHaveBeenCalledTimes(1);
    expect(next.runCalls).toHaveLength(1);
    expect(received.filter((e) => e.type === EventType.TOOL_CALL_RESULT)).toHaveLength(1);
  });

  it("assembles tool-call arguments streamed across multiple chunks", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather", ['{"ci', 'ty":', '"sf"}']), runFinished()],
      [runStarted("r2"), ...textMessage("m2", "done"), runFinished("r2")],
    ]);
    await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).toHaveBeenCalledWith({
      name: "weather",
      arguments: { city: "sf" },
    });
  });

  it("loops multiple hops until no MCP calls remain", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather"), runFinished()],
      [runStarted("r2"), ...toolCall("c2", "mcp__s__weather"), runFinished("r2")],
      [runStarted("r3"), ...textMessage("m3", "finally done"), runFinished("r3")],
    ]);
    await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).toHaveBeenCalledTimes(2);
    expect(next.runCalls).toHaveLength(3);
  });

  it("executes multiple MCP calls in one round, surfacing per-call failures", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    mockCallTool
      .mockResolvedValueOnce({ content: [{ type: "text", text: "sunny" }] })
      .mockRejectedValueOnce(new Error("server exploded"));
    const next = new BatchMockAgent([
      [
        runStarted(),
        ...toolCall("c1", "mcp__s__weather"),
        ...toolCall("c2", "mcp__s__weather"),
        runFinished(),
      ],
      [runStarted("r2"), ...textMessage("m2", "ok"), runFinished("r2")],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    const results = received.filter((e) => e.type === EventType.TOOL_CALL_RESULT);
    expect(results).toHaveLength(2);
    const contents = results.map((r) => (r as unknown as { content: string }).content);
    expect(contents).toContain("sunny");
    expect(contents.some((c) => c.includes("Error executing tool weather"))).toBe(true);
    expect(next.runCalls).toHaveLength(2); // still looped — failures don't block
  });

  it("stringifies non-text tool results", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    mockCallTool.mockResolvedValue({
      content: [{ type: "image", data: "base64..." }],
    });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather"), runFinished()],
      [runStarted("r2"), runFinished("r2")],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    const result = received.find((e) => e.type === EventType.TOOL_CALL_RESULT);
    const content = (result as unknown as { content: string }).content;
    expect(content).toContain("image");
  });

  it("stops at maxIterations instead of looping forever", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    // This agent ALWAYS emits an unresolved MCP tool call.
    const next = new LoopingMockAgent([
      runStarted(),
      ...toolCall("c1", "mcp__s__weather"),
      runFinished(),
    ]);
    await collectEvents(
      new MCPMiddleware([weatherServer()], { maxIterations: 3 }).run(
        createRunAgentInput(),
        next,
      ),
    );
    expect(mockCallTool).toHaveBeenCalledTimes(3);
    // 3 execution rounds → 4 agent runs (the 4th detects the cap and stops).
    expect(next.runCount).toBe(4);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });

  it("does not execute tools when the run errors", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather"), runError("kaboom")],
    ]);
    const received = await collectEvents(
      new MCPMiddleware([weatherServer()]).run(createRunAgentInput(), next),
    );
    expect(mockCallTool).not.toHaveBeenCalled();
    expect(next.runCalls).toHaveLength(1);
    expect(received.some((e) => e.type === EventType.RUN_ERROR)).toBe(true);
  });

  it("stops the loop when the subscription is cancelled mid-execution", async () => {
    mockListTools.mockResolvedValue({ tools: [{ name: "weather", inputSchema: {} }] });
    let releaseCall: (v: unknown) => void = () => {};
    mockCallTool.mockImplementation(
      () => new Promise((resolve) => (releaseCall = resolve)),
    );
    const next = new BatchMockAgent([
      [runStarted(), ...toolCall("c1", "mcp__s__weather"), runFinished()],
      [runStarted("r2"), runFinished("r2")],
    ]);
    const received: BaseEvent[] = [];
    const sub = new MCPMiddleware([weatherServer()])
      .run(createRunAgentInput(), next)
      .subscribe((e) => received.push(e));

    // Wait until execution is in-flight (callTool invoked), then cancel.
    await vi.waitFor(() => expect(mockCallTool).toHaveBeenCalledTimes(1));
    sub.unsubscribe();
    releaseCall({ content: [{ type: "text", text: "late" }] });
    await new Promise((r) => setTimeout(r, 10));

    expect(received.some((e) => e.type === EventType.TOOL_CALL_RESULT)).toBe(false);
    expect(next.runCalls).toHaveLength(1); // never looped
  });
});
