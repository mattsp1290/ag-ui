import type { NormalizedToolCall } from "../../lib/events/types";

/**
 * Base props for tool components
 */
export interface BaseToolProps {
  /** Additional CSS class names */
  class?: string;
}

/**
 * Props for ToolCallCard component
 */
export interface ToolCallCardProps extends BaseToolProps {
  /** The tool call to display */
  toolCall: NormalizedToolCall;
  /** Whether to show the arguments */
  showArguments?: boolean;
  /** Whether to show the result */
  showResult?: boolean;
  /** Whether the card is expanded */
  expanded?: boolean;
  /** Callback when expansion state changes */
  onToggle?: (expanded: boolean) => void;
}

/**
 * Props for ToolCallList component
 */
export interface ToolCallListProps extends BaseToolProps {
  /** The tool calls to display */
  toolCalls: NormalizedToolCall[];
  /** Whether to show empty state when no tool calls */
  showEmpty?: boolean;
  /** Empty state message */
  emptyMessage?: string;
}

/**
 * Props for ToolResult component
 */
export interface ToolResultProps extends BaseToolProps {
  /** The result content */
  result: string;
  /** Whether the result is an error */
  isError?: boolean;
  /** Maximum height before scrolling */
  maxHeight?: string;
}

/**
 * Tool call status display configuration
 */
export interface ToolStatusConfig {
  pending: { label: string; icon?: string };
  streaming: { label: string; icon?: string };
  completed: { label: string; icon?: string };
  error: { label: string; icon?: string };
}

/**
 * Default status configuration
 */
export const defaultToolStatusConfig: ToolStatusConfig = {
  pending: { label: "Pending" },
  streaming: { label: "Running" },
  completed: { label: "Completed" },
  error: { label: "Error" },
};
