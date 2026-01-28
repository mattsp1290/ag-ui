import type { NormalizedActivity } from "../events/types";

/**
 * Props passed to activity renderer components
 */
export interface ActivityRendererProps<T = unknown> {
  /** The activity being rendered */
  activity: NormalizedActivity;
  /** Parsed activity content */
  content: T;
  /** Whether the activity is currently being updated */
  isUpdating?: boolean;
}

/**
 * Activity renderer component type (Svelte 5 compatible)
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type ActivityRenderer<T = unknown> = new (...args: any[]) => any;

/**
 * Activity renderer registration
 */
export interface ActivityRendererRegistration<T = unknown> {
  /** The activity type this renderer handles */
  type: string;
  /** The Svelte component to render the activity */
  component: ActivityRenderer<T>;
  /** Optional validator for activity content */
  validate?: (content: unknown) => content is T;
  /** Optional priority (higher = checked first, default: 0) */
  priority?: number;
}

/**
 * Options for the activity registry
 */
export interface ActivityRegistryOptions {
  /** Fallback component when no renderer matches */
  fallbackComponent?: ActivityRenderer;
  /** Whether to log warnings when no renderer is found */
  warnOnMissing?: boolean;
}

/**
 * Result of looking up a renderer
 */
export interface RendererLookupResult<T = unknown> {
  /** The renderer component (or fallback if not found) */
  component: ActivityRenderer<T>;
  /** Whether this is a fallback renderer */
  isFallback: boolean;
  /** The registration that matched (undefined for fallback) */
  registration?: ActivityRendererRegistration<T>;
}
