import type { NormalizedToolCall } from "../../lib/events/types";
import type { HITLStore } from "../../features/hitl/types";

/**
 * Base props for HITL components
 */
export interface BaseHITLProps {
  /** Additional CSS classes */
  class?: string;
}

/**
 * Props for ApprovalPrompt component
 */
export interface ApprovalPromptProps extends BaseHITLProps {
  /** The tool call awaiting approval */
  toolCall: NormalizedToolCall;
  /** Callback when approved */
  onApprove?: () => void;
  /** Callback when rejected */
  onReject?: (reason?: string) => void;
  /** Callback when modified and approved */
  onModify?: (newArguments: string) => void;
  /** Whether to show a modify option */
  allowModify?: boolean;
  /** Whether to show a reason input for rejection */
  allowRejectReason?: boolean;
  /** Custom approve button text */
  approveText?: string;
  /** Custom reject button text */
  rejectText?: string;
}

/**
 * Props for ActionButtons component
 */
export interface ActionButtonsProps extends BaseHITLProps {
  /** Whether actions are disabled */
  disabled?: boolean;
  /** Whether an action is in progress */
  loading?: boolean;
  /** Callback for approve action */
  onApprove?: () => void;
  /** Callback for reject action */
  onReject?: () => void;
  /** Callback for modify action */
  onModify?: () => void;
  /** Whether to show modify button */
  showModify?: boolean;
  /** Size of buttons */
  size?: "small" | "medium" | "large";
}

/**
 * Props for RunStatus component
 */
export interface RunStatusProps extends BaseHITLProps {
  /** Current run status */
  status: "idle" | "starting" | "running" | "paused" | "completed" | "error" | "cancelled";
  /** Whether there are pending approvals */
  hasPendingApprovals?: boolean;
  /** Number of pending approvals */
  pendingCount?: number;
  /** Current step name if any */
  currentStep?: string | null;
  /** Error message if in error state */
  errorMessage?: string;
  /** Whether to show cancel button */
  showCancel?: boolean;
  /** Cancel callback */
  onCancel?: () => void;
}

/**
 * Props for ApprovalQueue component
 */
export interface ApprovalQueueProps extends BaseHITLProps {
  /** HITL store instance */
  store: HITLStore;
  /** Tool calls pending approval */
  pending: NormalizedToolCall[];
  /** Whether to auto-scroll to new items */
  autoScroll?: boolean;
  /** Maximum items to show before collapsing */
  maxVisible?: number;
}

/**
 * Approval action type
 */
export type ApprovalAction = "approve" | "reject" | "modify";

/**
 * Status display configuration
 */
export interface StatusConfig {
  /** Label to display */
  label: string;
  /** Icon identifier */
  icon?: string;
  /** Color class */
  colorClass?: string;
  /** Whether this status indicates activity */
  isActive?: boolean;
}

/**
 * Default status configurations
 */
export const defaultStatusConfigs: Record<RunStatusProps["status"], StatusConfig> = {
  idle: { label: "Idle", icon: "circle", colorClass: "status--idle" },
  starting: { label: "Starting...", icon: "loader", colorClass: "status--starting", isActive: true },
  running: { label: "Running", icon: "play", colorClass: "status--running", isActive: true },
  paused: { label: "Awaiting Approval", icon: "pause", colorClass: "status--paused", isActive: true },
  completed: { label: "Completed", icon: "check", colorClass: "status--completed" },
  error: { label: "Error", icon: "alert", colorClass: "status--error" },
  cancelled: { label: "Cancelled", icon: "x", colorClass: "status--cancelled" },
};
