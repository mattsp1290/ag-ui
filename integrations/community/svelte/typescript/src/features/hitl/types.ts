import type { NormalizedToolCall } from "../../lib/events/types";
import type { Readable } from "svelte/store";

/**
 * Approval decision
 */
export type ApprovalDecision = "approve" | "reject" | "modify";

/**
 * Result of an approval action
 */
export interface ApprovalResult {
  decision: ApprovalDecision;
  toolCallId: string;
  modifiedArguments?: string;
  reason?: string;
}

/**
 * Props for ApprovalCard component
 */
export interface ApprovalCardProps {
  /** The tool call requiring approval */
  toolCall: NormalizedToolCall;
  /** Title for the approval card */
  title?: string;
  /** Description/instructions */
  description?: string;
  /** Allow modifying arguments before approval */
  allowModify?: boolean;
  /** Require a reason for rejection */
  requireRejectReason?: boolean;
  /** Callback when decision is made */
  onDecision: (result: ApprovalResult) => void;
  /** Additional CSS class names */
  class?: string;
}

/**
 * Props for ApprovalQueue component
 */
export interface ApprovalQueueProps {
  /** Tool calls pending approval */
  pendingApprovals: NormalizedToolCall[];
  /** Callback when a decision is made */
  onDecision: (result: ApprovalResult) => void;
  /** Empty state message */
  emptyMessage?: string;
  /** Additional CSS class names */
  class?: string;
}

/**
 * Human-in-the-loop store configuration
 */
export interface HITLConfig {
  /** Tool names that require approval */
  requireApproval?: string[];
  /** Tool names that never require approval */
  autoApprove?: string[];
  /** Auto-approve after timeout (ms), 0 = never */
  autoApproveTimeout?: number;
}

/**
 * Human-in-the-loop store interface
 */
export interface HITLStore {
  /** Pending approvals */
  pendingApprovals: Readable<NormalizedToolCall[]>;
  /** Approve a tool call */
  approve(toolCallId: string): void;
  /** Reject a tool call */
  reject(toolCallId: string, reason?: string): void;
  /** Modify and approve a tool call */
  modify(toolCallId: string, newArguments: string): void;
  /** Check if a tool requires approval */
  requiresApproval(toolName: string): boolean;
  /**
   * Request approval for a tool call programmatically.
   * Returns a Promise that resolves when the user makes a decision.
   */
  requestApproval(toolCall: NormalizedToolCall): Promise<ApprovalResult>;
  /**
   * Clean up all pending timeouts and callbacks.
   * Call this when the component using this store unmounts.
   */
  destroy(): void;
}
