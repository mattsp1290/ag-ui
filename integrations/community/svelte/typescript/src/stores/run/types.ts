import type { Readable } from "svelte/store";
import type {
  NormalizedMessage,
  NormalizedToolCall,
  NormalizedActivity,
} from "../../lib/events/types";

/**
 * Message type for the store
 */
export interface Message {
  id: string;
  role: string;
  content?: string;
}

/**
 * Tool definition
 */
export interface Tool {
  name: string;
  description: string;
  parameters?: unknown;
}

/**
 * Context item
 */
export interface Context {
  description: string;
  value: string;
}

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
 * Abstract agent interface (compatible with @ag-ui/client AbstractAgent)
 */
export interface AbstractAgent {
  threadId: string;
  messages: Message[];
  state: unknown;
  subscribe(subscriber: unknown): { unsubscribe: () => void };
  addMessage(message: Message): void;
  setMessages(messages: Message[]): void;
  setState(state: unknown): void;
  runAgent(params?: unknown): Promise<unknown>;
  connectAgent(params?: unknown): Promise<unknown>;
  abortRun(): void;
  detachActiveRun(): void | Promise<void>;
}

/**
 * The reactive agent store interface
 */
export interface AgentStore {
  /** The underlying agent instance */
  agent: AbstractAgent;

  /** Readable store of normalized messages */
  messages: Readable<NormalizedMessage[]>;

  /** Readable store of agent state */
  state: Readable<Record<string, unknown>>;

  /** Readable store indicating if a run is active */
  isRunning: Readable<boolean>;

  /** Readable store of current run status */
  status: Readable<RunStatus>;

  /** Readable store of the current error (if any) */
  error: Readable<Error | null>;

  /** Readable store of active tool calls */
  activeToolCalls: Readable<NormalizedToolCall[]>;

  /** Readable store of all tool calls */
  toolCalls: Readable<NormalizedToolCall[]>;

  /** Readable store of activities */
  activities: Readable<NormalizedActivity[]>;

  /** Readable store of current run ID */
  runId: Readable<string | null>;

  /** Readable store of current thread ID */
  threadId: Readable<string | null>;

  /** Readable store of current step name */
  currentStep: Readable<string | null>;

  /**
   * Start a new agent run
   * @param input - The input for the run
   * @returns Promise that resolves when the run completes
   */
  start(input: StartRunInput): Promise<void>;

  /**
   * Cancel the current run
   */
  cancel(): void;

  /**
   * Reconnect to an existing run (if supported by the agent)
   * @returns Promise that resolves when reconnection completes
   */
  reconnect(): Promise<void>;

  /**
   * Add a message to the store without starting a run
   * @param message - The message to add
   */
  addMessage(message: Message): void;

  /**
   * Clear all messages and reset state
   */
  reset(): void;

  /**
   * Clear the current error
   */
  clearError(): void;

  /**
   * Destroy the store and clean up subscriptions
   */
  destroy(): void;
}
