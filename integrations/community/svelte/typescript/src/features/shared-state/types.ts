import type { Readable } from "svelte/store";

/**
 * Configuration for shared state viewer
 */
export interface SharedStateConfig {
  /** Path within state to display (empty for root) */
  path?: string[];
  /** Whether to expand objects by default */
  defaultExpanded?: boolean;
  /** Maximum depth to auto-expand */
  maxExpandDepth?: number;
  /** Whether state updates are allowed */
  readonly?: boolean;
}

/**
 * Props for StateViewer component
 */
export interface StateViewerProps {
  /** The state to display */
  state: Record<string, unknown>;
  /** Configuration options */
  config?: SharedStateConfig;
  /** Additional CSS class names */
  class?: string;
}

/**
 * Shared state store interface
 */
export interface SharedStateStore {
  /** Current state value */
  state: Readable<Record<string, unknown>>;
  /** Get value at a specific path */
  getPath(path: string[]): unknown;
  /** Subscribe to changes at a specific path */
  subscribePath(
    path: string[],
    callback: (value: unknown) => void
  ): () => void;
}

/**
 * Node in state tree for rendering
 */
export interface StateTreeNode {
  key: string;
  value: unknown;
  type: "object" | "array" | "string" | "number" | "boolean" | "null" | "undefined";
  path: string[];
  depth: number;
  expandable: boolean;
}
