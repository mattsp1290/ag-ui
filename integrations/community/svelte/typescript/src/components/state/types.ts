/**
 * Base props for state components
 */
export interface BaseStateProps {
  /** Additional CSS classes */
  class?: string;
}

/**
 * Props for StatePanel component
 */
export interface StatePanelProps extends BaseStateProps {
  /** The state object to display */
  state: Record<string, unknown>;
  /** Title for the panel */
  title?: string;
  /** Whether the panel is collapsible */
  collapsible?: boolean;
  /** Initial collapsed state */
  defaultCollapsed?: boolean;
  /** Maximum depth to auto-expand */
  maxAutoExpandDepth?: number;
}

/**
 * Props for StateDiff component
 */
export interface StateDiffProps extends BaseStateProps {
  /** Previous state */
  prev: Record<string, unknown>;
  /** Current state */
  current: Record<string, unknown>;
  /** Whether to show only changes */
  changesOnly?: boolean;
  /** Whether to highlight additions in green */
  highlightAdditions?: boolean;
  /** Whether to highlight removals in red */
  highlightRemovals?: boolean;
}

/**
 * Props for JsonViewer component
 */
export interface JsonViewerProps extends BaseStateProps {
  /** Data to display */
  data: unknown;
  /** Whether the root node is expanded */
  expanded?: boolean;
  /** Maximum depth to render */
  maxDepth?: number;
  /** Whether to enable copy-to-clipboard */
  copyable?: boolean;
  /** Whether to show data types */
  showTypes?: boolean;
  /** Custom key formatter */
  formatKey?: (key: string) => string;
  /** Custom value formatter */
  formatValue?: (value: unknown) => string;
}

/**
 * Diff operation type
 */
export type DiffOperation = "add" | "remove" | "replace" | "unchanged";

/**
 * A single diff entry
 */
export interface DiffEntry {
  /** Path to the changed value */
  path: string[];
  /** Type of change */
  operation: DiffOperation;
  /** Old value (for remove/replace) */
  oldValue?: unknown;
  /** New value (for add/replace) */
  newValue?: unknown;
}

/**
 * JSON viewer node state
 */
export interface ViewerNodeState {
  /** Whether the node is expanded */
  expanded: boolean;
  /** Whether the node has been visited/viewed */
  visited: boolean;
}
