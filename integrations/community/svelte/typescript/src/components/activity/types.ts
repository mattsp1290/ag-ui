import type { NormalizedActivity } from "../../lib/events/types";
import type { ActivityRegistry } from "../../lib/activity";

/**
 * Base props for activity components
 */
export interface BaseActivityProps {
  /** Additional CSS classes */
  class?: string;
}

/**
 * Props for ActivityHost component
 */
export interface ActivityHostProps extends BaseActivityProps {
  /** Activity to render */
  activity: NormalizedActivity;
  /** Registry to use for looking up renderers */
  registry?: ActivityRegistry;
  /** Whether the activity is currently updating */
  isUpdating?: boolean;
  /** Fallback content when no renderer is found */
  fallback?: unknown;
}

/**
 * Props for ActivitySlot component
 */
export interface ActivitySlotProps extends BaseActivityProps {
  /** The activity type this slot handles */
  type: string;
  /** Whether to show a loading state while activity is pending */
  showLoading?: boolean;
  /** Loading element to display */
  loadingElement?: unknown;
}

/**
 * Props for ActivityRenderer wrapper component
 */
export interface ActivityRendererWrapperProps extends BaseActivityProps {
  /** Activity being rendered */
  activity: NormalizedActivity;
  /** Whether to animate content changes */
  animate?: boolean;
  /** Animation duration in ms */
  animationDuration?: number;
}

/**
 * Activity status for UI display
 */
export type ActivityStatus = "pending" | "updating" | "complete" | "error";

/**
 * Activity metadata for display
 */
export interface ActivityMetadata {
  /** Display title for the activity */
  title?: string;
  /** Description of what the activity does */
  description?: string;
  /** Icon identifier */
  icon?: string;
  /** Whether this activity is expandable */
  expandable?: boolean;
  /** Initial expanded state */
  defaultExpanded?: boolean;
}
