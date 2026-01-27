import type { NormalizedActivity } from "../../lib/events/types";
import type { ActivityStatus, ActivityMetadata } from "./types";

/**
 * Determine the status of an activity
 */
export function getActivityStatus(
  activity: NormalizedActivity,
  isUpdating = false
): ActivityStatus {
  if (isUpdating) return "updating";
  // Activities are typically complete when we receive them
  return "complete";
}

/**
 * Get CSS class for activity status
 */
export function getActivityStatusClass(status: ActivityStatus): string {
  return `activity--${status}`;
}

/**
 * Extract metadata from activity content if available
 */
export function extractActivityMetadata(
  activity: NormalizedActivity
): ActivityMetadata {
  const content = activity.content as Record<string, unknown> | undefined;

  return {
    title: typeof content?.title === "string" ? content.title : undefined,
    description:
      typeof content?.description === "string" ? content.description : undefined,
    icon: typeof content?.icon === "string" ? content.icon : undefined,
    expandable:
      typeof content?.expandable === "boolean" ? content.expandable : undefined,
    defaultExpanded:
      typeof content?.defaultExpanded === "boolean"
        ? content.defaultExpanded
        : undefined,
  };
}

/**
 * Format activity type for display
 */
export function formatActivityType(type: string): string {
  // Convert snake_case or kebab-case to Title Case
  return type
    .replace(/[_-]/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

/**
 * Get a display title for an activity
 */
export function getActivityTitle(activity: NormalizedActivity): string {
  const metadata = extractActivityMetadata(activity);
  return metadata.title ?? formatActivityType(activity.type);
}

/**
 * Check if activity content has changed significantly
 * Useful for deciding whether to trigger animations
 */
export function hasActivityContentChanged(
  prev: NormalizedActivity | undefined,
  next: NormalizedActivity
): boolean {
  if (!prev) return true;
  if (prev.id !== next.id) return true;

  // Compare content by JSON serialization
  try {
    return JSON.stringify(prev.content) !== JSON.stringify(next.content);
  } catch {
    return true;
  }
}

/**
 * Get a preview of activity content
 */
export function getActivityPreview(
  activity: NormalizedActivity,
  maxLength = 100
): string {
  const content = activity.content;

  if (typeof content === "string") {
    if (content.length <= maxLength) return content;
    return content.substring(0, maxLength - 3) + "...";
  }

  if (typeof content === "object" && content !== null) {
    try {
      const json = JSON.stringify(content);
      if (json.length <= maxLength) return json;
      return json.substring(0, maxLength - 3) + "...";
    } catch {
      return "[Complex content]";
    }
  }

  return String(content);
}

/**
 * Check if activity has visual content (images, charts, etc.)
 */
export function hasVisualContent(activity: NormalizedActivity): boolean {
  const content = activity.content as Record<string, unknown> | undefined;
  if (!content) return false;

  // Check for common visual content indicators
  return (
    "imageUrl" in content ||
    "chartData" in content ||
    "svgContent" in content ||
    "html" in content ||
    activity.type.includes("chart") ||
    activity.type.includes("image") ||
    activity.type.includes("graph") ||
    activity.type.includes("visual")
  );
}

/**
 * Get default icon for activity type
 */
export function getActivityIcon(type: string): string {
  // Return common icon identifiers based on activity type
  const typeLC = type.toLowerCase();

  if (typeLC.includes("chart") || typeLC.includes("graph")) return "chart";
  if (typeLC.includes("image")) return "image";
  if (typeLC.includes("table")) return "table";
  if (typeLC.includes("list")) return "list";
  if (typeLC.includes("form") || typeLC.includes("input")) return "form";
  if (typeLC.includes("code")) return "code";
  if (typeLC.includes("search")) return "search";
  if (typeLC.includes("loading") || typeLC.includes("progress")) return "loading";
  if (typeLC.includes("error")) return "error";
  if (typeLC.includes("success")) return "success";
  if (typeLC.includes("warning")) return "warning";

  return "activity"; // default
}
