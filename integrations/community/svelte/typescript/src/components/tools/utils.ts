import type { NormalizedToolCall } from "../../lib/events/types";
import { defaultToolStatusConfig, type ToolStatusConfig } from "./types";

/**
 * Get status display info for a tool call
 */
export function getToolStatus(
  toolCall: NormalizedToolCall,
  config: ToolStatusConfig = defaultToolStatusConfig
): { label: string; icon?: string } {
  return config[toolCall.status];
}

/**
 * Format tool arguments for display
 */
export function formatToolArguments(
  toolCall: NormalizedToolCall,
  indent = 2
): string {
  if (toolCall.parsedArguments) {
    try {
      return JSON.stringify(toolCall.parsedArguments, null, indent);
    } catch {
      return toolCall.arguments;
    }
  }

  // Try to parse and format
  try {
    const parsed = JSON.parse(toolCall.arguments);
    return JSON.stringify(parsed, null, indent);
  } catch {
    return toolCall.arguments;
  }
}

/**
 * Check if a tool call is still active (not completed or errored)
 */
export function isToolCallActive(toolCall: NormalizedToolCall): boolean {
  return toolCall.status === "pending" || toolCall.status === "streaming";
}

/**
 * Get a brief summary of tool arguments
 */
export function getToolArgumentsSummary(
  toolCall: NormalizedToolCall,
  maxLength = 50
): string {
  const formatted = formatToolArguments(toolCall, 0);
  if (formatted.length <= maxLength) {
    return formatted;
  }
  return formatted.substring(0, maxLength - 3) + "...";
}

/**
 * Parse tool result as JSON if possible
 */
export function parseToolResult(result: string): unknown {
  try {
    return JSON.parse(result);
  } catch {
    return result;
  }
}

/**
 * Format tool result for display
 */
export function formatToolResult(result: string, indent = 2): string {
  try {
    const parsed = JSON.parse(result);
    return JSON.stringify(parsed, null, indent);
  } catch {
    return result;
  }
}
