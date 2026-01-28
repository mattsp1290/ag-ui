import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { get } from "svelte/store";
import { createHITLStore, withHITL } from "../store";
import type { HITLStore } from "../types";
import type { NormalizedToolCall } from "../../../lib/events/types";

/**
 * Create a mock tool call for testing
 */
function createMockToolCall(overrides: Partial<NormalizedToolCall> = {}): NormalizedToolCall {
  return {
    id: `tc-${Math.random().toString(36).substring(7)}`,
    name: "test-tool",
    arguments: '{"key": "value"}',
    status: "completed",
    ...overrides,
  };
}

describe("HITL Store", () => {
  let store: HITLStore;

  beforeEach(() => {
    vi.useFakeTimers();
    store = createHITLStore();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("requiresApproval()", () => {
    it("should require approval for all tools by default", () => {
      expect(store.requiresApproval("search")).toBe(true);
      expect(store.requiresApproval("execute")).toBe(true);
      expect(store.requiresApproval("any-tool")).toBe(true);
    });

    it("should not require approval for tools in autoApprove list", () => {
      const storeWithAutoApprove = createHITLStore({
        autoApprove: ["safe-tool"],
      });

      expect(storeWithAutoApprove.requiresApproval("safe-tool")).toBe(false);
      expect(storeWithAutoApprove.requiresApproval("other-tool")).toBe(true);
    });

    it("should handle wildcard in autoApprove", () => {
      const storeWithWildcard = createHITLStore({
        autoApprove: ["*"],
      });

      expect(storeWithWildcard.requiresApproval("any-tool")).toBe(false);
      expect(storeWithWildcard.requiresApproval("another-tool")).toBe(false);
    });

    it("should only require approval for tools in requireApproval list", () => {
      const storeWithRequireList = createHITLStore({
        requireApproval: ["dangerous-tool"],
      });

      expect(storeWithRequireList.requiresApproval("dangerous-tool")).toBe(true);
      expect(storeWithRequireList.requiresApproval("safe-tool")).toBe(false);
    });

    it("should handle wildcard in requireApproval", () => {
      const storeWithWildcard = createHITLStore({
        requireApproval: ["*"],
      });

      expect(storeWithWildcard.requiresApproval("any-tool")).toBe(true);
    });

    it("should prioritize autoApprove over requireApproval", () => {
      const store = createHITLStore({
        requireApproval: ["tool-a", "tool-b"],
        autoApprove: ["tool-a"],
      });

      expect(store.requiresApproval("tool-a")).toBe(false);
      expect(store.requiresApproval("tool-b")).toBe(true);
    });
  });

  describe("approve()", () => {
    it("should resolve requestApproval promise with approve decision", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });

      const approvalPromise = store.requestApproval(toolCall);

      // Approve the tool call
      store.approve("tc-1");

      const result = await approvalPromise;
      expect(result.decision).toBe("approve");
      expect(result.toolCallId).toBe("tc-1");
    });

    it("should remove tool call from pending queue", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });
      store.requestApproval(toolCall);

      expect(get(store.pendingApprovals)).toHaveLength(1);

      store.approve("tc-1");

      expect(get(store.pendingApprovals)).toHaveLength(0);
    });
  });

  describe("reject()", () => {
    it("should resolve requestApproval promise with reject decision", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });

      const approvalPromise = store.requestApproval(toolCall);

      store.reject("tc-1", "Not allowed");

      const result = await approvalPromise;
      expect(result.decision).toBe("reject");
      expect(result.toolCallId).toBe("tc-1");
      expect(result.reason).toBe("Not allowed");
    });

    it("should work without a reason", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });

      const approvalPromise = store.requestApproval(toolCall);

      store.reject("tc-1");

      const result = await approvalPromise;
      expect(result.decision).toBe("reject");
      expect(result.reason).toBeUndefined();
    });

    it("should remove tool call from pending queue", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });
      store.requestApproval(toolCall);

      expect(get(store.pendingApprovals)).toHaveLength(1);

      store.reject("tc-1");

      expect(get(store.pendingApprovals)).toHaveLength(0);
    });
  });

  describe("modify()", () => {
    it("should resolve requestApproval promise with modify decision", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });

      const approvalPromise = store.requestApproval(toolCall);

      store.modify("tc-1", '{"modified": true}');

      const result = await approvalPromise;
      expect(result.decision).toBe("modify");
      expect(result.toolCallId).toBe("tc-1");
      expect(result.modifiedArguments).toBe('{"modified": true}');
    });

    it("should remove tool call from pending queue", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });
      store.requestApproval(toolCall);

      expect(get(store.pendingApprovals)).toHaveLength(1);

      store.modify("tc-1", '{}');

      expect(get(store.pendingApprovals)).toHaveLength(0);
    });
  });

  describe("requestApproval()", () => {
    it("should add tool call to pending queue", () => {
      const toolCall = createMockToolCall({ id: "tc-1", name: "search" });

      store.requestApproval(toolCall);

      const pending = get(store.pendingApprovals);
      expect(pending).toHaveLength(1);
      expect(pending[0].id).toBe("tc-1");
      expect(pending[0].name).toBe("search");
    });

    it("should handle multiple pending approvals", () => {
      const toolCall1 = createMockToolCall({ id: "tc-1" });
      const toolCall2 = createMockToolCall({ id: "tc-2" });
      const toolCall3 = createMockToolCall({ id: "tc-3" });

      store.requestApproval(toolCall1);
      store.requestApproval(toolCall2);
      store.requestApproval(toolCall3);

      expect(get(store.pendingApprovals)).toHaveLength(3);
    });

    it("should return a promise that resolves with the decision", async () => {
      const toolCall = createMockToolCall({ id: "tc-1" });

      const approvalPromise = store.requestApproval(toolCall);

      // Approve should resolve the promise
      store.approve("tc-1");

      const result = await approvalPromise;
      expect(result).toBeDefined();
      expect(result.toolCallId).toBe("tc-1");
    });
  });

  describe("auto-approval timeout", () => {
    it("should auto-approve after timeout when configured", async () => {
      const storeWithTimeout = createHITLStore({
        autoApproveTimeout: 5000,
      });

      const toolCall = createMockToolCall({ id: "tc-1" });
      const approvalPromise = storeWithTimeout.requestApproval(toolCall);

      expect(get(storeWithTimeout.pendingApprovals)).toHaveLength(1);

      // Advance time past the timeout
      vi.advanceTimersByTime(5000);

      const result = await approvalPromise;
      expect(result.decision).toBe("approve");
      expect(get(storeWithTimeout.pendingApprovals)).toHaveLength(0);
    });

    it("should not auto-approve if manually approved before timeout", async () => {
      const storeWithTimeout = createHITLStore({
        autoApproveTimeout: 5000,
      });

      const toolCall = createMockToolCall({ id: "tc-1" });
      const approvalPromise = storeWithTimeout.requestApproval(toolCall);

      // Manually approve before timeout
      storeWithTimeout.approve("tc-1");

      const result = await approvalPromise;
      expect(result.decision).toBe("approve");

      // Advance time - should not cause issues
      vi.advanceTimersByTime(5000);
    });

    it("should not auto-approve when timeout is 0", async () => {
      const storeNoTimeout = createHITLStore({
        autoApproveTimeout: 0,
      });

      const toolCall = createMockToolCall({ id: "tc-1" });
      storeNoTimeout.requestApproval(toolCall);

      expect(get(storeNoTimeout.pendingApprovals)).toHaveLength(1);

      // Advance time significantly
      vi.advanceTimersByTime(60000);

      // Should still be pending
      expect(get(storeNoTimeout.pendingApprovals)).toHaveLength(1);
    });

    it("should clear timeout when manually rejected", async () => {
      const storeWithTimeout = createHITLStore({
        autoApproveTimeout: 5000,
      });

      const toolCall = createMockToolCall({ id: "tc-1" });
      const approvalPromise = storeWithTimeout.requestApproval(toolCall);

      // Reject before timeout
      storeWithTimeout.reject("tc-1", "Rejected");

      const result = await approvalPromise;
      expect(result.decision).toBe("reject");

      // Advance time - should not auto-approve (already handled)
      vi.advanceTimersByTime(5000);

      // No pending approvals
      expect(get(storeWithTimeout.pendingApprovals)).toHaveLength(0);
    });
  });

  describe("pending queue management", () => {
    it("should maintain order of pending approvals", () => {
      const toolCall1 = createMockToolCall({ id: "tc-1", name: "first" });
      const toolCall2 = createMockToolCall({ id: "tc-2", name: "second" });
      const toolCall3 = createMockToolCall({ id: "tc-3", name: "third" });

      store.requestApproval(toolCall1);
      store.requestApproval(toolCall2);
      store.requestApproval(toolCall3);

      const pending = get(store.pendingApprovals);
      expect(pending.map((tc) => tc.id)).toEqual(["tc-1", "tc-2", "tc-3"]);
    });

    it("should handle approving middle item in queue", () => {
      const toolCall1 = createMockToolCall({ id: "tc-1" });
      const toolCall2 = createMockToolCall({ id: "tc-2" });
      const toolCall3 = createMockToolCall({ id: "tc-3" });

      store.requestApproval(toolCall1);
      store.requestApproval(toolCall2);
      store.requestApproval(toolCall3);

      // Approve the middle one
      store.approve("tc-2");

      const pending = get(store.pendingApprovals);
      expect(pending).toHaveLength(2);
      expect(pending.map((tc) => tc.id)).toEqual(["tc-1", "tc-3"]);
    });
  });

  describe("withHITL()", () => {
    it("should combine agent store with HITL store", () => {
      const mockAgentStore = {
        toolCalls: { subscribe: vi.fn() } as any,
        messages: { subscribe: vi.fn() } as any,
      };

      const combined = withHITL(mockAgentStore, store);

      expect(combined.toolCalls).toBe(mockAgentStore.toolCalls);
      expect(combined.hitl).toBe(store);
    });
  });
});
