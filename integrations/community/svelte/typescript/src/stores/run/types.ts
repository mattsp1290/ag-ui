import type {
  Message,
  Tool,
  Context,
  AbstractAgent,
} from "./subscriber-types";

// Re-export core types from subscriber-types
export type { Message, Tool, Context, AbstractAgent };

/**
 * Input for starting an agent run
 */
export interface StartRunInput {
  /** User message text */
  text?: string;
  /** Full user message (alternative to text) */
  message?: Message;
  /** Tools available to the agent */
  tools?: Tool[];
  /** Context for the agent */
  context?: Context[];
  /** Custom run ID (auto-generated if not provided) */
  runId?: string;
  /** Additional properties to forward to the agent */
  forwardedProps?: Record<string, unknown>;
}

/**
 * Configuration for creating an agent store
 */
export interface AgentStoreConfig {
  /** Initial messages to populate the store */
  initialMessages?: Message[];
  /** Initial agent state */
  initialState?: Record<string, unknown>;
  /** Enable debug logging */
  debug?: boolean;
  /**
   * Enable event batching for high-frequency events (TEXT_MESSAGE_CONTENT, TOOL_CALL_ARGS).
   * This reduces re-renders by batching rapid events into fewer store updates.
   * Default: true
   */
  enableBatching?: boolean;
  /**
   * Batch interval in milliseconds. Events are flushed at this interval.
   * Lower values = more responsive, higher values = fewer re-renders.
   * Default: 16 (~60fps)
   */
  batchIntervalMs?: number;
  /**
   * Maximum events to batch before forcing a flush.
   * Default: 100
   */
  maxBatchSize?: number;
}

/**
 * Run status
 */
export type RunStatus = "idle" | "starting" | "running" | "error" | "cancelled";

/**
 * Options for reconnect with retry
 */
export interface ReconnectRetryOptions {
  /** Maximum number of retry attempts (default: 3) */
  maxRetries?: number;
  /** Initial delay in milliseconds (default: 1000) */
  baseDelayMs?: number;
  /** Maximum delay cap in milliseconds (default: 30000) */
  maxDelayMs?: number;
  /** Callback invoked before each retry with attempt number and delay */
  onRetry?: (attempt: number, delayMs: number) => void;
}
