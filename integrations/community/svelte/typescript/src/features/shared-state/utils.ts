import type { StateTreeNode } from "./types";

/**
 * Get the type of a value for display
 */
export function getValueType(
  value: unknown
): StateTreeNode["type"] {
  if (value === null) return "null";
  if (value === undefined) return "undefined";
  if (Array.isArray(value)) return "array";
  const type = typeof value;
  if (type === "object") return "object";
  if (type === "string") return "string";
  if (type === "number") return "number";
  if (type === "boolean") return "boolean";
  return "string"; // fallback
}

/**
 * Check if a value is expandable (object or array with entries)
 */
export function isExpandable(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "object") return Object.keys(value).length > 0;
  return false;
}

/**
 * Get value at a path in an object
 */
export function getValueAtPath(
  obj: Record<string, unknown>,
  path: string[]
): unknown {
  let current: unknown = obj;
  for (const key of path) {
    if (current === null || current === undefined) return undefined;
    if (typeof current !== "object") return undefined;
    current = (current as Record<string, unknown>)[key];
  }
  return current;
}

/**
 * Create a state tree node
 */
export function createTreeNode(
  key: string,
  value: unknown,
  path: string[],
  depth: number
): StateTreeNode {
  return {
    key,
    value,
    type: getValueType(value),
    path,
    depth,
    expandable: isExpandable(value),
  };
}

/**
 * Flatten state into tree nodes for rendering
 */
export function flattenState(
  state: Record<string, unknown>,
  expandedPaths: Set<string>,
  maxDepth = Infinity
): StateTreeNode[] {
  const nodes: StateTreeNode[] = [];

  function traverse(
    value: unknown,
    key: string,
    path: string[],
    depth: number
  ): void {
    const node = createTreeNode(key, value, path, depth);
    nodes.push(node);

    if (!node.expandable || depth >= maxDepth) return;

    const pathKey = path.join(".");
    if (!expandedPaths.has(pathKey)) return;

    if (Array.isArray(value)) {
      value.forEach((item, index) => {
        traverse(item, String(index), [...path, String(index)], depth + 1);
      });
    } else if (typeof value === "object" && value !== null) {
      Object.entries(value).forEach(([k, v]) => {
        traverse(v, k, [...path, k], depth + 1);
      });
    }
  }

  Object.entries(state).forEach(([key, value]) => {
    traverse(value, key, [key], 0);
  });

  return nodes;
}

/**
 * Format a value for display
 */
export function formatValue(value: unknown): string {
  if (value === null) return "null";
  if (value === undefined) return "undefined";
  if (typeof value === "string") return `"${value}"`;
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    return `Array(${value.length})`;
  }
  if (typeof value === "object") {
    return `Object(${Object.keys(value).length})`;
  }
  return String(value);
}

/**
 * Get a preview of an object/array value
 */
export function getValuePreview(value: unknown, maxLength = 50): string {
  if (!isExpandable(value)) return formatValue(value);

  try {
    const json = JSON.stringify(value);
    if (json.length <= maxLength) return json;
    return json.substring(0, maxLength - 3) + "...";
  } catch {
    return formatValue(value);
  }
}
