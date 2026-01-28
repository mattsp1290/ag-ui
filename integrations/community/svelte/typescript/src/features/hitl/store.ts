import { writable, derived, type Readable } from "svelte/store";
import type { NormalizedToolCall } from "../../lib/events/types";
import type { HITLConfig, HITLStore, ApprovalResult } from "./types";

/**
 * Create a Human-in-the-Loop store for managing tool call approvals
 */
export function createHITLStore(config: HITLConfig = {}): HITLStore {
  const {
    requireApproval = [],
    autoApprove = [],
    autoApproveTimeout = 0,
  } = config;

  // Internal state
  const pendingQueue = writable<Map<string, NormalizedToolCall>>(new Map());
  const approvalCallbacks = new Map<
    string,
    (result: ApprovalResult) => void
  >();
  const timeoutIds = new Map<string, ReturnType<typeof setTimeout>>();

  // Derived store for pending approvals as array
  const pendingApprovals: Readable<NormalizedToolCall[]> = derived(
    pendingQueue,
    ($queue) => Array.from($queue.values())
  );

  /**
   * Check if a tool requires approval
   */
  function requiresApproval(toolName: string): boolean {
    // If in auto-approve list, never require approval
    if (autoApprove.includes(toolName) || autoApprove.includes("*")) {
      return false;
    }

    // If require list is empty, require all by default
    // If require list has entries, only require those
    if (requireApproval.length === 0) {
      return true;
    }

    return (
      requireApproval.includes(toolName) || requireApproval.includes("*")
    );
  }

  /**
   * Clear timeout for a tool call
   */
  function clearApprovalTimeout(toolCallId: string): void {
    const timeoutId = timeoutIds.get(toolCallId);
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutIds.delete(toolCallId);
    }
  }

  /**
   * Remove a tool call from the pending queue
   */
  function removeFromQueue(toolCallId: string): void {
    clearApprovalTimeout(toolCallId);
    pendingQueue.update((queue) => {
      const newQueue = new Map(queue);
      newQueue.delete(toolCallId);
      return newQueue;
    });
  }

  /**
   * Execute callback for a tool call
   */
  function executeCallback(result: ApprovalResult): void {
    const callback = approvalCallbacks.get(result.toolCallId);
    if (callback) {
      callback(result);
      approvalCallbacks.delete(result.toolCallId);
    }
  }

  /**
   * Approve a tool call
   */
  function approve(toolCallId: string): void {
    const result: ApprovalResult = {
      decision: "approve",
      toolCallId,
    };
    removeFromQueue(toolCallId);
    executeCallback(result);
  }

  /**
   * Reject a tool call
   */
  function reject(toolCallId: string, reason?: string): void {
    const result: ApprovalResult = {
      decision: "reject",
      toolCallId,
      reason,
    };
    removeFromQueue(toolCallId);
    executeCallback(result);
  }

  /**
   * Modify and approve a tool call
   */
  function modify(toolCallId: string, newArguments: string): void {
    const result: ApprovalResult = {
      decision: "modify",
      toolCallId,
      modifiedArguments: newArguments,
    };
    removeFromQueue(toolCallId);
    executeCallback(result);
  }

  /**
   * Request approval for a tool call programmatically.
   * Returns a Promise that resolves when the user makes a decision.
   *
   * @param toolCall - The tool call requiring approval
   * @returns Promise that resolves with the approval result
   */
  function requestApproval(toolCall: NormalizedToolCall): Promise<ApprovalResult> {
    return new Promise((resolve) => {
      // Add to pending queue
      pendingQueue.update((queue) => {
        const newQueue = new Map(queue);
        newQueue.set(toolCall.id, toolCall);
        return newQueue;
      });

      // Register callback
      approvalCallbacks.set(toolCall.id, resolve);

      // Set up auto-approval timeout if configured
      if (autoApproveTimeout > 0) {
        const timeoutId = setTimeout(() => {
          // Auto-approve if still pending
          const result: ApprovalResult = {
            decision: "approve",
            toolCallId: toolCall.id,
          };
          removeFromQueue(toolCall.id);
          const callback = approvalCallbacks.get(toolCall.id);
          if (callback) {
            callback(result);
            approvalCallbacks.delete(toolCall.id);
          }
        }, autoApproveTimeout);
        timeoutIds.set(toolCall.id, timeoutId);
      }
    });
  }

  /**
   * Clean up all pending timeouts and callbacks.
   * Call this when the component using this store unmounts.
   */
  function destroy(): void {
    timeoutIds.forEach((id) => clearTimeout(id));
    timeoutIds.clear();
    approvalCallbacks.clear();
    pendingQueue.set(new Map());
  }

  return {
    pendingApprovals,
    approve,
    reject,
    modify,
    requiresApproval,
    requestApproval,
    destroy,
  };
}

/**
 * Utility to integrate HITL store with agent store
 */
export function withHITL<T extends { toolCalls: Readable<NormalizedToolCall[]> }>(
  agentStore: T,
  hitlStore: HITLStore
): T & { hitl: HITLStore } {
  return {
    ...agentStore,
    hitl: hitlStore,
  };
}
