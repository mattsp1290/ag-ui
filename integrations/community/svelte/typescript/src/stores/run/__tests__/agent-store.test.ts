import { describe, it, expect, vi, beforeEach } from "vitest";
import { createAgentStore, type AgentStore } from "../agent-store.svelte";
import type { AbstractAgent, Message } from "../types";

/**
 * Create a mock agent for testing
 */
function createMockAgent(overrides: Partial<AbstractAgent> = {}): AbstractAgent {
  return {
    threadId: "test-thread-123",
    messages: [],
    state: {},
    subscribe: vi.fn(() => ({ unsubscribe: vi.fn() })),
    addMessage: vi.fn(),
    setMessages: vi.fn(),
    setState: vi.fn(),
    runAgent: vi.fn(() => Promise.resolve()),
    connectAgent: vi.fn(() => Promise.resolve()),
    abortRun: vi.fn(),
    detachActiveRun: vi.fn(),
    ...overrides,
  };
}

describe("Agent Store", () => {
  let mockAgent: AbstractAgent;
  let store: AgentStore;

  beforeEach(() => {
    mockAgent = createMockAgent();
    store = createAgentStore(mockAgent);
  });

  describe("initialization", () => {
    it("should initialize with idle status", () => {
      expect(store.status).toBe("idle");
    });

    it("should initialize with empty messages", () => {
      expect(store.messages).toEqual([]);
    });

    it("should initialize with empty state", () => {
      expect(store.state).toEqual({});
    });

    it("should initialize with isRunning as false", () => {
      expect(store.isRunning).toBe(false);
    });

    it("should initialize with no error", () => {
      expect(store.error).toBeNull();
    });

    it("should initialize with empty tool calls", () => {
      expect(store.toolCalls).toEqual([]);
      expect(store.activeToolCalls).toEqual([]);
    });

    it("should initialize with null runId and threadId", () => {
      expect(store.runId).toBeNull();
      expect(store.threadId).toBeNull();
    });

    it("should initialize with initial messages when provided", () => {
      const initialMessages: Message[] = [
        { id: "msg-1", role: "user", content: "Hello" },
        { id: "msg-2", role: "assistant", content: "Hi there!" },
      ];
      const storeWithMessages = createAgentStore(mockAgent, { initialMessages });

      const messages = storeWithMessages.messages;
      expect(messages).toHaveLength(2);
      expect(messages[0].id).toBe("msg-1");
      expect(messages[0].role).toBe("user");
      expect(messages[1].id).toBe("msg-2");
      expect(messages[1].role).toBe("assistant");
    });

    it("should initialize with initial state when provided", () => {
      const initialState = { counter: 42, user: { name: "Test" } };
      const storeWithState = createAgentStore(mockAgent, { initialState });

      expect(storeWithState.state).toEqual(initialState);
    });
  });

  describe("start()", () => {
    it("should set status to starting then call runAgent", async () => {
      const runAgentPromise = new Promise<void>((resolve) => {
        setTimeout(resolve, 10);
      });
      mockAgent.runAgent = vi.fn(() => runAgentPromise);

      const startPromise = store.start({ text: "Hello" });

      // Status should be starting
      expect(store.status).toBe("starting");

      await startPromise;
    });

    it("should add user message when text is provided", async () => {
      await store.start({ text: "Hello agent" });

      expect(mockAgent.addMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          role: "user",
          content: "Hello agent",
        })
      );

      const messages = store.messages;
      expect(messages).toHaveLength(1);
      expect(messages[0].role).toBe("user");
      expect(messages[0].content).toBe("Hello agent");
    });

    it("should add custom message when message is provided", async () => {
      const customMessage: Message = {
        id: "custom-1",
        role: "user",
        content: "Custom message",
      };

      await store.start({ message: customMessage });

      expect(mockAgent.addMessage).toHaveBeenCalledWith(customMessage);
    });

    it("should subscribe to agent events", async () => {
      await store.start({ text: "Hello" });

      expect(mockAgent.subscribe).toHaveBeenCalled();
    });

    it("should pass tools and context to runAgent", async () => {
      const tools = [{ name: "search", description: "Search tool" }];
      const context = [{ description: "Context", value: "value" }];

      await store.start({ text: "Hello", tools, context });

      expect(mockAgent.runAgent).toHaveBeenCalledWith(
        expect.objectContaining({
          tools,
          context,
        })
      );
    });

    it("should set status to error when runAgent fails", async () => {
      mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("Agent error")));

      await expect(store.start({ text: "Hello" })).rejects.toThrow("Agent error");

      expect(store.status).toBe("error");
      expect(store.error).not.toBeNull();
    });

    it("should clear previous error when starting", async () => {
      // First, trigger an error
      mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("First error")));
      await expect(store.start({ text: "Hello" })).rejects.toThrow();
      expect(store.error).not.toBeNull();

      // Reset mock to succeed
      mockAgent.runAgent = vi.fn(() => Promise.resolve());

      // Start again should clear error
      const startPromise = store.start({ text: "Hello again" });
      expect(store.error).toBeNull();
      await startPromise;
    });
  });

  describe("cancel()", () => {
    it("should abort the running agent", async () => {
      // Start a run
      mockAgent.runAgent = vi.fn(() => new Promise(() => {})); // Never resolves
      store.start({ text: "Hello" });

      // Cancel it
      store.cancel();

      expect(mockAgent.abortRun).toHaveBeenCalled();
      expect(mockAgent.detachActiveRun).toHaveBeenCalled();
    });

    it("should set status to cancelled", () => {
      store.cancel();

      expect(store.status).toBe("cancelled");
    });

    it("should set error to RunCancelledError", () => {
      store.cancel();

      const error = store.error;
      expect(error).not.toBeNull();
      expect(error?.name).toBe("RunCancelledError");
    });
  });

  describe("addMessage()", () => {
    it("should add message to agent and store", () => {
      const message: Message = {
        id: "msg-1",
        role: "user",
        content: "Hello",
      };

      store.addMessage(message);

      expect(mockAgent.addMessage).toHaveBeenCalledWith(message);

      const messages = store.messages;
      expect(messages).toHaveLength(1);
      expect(messages[0].id).toBe("msg-1");
    });
  });

  describe("reset()", () => {
    it("should clear all messages", async () => {
      await store.start({ text: "Hello" });

      store.reset();

      expect(mockAgent.setMessages).toHaveBeenCalledWith([]);
      expect(store.messages).toEqual([]);
    });

    it("should reset state", async () => {
      const storeWithState = createAgentStore(mockAgent, {
        initialState: { counter: 42 }
      });

      storeWithState.reset();

      expect(mockAgent.setState).toHaveBeenCalledWith({});
      expect(storeWithState.state).toEqual({});
    });

    it("should set status to idle", () => {
      store.reset();

      expect(store.status).toBe("idle");
    });

    it("should clear error", async () => {
      mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("Error")));
      await expect(store.start({ text: "Hello" })).rejects.toThrow();

      store.reset();

      expect(store.error).toBeNull();
    });
  });

  describe("clearError()", () => {
    it("should clear the error", async () => {
      mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("Error")));
      await expect(store.start({ text: "Hello" })).rejects.toThrow();

      store.clearError();

      expect(store.error).toBeNull();
    });

    it("should reset status to idle if in error state", async () => {
      mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("Error")));
      await expect(store.start({ text: "Hello" })).rejects.toThrow();
      expect(store.status).toBe("error");

      store.clearError();

      expect(store.status).toBe("idle");
    });
  });

  describe("destroy()", () => {
    it("should unsubscribe from agent", async () => {
      const unsubscribe = vi.fn();
      mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));

      await store.start({ text: "Hello" });
      store.destroy();

      expect(unsubscribe).toHaveBeenCalled();
    });

    it("should cancel any running agent", () => {
      store.destroy();

      expect(mockAgent.abortRun).toHaveBeenCalled();
    });
  });

  describe("reconnect()", () => {
    it("should call connectAgent", async () => {
      await store.reconnect();

      expect(mockAgent.connectAgent).toHaveBeenCalled();
    });

    it("should set status to error if reconnect fails", async () => {
      mockAgent.connectAgent = vi.fn(() => Promise.reject(new Error("Reconnect failed")));

      await expect(store.reconnect()).rejects.toThrow();

      expect(store.status).toBe("error");
    });
  });

  describe("reconnectWithRetry()", () => {
    it("should succeed on first attempt if connection works", async () => {
      await store.reconnectWithRetry();

      expect(mockAgent.connectAgent).toHaveBeenCalledTimes(1);
    });

    it("should retry on failure", async () => {
      let attempts = 0;
      mockAgent.connectAgent = vi.fn(() => {
        attempts++;
        if (attempts < 2) {
          return Promise.reject(new Error("Network error"));
        }
        return Promise.resolve();
      });

      await store.reconnectWithRetry({ maxRetries: 3, baseDelayMs: 10 });

      expect(mockAgent.connectAgent).toHaveBeenCalledTimes(2);
    });

    it("should throw after max retries exhausted", async () => {
      mockAgent.connectAgent = vi.fn(() => Promise.reject(new Error("Always fails")));

      await expect(
        store.reconnectWithRetry({ maxRetries: 2, baseDelayMs: 10 })
      ).rejects.toThrow(/Failed to reconnect after 3 attempts/);

      // 1 initial + 2 retries = 3 total
      expect(mockAgent.connectAgent).toHaveBeenCalledTimes(3);
    });

    it("should call onRetry callback with attempt and delay", async () => {
      let attempts = 0;
      mockAgent.connectAgent = vi.fn(() => {
        attempts++;
        if (attempts < 3) {
          return Promise.reject(new Error("Network error"));
        }
        return Promise.resolve();
      });

      const onRetry = vi.fn();

      await store.reconnectWithRetry({
        maxRetries: 3,
        baseDelayMs: 100,
        onRetry,
      });

      // Should have been called twice (before retry 1 and retry 2)
      expect(onRetry).toHaveBeenCalledTimes(2);
      expect(onRetry).toHaveBeenNthCalledWith(1, 1, 100); // First retry: 100ms
      expect(onRetry).toHaveBeenNthCalledWith(2, 2, 200); // Second retry: 200ms
    });

    it("should respect maxDelayMs cap", async () => {
      let attempts = 0;
      mockAgent.connectAgent = vi.fn(() => {
        attempts++;
        if (attempts < 5) {
          return Promise.reject(new Error("Network error"));
        }
        return Promise.resolve();
      });

      const onRetry = vi.fn();

      await store.reconnectWithRetry({
        maxRetries: 5,
        baseDelayMs: 100,
        maxDelayMs: 300,
        onRetry,
      });

      // Delays should be: 100, 200, 300, 300 (capped)
      expect(onRetry).toHaveBeenNthCalledWith(1, 1, 100);
      expect(onRetry).toHaveBeenNthCalledWith(2, 2, 200);
      expect(onRetry).toHaveBeenNthCalledWith(3, 3, 300); // capped
      expect(onRetry).toHaveBeenNthCalledWith(4, 4, 300); // still capped
    });

    it("should use default maxRetries when not provided", async () => {
      mockAgent.connectAgent = vi.fn(() => Promise.reject(new Error("Always fails")));

      // With defaults (maxRetries=3), should attempt 4 times total
      // Use minimal delays to avoid timeout
      await expect(
        store.reconnectWithRetry({ baseDelayMs: 1, maxDelayMs: 5 })
      ).rejects.toThrow();

      expect(mockAgent.connectAgent).toHaveBeenCalledTimes(4);
    });
  });

  describe("batching configuration", () => {
    it("should accept batching config options", () => {
      const storeWithBatching = createAgentStore(mockAgent, {
        enableBatching: true,
        batchIntervalMs: 32,
        maxBatchSize: 50,
      });

      // Store should be created without error
      expect(storeWithBatching).toBeDefined();
      expect(storeWithBatching.status).toBe("idle");
    });

    it("should work with batching disabled", () => {
      const storeNoBatching = createAgentStore(mockAgent, {
        enableBatching: false,
      });

      expect(storeNoBatching).toBeDefined();
      expect(storeNoBatching.status).toBe("idle");
    });
  });

  describe("debug mode", () => {
    it("should accept debug config option", () => {
      const debugStore = createAgentStore(mockAgent, { debug: true });

      expect(debugStore).toBeDefined();
    });
  });

  describe("edge cases", () => {
    describe("concurrent start() calls", () => {
      it("should handle multiple rapid start() calls", async () => {
        // Create a delayed agent that takes time to complete
        let runCount = 0;
        mockAgent.runAgent = vi.fn(() => {
          runCount++;
          return new Promise((resolve) => setTimeout(resolve, 50));
        });

        // Start multiple runs rapidly
        const promise1 = store.start({ text: "First" });
        const promise2 = store.start({ text: "Second" });
        const promise3 = store.start({ text: "Third" });

        // All should eventually resolve
        await Promise.all([promise1, promise2, promise3]);

        // runAgent should have been called for each start
        expect(runCount).toBe(3);
      });

      it("should maintain correct status through concurrent calls", async () => {
        mockAgent.runAgent = vi.fn(() => Promise.resolve());

        // Start first run
        const promise1 = store.start({ text: "First" });
        expect(store.status).toBe("starting");

        await promise1;
      });
    });

    describe("destroy() during active run", () => {
      it("should clean up properly when destroyed during active run", async () => {
        const unsubscribe = vi.fn();
        mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));

        // Start a long-running operation
        mockAgent.runAgent = vi.fn(() => new Promise(() => {})); // Never resolves

        // Start the run (don't await)
        store.start({ text: "Hello" });

        // Immediately destroy
        store.destroy();

        // Should have cleaned up
        expect(mockAgent.abortRun).toHaveBeenCalled();
        expect(store.status).toBe("cancelled");
      });

      it("should not throw when destroyed multiple times", () => {
        // Destroy should be idempotent
        expect(() => {
          store.destroy();
          store.destroy();
          store.destroy();
        }).not.toThrow();
      });
    });

    describe("network errors and agent failures", () => {
      it("should handle agent throwing non-Error objects", async () => {
        mockAgent.runAgent = vi.fn(() => Promise.reject("string error"));

        await expect(store.start({ text: "Hello" })).rejects.toThrow();

        expect(store.status).toBe("error");
        expect(store.error).not.toBeNull();
      });

      it("should handle agent throwing null", async () => {
        mockAgent.runAgent = vi.fn(() => Promise.reject(null));

        await expect(store.start({ text: "Hello" })).rejects.toThrow();

        expect(store.status).toBe("error");
      });

      it("should handle reconnect throwing non-Error objects", async () => {
        mockAgent.connectAgent = vi.fn(() => Promise.reject("reconnect failed"));

        await expect(store.reconnect()).rejects.toThrow();

        expect(store.status).toBe("error");
        // Non-Error rejections get wrapped with a generic message
        expect(store.error?.message).toBe("Failed to reconnect");
      });

      it("should preserve error details from original error", async () => {
        const originalError = new Error("Network timeout");
        originalError.name = "NetworkError";
        mockAgent.runAgent = vi.fn(() => Promise.reject(originalError));

        await expect(store.start({ text: "Hello" })).rejects.toThrow();

        const error = store.error;
        expect(error?.message).toContain("Network timeout");
      });
    });

    describe("subscription cleanup", () => {
      it("should unsubscribe after successful run", async () => {
        const unsubscribe = vi.fn();
        mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));

        await store.start({ text: "Hello" });

        // After run completes, subscription should be cleaned up
        expect(unsubscribe).toHaveBeenCalled();
      });

      it("should unsubscribe after failed run", async () => {
        const unsubscribe = vi.fn();
        mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));
        mockAgent.runAgent = vi.fn(() => Promise.reject(new Error("Failed")));

        await expect(store.start({ text: "Hello" })).rejects.toThrow();

        // Subscription should be cleaned up even on failure
        expect(unsubscribe).toHaveBeenCalled();
      });

      it("should unsubscribe on cancel", async () => {
        const unsubscribe = vi.fn();
        mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));
        mockAgent.runAgent = vi.fn(() => new Promise(() => {})); // Never resolves

        // Start but don't await
        store.start({ text: "Hello" });

        // Cancel
        store.cancel();

        expect(unsubscribe).toHaveBeenCalled();
      });

      it("should handle cancel when no subscription exists", () => {
        // Cancel without starting should not throw
        expect(() => store.cancel()).not.toThrow();
        expect(store.status).toBe("cancelled");
      });

      it("should unsubscribe after reconnect completes", async () => {
        const unsubscribe = vi.fn();
        mockAgent.subscribe = vi.fn(() => ({ unsubscribe }));

        await store.reconnect();

        expect(unsubscribe).toHaveBeenCalled();
      });
    });

    describe("config validation", () => {
      it("should throw on invalid batchIntervalMs (negative)", () => {
        expect(() => {
          createAgentStore(mockAgent, { batchIntervalMs: -1 });
        }).toThrow(/batchIntervalMs/);
      });

      it("should throw on invalid batchIntervalMs (too high)", () => {
        expect(() => {
          createAgentStore(mockAgent, { batchIntervalMs: 2000 });
        }).toThrow(/batchIntervalMs/);
      });

      it("should throw on invalid maxBatchSize (zero)", () => {
        expect(() => {
          createAgentStore(mockAgent, { maxBatchSize: 0 });
        }).toThrow(/maxBatchSize/);
      });

      it("should throw on invalid maxBatchSize (too high)", () => {
        expect(() => {
          createAgentStore(mockAgent, { maxBatchSize: 5000 });
        }).toThrow(/maxBatchSize/);
      });

      it("should accept valid edge case config values", () => {
        // Minimum valid values
        const minStore = createAgentStore(mockAgent, {
          batchIntervalMs: 0,
          maxBatchSize: 1,
        });
        expect(minStore).toBeDefined();

        // Maximum valid values
        const maxStore = createAgentStore(mockAgent, {
          batchIntervalMs: 1000,
          maxBatchSize: 1000,
        });
        expect(maxStore).toBeDefined();
      });
    });
  });
});
