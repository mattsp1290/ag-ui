/**
 * Runtime validation for agent store configuration
 *
 * Uses Zod to validate configuration at runtime, providing helpful
 * error messages when invalid configuration is detected.
 */
import { z } from "zod";

/**
 * Schema for validating Message objects
 */
export const MessageSchema = z.object({
  id: z.string(),
  role: z.string(),
  content: z.string().optional(),
});

/**
 * Schema for validating AgentStoreConfig
 *
 * Validates all configuration options with sensible constraints:
 * - batchIntervalMs: 0-1000ms (0 = immediate, max 1 second)
 * - maxBatchSize: 1-1000 events per batch
 */
export const AgentStoreConfigSchema = z
  .object({
    debug: z.boolean().optional(),
    enableBatching: z.boolean().optional(),
    batchIntervalMs: z
      .number()
      .min(0, "batchIntervalMs must be non-negative")
      .max(1000, "batchIntervalMs must not exceed 1000ms")
      .optional(),
    maxBatchSize: z
      .number()
      .int("maxBatchSize must be an integer")
      .min(1, "maxBatchSize must be at least 1")
      .max(1000, "maxBatchSize must not exceed 1000")
      .optional(),
    initialMessages: z.array(MessageSchema).optional(),
    initialState: z.record(z.unknown()).optional(),
  })
  .strict();

/**
 * Validation result type
 */
export interface ValidationResult {
  success: boolean;
  error?: string;
  issues?: z.ZodIssue[];
}

/**
 * Validate AgentStoreConfig at runtime
 *
 * @param config - The configuration object to validate
 * @returns Validation result with success status and any errors
 *
 * @example
 * ```ts
 * const result = validateAgentStoreConfig({ batchIntervalMs: -1 });
 * if (!result.success) {
 *   console.error(result.error);
 *   // "batchIntervalMs must be non-negative"
 * }
 * ```
 */
export function validateAgentStoreConfig(config: unknown): ValidationResult {
  const result = AgentStoreConfigSchema.safeParse(config);

  if (result.success) {
    return { success: true };
  }

  const issues = result.error.issues;
  const errorMessages = issues.map((issue) => {
    const path = issue.path.join(".");
    return path ? `${path}: ${issue.message}` : issue.message;
  });

  return {
    success: false,
    error: errorMessages.join("; "),
    issues,
  };
}

/**
 * Validate and throw on invalid config
 *
 * @param config - The configuration object to validate
 * @throws Error if validation fails
 *
 * @example
 * ```ts
 * // Throws: "Invalid AgentStoreConfig: batchIntervalMs must be non-negative"
 * assertValidAgentStoreConfig({ batchIntervalMs: -1 });
 * ```
 */
export function assertValidAgentStoreConfig(config: unknown): void {
  const result = validateAgentStoreConfig(config);
  if (!result.success) {
    throw new Error(`Invalid AgentStoreConfig: ${result.error}`);
  }
}
