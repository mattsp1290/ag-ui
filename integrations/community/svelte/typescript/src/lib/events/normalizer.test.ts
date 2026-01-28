import { describe, it, expect } from "vitest";
import {
  createInitialState,
  processEvent,
  getMessages,
  getActiveToolCalls,
  getAllToolCalls,
  EventType,
} from "./normalizer";

describe("Event Normalizer", () => {
  describe("createInitialState", () => {
    it("should create empty initial state", () => {
      const state = createInitialState();

      expect(state.messages).toEqual([]);
      expect(state.toolCalls.size).toBe(0);
      expect(state.activities.size).toBe(0);
      expect(state.agentState).toEqual({});
      expect(state.run.runId).toBeNull();
      expect(state.run.threadId).toBeNull();
      expect(state.run.isRunning).toBe(false);
      expect(state.run.currentStep).toBeNull();
    });
  });

  describe("processEvent", () => {
    it("should handle RUN_STARTED event", () => {
      const state = createInitialState();
      const event = {
        type: EventType.RUN_STARTED,
        runId: "run-123",
        threadId: "thread-456",
      };

      const result = processEvent(state, event);

      expect(result.run.runId).toBe("run-123");
      expect(result.run.threadId).toBe("thread-456");
      expect(result.run.isRunning).toBe(true);
    });

    it("should handle RUN_FINISHED event", () => {
      const state = createInitialState();
      state.run.isRunning = true;
      state.messages.push({
        id: "msg-1",
        role: "assistant",
        content: "Hello",
        isStreaming: true,
      });

      const event = { type: EventType.RUN_FINISHED };
      const result = processEvent(state, event);

      expect(result.run.isRunning).toBe(false);
      expect(result.messages[0].isStreaming).toBe(false);
    });

    it("should handle TEXT_MESSAGE_START event", () => {
      const state = createInitialState();
      const event = {
        type: EventType.TEXT_MESSAGE_START,
        messageId: "msg-1",
        role: "assistant",
      };

      const result = processEvent(state, event);

      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].id).toBe("msg-1");
      expect(result.messages[0].role).toBe("assistant");
      expect(result.messages[0].isStreaming).toBe(true);
    });

    it("should handle TEXT_MESSAGE_CONTENT event", () => {
      const state = createInitialState();
      state.messages.push({
        id: "msg-1",
        role: "assistant",
        content: "Hello",
        isStreaming: true,
      });

      const event = {
        type: EventType.TEXT_MESSAGE_CONTENT,
        messageId: "msg-1",
        delta: " world",
      };

      const result = processEvent(state, event);

      expect(result.messages[0].content).toBe("Hello world");
    });

    it("should handle TEXT_MESSAGE_END event", () => {
      const state = createInitialState();
      state.messages.push({
        id: "msg-1",
        role: "assistant",
        content: "Hello world",
        isStreaming: true,
      });

      const event = {
        type: EventType.TEXT_MESSAGE_END,
        messageId: "msg-1",
      };

      const result = processEvent(state, event);

      expect(result.messages[0].isStreaming).toBe(false);
    });

    it("should handle TOOL_CALL_START event", () => {
      const state = createInitialState();
      const event = {
        type: EventType.TOOL_CALL_START,
        toolCallId: "tc-1",
        toolCallName: "search",
      };

      const result = processEvent(state, event);

      expect(result.toolCalls.size).toBe(1);
      const toolCall = result.toolCalls.get("tc-1");
      expect(toolCall?.name).toBe("search");
      expect(toolCall?.status).toBe("pending");
    });

    it("should handle TOOL_CALL_ARGS event", () => {
      const state = createInitialState();
      state.toolCalls.set("tc-1", {
        id: "tc-1",
        name: "search",
        arguments: '{"query": "',
        status: "pending",
      });

      const event = {
        type: EventType.TOOL_CALL_ARGS,
        toolCallId: "tc-1",
        delta: 'hello"}',
      };

      const result = processEvent(state, event);

      const toolCall = result.toolCalls.get("tc-1");
      expect(toolCall?.arguments).toBe('{"query": "hello"}');
      expect(toolCall?.status).toBe("streaming");
    });

    it("should handle TOOL_CALL_END event", () => {
      const state = createInitialState();
      state.toolCalls.set("tc-1", {
        id: "tc-1",
        name: "search",
        arguments: '{"query": "hello"}',
        status: "streaming",
      });

      const event = {
        type: EventType.TOOL_CALL_END,
        toolCallId: "tc-1",
      };

      const result = processEvent(state, event);

      const toolCall = result.toolCalls.get("tc-1");
      expect(toolCall?.status).toBe("completed");
      expect(toolCall?.parsedArguments).toEqual({ query: "hello" });
    });

    it("should handle STATE_SNAPSHOT event", () => {
      const state = createInitialState();
      const event = {
        type: EventType.STATE_SNAPSHOT,
        snapshot: { counter: 42, user: { name: "John" } },
      };

      const result = processEvent(state, event);

      expect(result.agentState).toEqual({ counter: 42, user: { name: "John" } });
    });

    it("should handle STATE_DELTA event", () => {
      const state = createInitialState();
      state.agentState = { counter: 42 };

      const event = {
        type: EventType.STATE_DELTA,
        delta: [{ op: "replace", path: "/counter", value: 43 }],
      };

      const result = processEvent(state, event);

      expect(result.agentState).toEqual({ counter: 43 });
    });
  });

  describe("getMessages", () => {
    it("should return messages array", () => {
      const state = createInitialState();
      state.messages.push(
        { id: "1", role: "user", content: "Hi", isStreaming: false },
        { id: "2", role: "assistant", content: "Hello!", isStreaming: false }
      );

      const messages = getMessages(state);

      expect(messages).toHaveLength(2);
      expect(messages[0].role).toBe("user");
      expect(messages[1].role).toBe("assistant");
    });
  });

  describe("getActiveToolCalls", () => {
    it("should return only pending and streaming tool calls", () => {
      const state = createInitialState();
      state.toolCalls.set("tc-1", {
        id: "tc-1",
        name: "search",
        arguments: "",
        status: "pending",
      });
      state.toolCalls.set("tc-2", {
        id: "tc-2",
        name: "compute",
        arguments: "{}",
        status: "streaming",
      });
      state.toolCalls.set("tc-3", {
        id: "tc-3",
        name: "finished",
        arguments: "{}",
        status: "completed",
      });

      const active = getActiveToolCalls(state);

      expect(active).toHaveLength(2);
      expect(active.map((tc) => tc.id)).toContain("tc-1");
      expect(active.map((tc) => tc.id)).toContain("tc-2");
    });
  });

  describe("getAllToolCalls", () => {
    it("should return all tool calls", () => {
      const state = createInitialState();
      state.toolCalls.set("tc-1", {
        id: "tc-1",
        name: "search",
        arguments: "",
        status: "pending",
      });
      state.toolCalls.set("tc-2", {
        id: "tc-2",
        name: "finished",
        arguments: "{}",
        status: "completed",
      });

      const all = getAllToolCalls(state);

      expect(all).toHaveLength(2);
    });
  });

  describe("STATE_DELTA security", () => {
    it("should throw error on __proto__ prototype pollution attempt", () => {
      const state = createInitialState();
      state.agentState = { safe: "value" };

      const event = {
        type: EventType.STATE_DELTA,
        delta: [{ op: "add", path: "/__proto__/polluted", value: "malicious" }],
      };

      expect(() => processEvent(state, event)).toThrow(
        "[AG-UI Svelte] Blocked prototype pollution attempt: /__proto__/polluted"
      );
    });

    it("should throw error on constructor prototype pollution attempt", () => {
      const state = createInitialState();
      state.agentState = { safe: "value" };

      const event = {
        type: EventType.STATE_DELTA,
        delta: [{ op: "add", path: "/constructor/prototype", value: "malicious" }],
      };

      expect(() => processEvent(state, event)).toThrow(
        "[AG-UI Svelte] Blocked prototype pollution attempt: /constructor/prototype"
      );
    });

    it("should throw error on prototype key in nested path", () => {
      const state = createInitialState();
      state.agentState = { nested: { obj: {} } };

      const event = {
        type: EventType.STATE_DELTA,
        delta: [{ op: "add", path: "/nested/prototype/evil", value: "malicious" }],
      };

      expect(() => processEvent(state, event)).toThrow(
        "[AG-UI Svelte] Blocked prototype pollution attempt: /nested/prototype/evil"
      );
    });

    it("should allow legitimate state updates", () => {
      const state = createInitialState();
      state.agentState = { counter: 0 };

      const event = {
        type: EventType.STATE_DELTA,
        delta: [{ op: "replace", path: "/counter", value: 42 }],
      };

      const result = processEvent(state, event);
      expect(result.agentState).toEqual({ counter: 42 });
    });
  });
});
