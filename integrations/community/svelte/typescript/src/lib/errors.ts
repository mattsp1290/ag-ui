/**
 * Error types for the AG-UI Svelte integration
 */

/**
 * Base error class for AG-UI Svelte errors
 */
export class AgentStoreError extends Error {
  constructor(
    message: string,
    public readonly code: string,
    public readonly cause?: Error
  ) {
    super(message);
    this.name = "AgentStoreError";
  }
}

/**
 * Error thrown when a run fails to start
 */
export class RunStartError extends AgentStoreError {
  constructor(message: string, cause?: Error) {
    super(message, "RUN_START_ERROR", cause);
    this.name = "RunStartError";
  }
}

/**
 * Error thrown when a run is cancelled
 */
export class RunCancelledError extends AgentStoreError {
  constructor(message: string = "Run was cancelled") {
    super(message, "RUN_CANCELLED");
    this.name = "RunCancelledError";
  }
}

/**
 * Error thrown when a connection fails
 */
export class ConnectionError extends AgentStoreError {
  constructor(message: string, cause?: Error) {
    super(message, "CONNECTION_ERROR", cause);
    this.name = "ConnectionError";
  }
}

/**
 * Error thrown when reconnection fails
 */
export class ReconnectError extends AgentStoreError {
  constructor(message: string, cause?: Error) {
    super(message, "RECONNECT_ERROR", cause);
    this.name = "ReconnectError";
  }
}

/**
 * Error from the agent run (RUN_ERROR event)
 */
export class AgentRunError extends AgentStoreError {
  constructor(
    message: string,
    public readonly agentCode?: string
  ) {
    super(message, agentCode ?? "AGENT_ERROR");
    this.name = "AgentRunError";
  }
}

/**
 * Type guard to check if an error is an AgentStoreError
 */
export function isAgentStoreError(error: unknown): error is AgentStoreError {
  return error instanceof AgentStoreError;
}

/**
 * Type guard to check if a run was cancelled
 */
export function isRunCancelled(error: unknown): error is RunCancelledError {
  return error instanceof RunCancelledError;
}
