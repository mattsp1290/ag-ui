import type { DiffEntry, DiffOperation } from "./types";

/**
 * Deep compare two values for equality
 */
export function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) return false;

  if (typeof a === "object") {
    if (Array.isArray(a) && Array.isArray(b)) {
      if (a.length !== b.length) return false;
      return a.every((val, idx) => deepEqual(val, b[idx]));
    }

    if (Array.isArray(a) || Array.isArray(b)) return false;

    const aObj = a as Record<string, unknown>;
    const bObj = b as Record<string, unknown>;
    const aKeys = Object.keys(aObj);
    const bKeys = Object.keys(bObj);

    if (aKeys.length !== bKeys.length) return false;
    return aKeys.every((key) => deepEqual(aObj[key], bObj[key]));
  }

  return false;
}

/**
 * Calculate diff between two objects
 */
export function calculateDiff(
  prev: Record<string, unknown>,
  current: Record<string, unknown>,
  path: string[] = []
): DiffEntry[] {
  const diffs: DiffEntry[] = [];

  // Check for removed and changed keys
  for (const key of Object.keys(prev)) {
    const keyPath = [...path, key];
    const prevValue = prev[key];
    const currentValue = current[key];

    if (!(key in current)) {
      // Key was removed
      diffs.push({
        path: keyPath,
        operation: "remove",
        oldValue: prevValue,
      });
    } else if (!deepEqual(prevValue, currentValue)) {
      // Value changed
      if (
        typeof prevValue === "object" &&
        prevValue !== null &&
        typeof currentValue === "object" &&
        currentValue !== null &&
        !Array.isArray(prevValue) &&
        !Array.isArray(currentValue)
      ) {
        // Recurse into nested objects
        diffs.push(
          ...calculateDiff(
            prevValue as Record<string, unknown>,
            currentValue as Record<string, unknown>,
            keyPath
          )
        );
      } else {
        diffs.push({
          path: keyPath,
          operation: "replace",
          oldValue: prevValue,
          newValue: currentValue,
        });
      }
    }
  }

  // Check for added keys
  for (const key of Object.keys(current)) {
    if (!(key in prev)) {
      diffs.push({
        path: [...path, key],
        operation: "add",
        newValue: current[key],
      });
    }
  }

  return diffs;
}

/**
 * Format a path array as a string
 */
export function formatPath(path: string[]): string {
  return path.reduce((acc, part, idx) => {
    if (idx === 0) return part;
    // Use bracket notation for array indices or keys with special chars
    if (/^\d+$/.test(part) || /[.\s[\]]/.test(part)) {
      return `${acc}[${/^\d+$/.test(part) ? part : `"${part}"`}]`;
    }
    return `${acc}.${part}`;
  }, "");
}

/**
 * Get CSS class for diff operation
 */
export function getDiffOperationClass(operation: DiffOperation): string {
  switch (operation) {
    case "add":
      return "diff--added";
    case "remove":
      return "diff--removed";
    case "replace":
      return "diff--changed";
    default:
      return "diff--unchanged";
  }
}

/**
 * Get display symbol for diff operation
 */
export function getDiffOperationSymbol(operation: DiffOperation): string {
  switch (operation) {
    case "add":
      return "+";
    case "remove":
      return "-";
    case "replace":
      return "~";
    default:
      return " ";
  }
}

/**
 * Format a JSON value for display with syntax highlighting class hints
 */
export function getValueDisplayClass(value: unknown): string {
  if (value === null) return "json-null";
  if (value === undefined) return "json-undefined";
  switch (typeof value) {
    case "string":
      return "json-string";
    case "number":
      return "json-number";
    case "boolean":
      return "json-boolean";
    case "object":
      return Array.isArray(value) ? "json-array" : "json-object";
    default:
      return "json-unknown";
  }
}

/**
 * Format a value for compact display
 */
export function formatValueCompact(value: unknown, maxLength = 50): string {
  if (value === null) return "null";
  if (value === undefined) return "undefined";

  switch (typeof value) {
    case "string":
      if (value.length > maxLength) {
        return `"${value.substring(0, maxLength - 3)}..."`;
      }
      return `"${value}"`;
    case "number":
    case "boolean":
      return String(value);
    case "object":
      if (Array.isArray(value)) {
        return `Array(${value.length})`;
      }
      const keys = Object.keys(value);
      return `{${keys.length} keys}`;
    default:
      return String(value);
  }
}

/**
 * Check if a value should be collapsible in the viewer
 */
export function isCollapsibleValue(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (typeof value !== "object") return false;
  if (Array.isArray(value)) return value.length > 0;
  return Object.keys(value).length > 0;
}

/**
 * Get the count of items in a value (for arrays/objects)
 */
export function getItemCount(value: unknown): number {
  if (value === null || value === undefined) return 0;
  if (Array.isArray(value)) return value.length;
  if (typeof value === "object") return Object.keys(value).length;
  return 0;
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Fallback for older browsers
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    const success = document.execCommand("copy");
    document.body.removeChild(textarea);
    return success;
  }
}

/**
 * Serialize a value to JSON for copying
 */
export function serializeForCopy(value: unknown, indent = 2): string {
  try {
    return JSON.stringify(value, null, indent);
  } catch {
    return String(value);
  }
}
