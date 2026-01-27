export * from "./types";

// UI utility functions for Svelte components

/**
 * Create CSS classes from an object of class name to boolean conditions
 */
export function classNames(
  classes: Record<string, boolean | undefined | null>
): string {
  return Object.entries(classes)
    .filter(([, condition]) => condition)
    .map(([className]) => className)
    .join(" ");
}

/**
 * Format an error for display
 */
export function formatError(error: Error | null): string {
  if (!error) return "";
  return error.message || "An unknown error occurred";
}

/**
 * Get error code if available
 */
export function getErrorCode(error: Error | null): string | undefined {
  if (!error) return undefined;
  if ("code" in error && typeof error.code === "string") {
    return error.code;
  }
  return undefined;
}
