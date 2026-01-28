import type { NormalizedToolCall } from "../../lib/events/types";
import type { RunStatusProps, StatusConfig, ApprovalAction } from "./types";
import { defaultStatusConfigs } from "./types";

/**
 * Get status configuration for a run status
 */
export function getStatusConfig(status: RunStatusProps["status"]): StatusConfig {
  return defaultStatusConfigs[status];
}

/**
 * Get CSS class for run status
 */
export function getStatusClass(status: RunStatusProps["status"]): string {
  return `run-status--${status}`;
}

/**
 * Check if a run status indicates active processing
 */
export function isStatusActive(status: RunStatusProps["status"]): boolean {
  return defaultStatusConfigs[status]?.isActive ?? false;
}

/**
 * Get status text including pending count
 */
export function getStatusText(
  status: RunStatusProps["status"],
  pendingCount?: number
): string {
  const config = getStatusConfig(status);
  if (status === "paused" && pendingCount && pendingCount > 0) {
    return `${config.label} (${pendingCount} pending)`;
  }
  return config.label;
}

/**
 * Check if a tool call is high risk (based on name heuristics)
 */
export function isHighRiskToolCall(toolCall: NormalizedToolCall): boolean {
  const riskyPatterns = [
    /delete/i,
    /remove/i,
    /drop/i,
    /destroy/i,
    /execute/i,
    /shell/i,
    /bash/i,
    /cmd/i,
    /write/i,
    /create/i,
    /modify/i,
    /update/i,
    /send/i,
    /publish/i,
    /deploy/i,
    /transfer/i,
    /payment/i,
  ];

  return riskyPatterns.some((pattern) => pattern.test(toolCall.name));
}

/**
 * Get risk level for a tool call
 */
export function getToolCallRiskLevel(
  toolCall: NormalizedToolCall
): "low" | "medium" | "high" {
  if (isHighRiskToolCall(toolCall)) {
    return "high";
  }

  // Medium risk for tool calls with large arguments
  if (toolCall.arguments.length > 1000) {
    return "medium";
  }

  return "low";
}

/**
 * Get CSS class for risk level
 */
export function getRiskLevelClass(level: "low" | "medium" | "high"): string {
  return `risk--${level}`;
}

/**
 * Format tool call for approval display
 */
export function formatToolCallForApproval(toolCall: NormalizedToolCall): {
  name: string;
  summary: string;
  arguments: string;
} {
  let args = toolCall.arguments;
  try {
    const parsed = JSON.parse(args);
    args = JSON.stringify(parsed, null, 2);
  } catch {
    // Use raw arguments if not valid JSON
  }

  // Create a summary from arguments
  let summary = args.substring(0, 100);
  if (args.length > 100) {
    summary += "...";
  }

  return {
    name: toolCall.name,
    summary,
    arguments: args,
  };
}

/**
 * Get keyboard shortcut hint for action
 */
export function getActionShortcut(action: ApprovalAction): string | null {
  switch (action) {
    case "approve":
      return "Enter";
    case "reject":
      return "Escape";
    case "modify":
      return "Ctrl+E";
    default:
      return null;
  }
}

/**
 * Create a rejection reason from common templates
 */
export function createRejectionReason(
  template: "unsafe" | "unnecessary" | "incorrect" | "other",
  customReason?: string
): string {
  switch (template) {
    case "unsafe":
      return "This operation appears unsafe and was rejected.";
    case "unnecessary":
      return "This operation is not necessary for the current task.";
    case "incorrect":
      return "The parameters for this operation appear incorrect.";
    case "other":
      return customReason ?? "User rejected this operation.";
  }
}

/**
 * Sort pending approvals by priority (high risk first)
 */
export function sortPendingByPriority(
  pending: NormalizedToolCall[]
): NormalizedToolCall[] {
  return [...pending].sort((a, b) => {
    const aRisk = getToolCallRiskLevel(a);
    const bRisk = getToolCallRiskLevel(b);
    const riskOrder = { high: 0, medium: 1, low: 2 };
    return riskOrder[aRisk] - riskOrder[bRisk];
  });
}
