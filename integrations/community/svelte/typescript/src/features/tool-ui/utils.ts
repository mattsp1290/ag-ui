import type { NormalizedToolCall, NormalizedActivity } from "../../lib/events/types";
import type { ToolCallGroup, TimelineEntry, ToolBasedUIState } from "./types";

/**
 * Create initial ToolBasedUI state
 */
export function createToolUIState(): ToolBasedUIState {
  return {
    expandedToolCalls: new Set(),
    expandedActivities: new Set(),
    sortOrder: "chronological",
  };
}

/**
 * Group tool calls by status
 */
export function groupToolCallsByStatus(
  toolCalls: NormalizedToolCall[]
): ToolCallGroup[] {
  const pending = toolCalls.filter(
    (tc) => tc.status === "pending" || tc.status === "streaming"
  );
  const completed = toolCalls.filter((tc) => tc.status === "completed");

  const groups: ToolCallGroup[] = [];

  if (pending.length > 0) {
    groups.push({
      label: "In Progress",
      toolCalls: pending,
    });
  }

  if (completed.length > 0) {
    groups.push({
      label: "Completed",
      toolCalls: completed,
      collapsed: true,
    });
  }

  return groups;
}

/**
 * Group tool calls by name
 */
export function groupToolCallsByName(
  toolCalls: NormalizedToolCall[]
): ToolCallGroup[] {
  const groups = new Map<string, NormalizedToolCall[]>();

  for (const tc of toolCalls) {
    const existing = groups.get(tc.name) ?? [];
    existing.push(tc);
    groups.set(tc.name, existing);
  }

  return Array.from(groups.entries()).map(([name, calls]) => ({
    label: name,
    toolCalls: calls,
  }));
}

/**
 * Filter tool calls by name pattern
 */
export function filterToolCalls(
  toolCalls: NormalizedToolCall[],
  pattern?: string
): NormalizedToolCall[] {
  if (!pattern) return toolCalls;
  const lowerPattern = pattern.toLowerCase();
  return toolCalls.filter((tc) =>
    tc.name.toLowerCase().includes(lowerPattern)
  );
}

/**
 * Sort tool calls by various criteria
 */
export function sortToolCalls(
  toolCalls: NormalizedToolCall[],
  order: ToolBasedUIState["sortOrder"]
): NormalizedToolCall[] {
  const sorted = [...toolCalls];

  switch (order) {
    case "status":
      const statusOrder: Record<NormalizedToolCall["status"], number> = {
        pending: 0,
        streaming: 1,
        completed: 2,
        error: 3,
      };
      sorted.sort((a, b) => statusOrder[a.status] - statusOrder[b.status]);
      break;
    case "name":
      sorted.sort((a, b) => a.name.localeCompare(b.name));
      break;
    case "chronological":
    default:
      // Already in chronological order from the store
      break;
  }

  return sorted;
}

/**
 * Build a timeline from tool calls and activities
 */
export function buildTimeline(
  toolCalls: NormalizedToolCall[],
  activities: NormalizedActivity[]
): TimelineEntry[] {
  const entries: TimelineEntry[] = [];

  // Add tool call entries
  for (const tc of toolCalls) {
    entries.push({
      type: "tool_start",
      id: tc.id,
      data: tc,
    });

    if (tc.status === "completed") {
      entries.push({
        type: "tool_end",
        id: `${tc.id}-end`,
        data: tc,
      });
    }
  }

  // Add activity entries
  for (const activity of activities) {
    entries.push({
      type: "activity",
      timestamp: activity.timestamp,
      id: activity.id,
      data: activity,
    });
  }

  // Sort by timestamp if available
  entries.sort((a, b) => {
    const aTime = a.timestamp ?? 0;
    const bTime = b.timestamp ?? 0;
    return aTime - bTime;
  });

  return entries;
}

/**
 * Get statistics about tool calls
 */
export function getToolCallStats(toolCalls: NormalizedToolCall[]): {
  total: number;
  pending: number;
  streaming: number;
  completed: number;
  uniqueTools: number;
} {
  const uniqueNames = new Set(toolCalls.map((tc) => tc.name));

  return {
    total: toolCalls.length,
    pending: toolCalls.filter((tc) => tc.status === "pending").length,
    streaming: toolCalls.filter((tc) => tc.status === "streaming").length,
    completed: toolCalls.filter((tc) => tc.status === "completed").length,
    uniqueTools: uniqueNames.size,
  };
}

/**
 * Toggle a tool call's expanded state
 */
export function toggleToolCallExpanded(
  state: ToolBasedUIState,
  toolCallId: string
): ToolBasedUIState {
  const newExpanded = new Set(state.expandedToolCalls);
  if (newExpanded.has(toolCallId)) {
    newExpanded.delete(toolCallId);
  } else {
    newExpanded.add(toolCallId);
  }
  return { ...state, expandedToolCalls: newExpanded };
}

/**
 * Toggle an activity's expanded state
 */
export function toggleActivityExpanded(
  state: ToolBasedUIState,
  activityId: string
): ToolBasedUIState {
  const newExpanded = new Set(state.expandedActivities);
  if (newExpanded.has(activityId)) {
    newExpanded.delete(activityId);
  } else {
    newExpanded.add(activityId);
  }
  return { ...state, expandedActivities: newExpanded };
}

/**
 * Expand all tool calls
 */
export function expandAllToolCalls(
  state: ToolBasedUIState,
  toolCalls: NormalizedToolCall[]
): ToolBasedUIState {
  return {
    ...state,
    expandedToolCalls: new Set(toolCalls.map((tc) => tc.id)),
  };
}

/**
 * Collapse all tool calls
 */
export function collapseAllToolCalls(state: ToolBasedUIState): ToolBasedUIState {
  return {
    ...state,
    expandedToolCalls: new Set(),
  };
}
