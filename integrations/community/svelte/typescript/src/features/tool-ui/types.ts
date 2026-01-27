import type { NormalizedToolCall, NormalizedActivity } from "../../lib/events/types";
import type { AgentStore } from "../../stores/run/types";
import type { ActivityRegistry } from "../../lib/activity";

/**
 * Configuration for the ToolBasedUI template
 */
export interface ToolBasedUIConfig {
  /** Whether to show tool calls inline with messages */
  showToolCallsInline?: boolean;
  /** Whether to auto-expand tool call details */
  autoExpandToolCalls?: boolean;
  /** Maximum tool calls to show before collapsing */
  maxVisibleToolCalls?: number;
  /** Whether to show tool results */
  showToolResults?: boolean;
  /** Activity registry for rendering custom activities */
  activityRegistry?: ActivityRegistry;
  /** Whether to render activities inline */
  showActivitiesInline?: boolean;
  /** Custom renderer for tool calls */
  toolCallRenderer?: (toolCall: NormalizedToolCall) => unknown;
  /** Custom renderer for activities */
  activityRenderer?: (activity: NormalizedActivity) => unknown;
}

/**
 * Props for ToolBasedUI container component
 */
export interface ToolBasedUIProps {
  /** The agent store */
  store: AgentStore;
  /** Configuration options */
  config?: ToolBasedUIConfig;
  /** Additional CSS classes */
  class?: string;
}

/**
 * State for the ToolBasedUI template
 */
export interface ToolBasedUIState {
  /** Currently expanded tool call IDs */
  expandedToolCalls: Set<string>;
  /** Currently expanded activity IDs */
  expandedActivities: Set<string>;
  /** Filter for tool calls by name */
  toolCallFilter?: string;
  /** Sort order for tool calls */
  sortOrder: "chronological" | "status" | "name";
}

/**
 * Tool call group for display
 */
export interface ToolCallGroup {
  /** Group label */
  label: string;
  /** Tool calls in this group */
  toolCalls: NormalizedToolCall[];
  /** Whether the group is collapsed */
  collapsed?: boolean;
}

/**
 * Tool execution timeline entry
 */
export interface TimelineEntry {
  /** Entry type */
  type: "tool_start" | "tool_end" | "activity" | "message";
  /** Timestamp */
  timestamp?: number;
  /** Reference ID */
  id: string;
  /** Associated data */
  data: NormalizedToolCall | NormalizedActivity | { content: string };
}
