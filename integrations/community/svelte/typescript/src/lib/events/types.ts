/**
 * Base event interface (compatible with @ag-ui/core BaseEvent)
 */
export interface BaseEvent {
  type: string;
  timestamp?: number;
  [key: string]: unknown;
}

/**
 * Message interface (compatible with @ag-ui/core Message)
 */
export interface Message {
  id: string;
  role: string;
  content?: string;
}

/**
 * Normalized message model for UI display
 */
export interface NormalizedMessage {
  id: string;
  role: "user" | "assistant" | "system" | "developer" | "tool" | "activity";
  content: string;
  isStreaming: boolean;
  toolCalls?: NormalizedToolCall[];
  timestamp?: number;
}

/**
 * Normalized tool call model
 */
export interface NormalizedToolCall {
  id: string;
  name: string;
  arguments: string;
  parsedArguments?: Record<string, unknown>;
  result?: string;
  error?: string;
  status: "pending" | "streaming" | "completed" | "error";
  parentMessageId?: string;
}

/**
 * Normalized activity model
 */
export interface NormalizedActivity {
  id: string;
  type: string;
  content: Record<string, unknown>;
  messageId: string;
  timestamp?: number;
}

/**
 * Run state tracking
 */
export interface RunState {
  runId: string | null;
  threadId: string | null;
  isRunning: boolean;
  currentStep: string | null;
}

/**
 * Accumulated state from events
 */
export interface AccumulatedState {
  messages: NormalizedMessage[];
  toolCalls: Map<string, NormalizedToolCall>;
  activities: Map<string, NormalizedActivity>;
  agentState: Record<string, unknown>;
  run: RunState;
}

/**
 * Event handler context
 */
export interface EventContext {
  state: AccumulatedState;
  event: BaseEvent;
}
