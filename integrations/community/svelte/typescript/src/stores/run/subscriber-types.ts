/**
 * Subscriber types for AG-UI agent store
 *
 * These types define the interface for communicating with @ag-ui/client AbstractAgent
 * without creating a hard dependency on the client package's internal implementation.
 */

/**
 * Basic message interface compatible with @ag-ui/core Message
 */
export interface Message {
  id: string;
  role: string;
  content?: string;
}

/**
 * Tool definition compatible with @ag-ui/core Tool
 */
export interface Tool {
  name: string;
  description: string;
  parameters?: unknown;
}

/**
 * Context item compatible with @ag-ui/core Context
 */
export interface Context {
  description: string;
  value: string;
}

/**
 * Parameters for running an agent
 */
export interface RunAgentParameters {
  tools?: Tool[];
  context?: Context[];
  runId?: string;
  forwardedProps?: Record<string, unknown>;
}

/**
 * Abstract agent interface compatible with @ag-ui/client AbstractAgent
 *
 * This interface allows the Svelte integration to work with any agent
 * that implements these methods, providing flexibility for custom agents.
 */
export interface AbstractAgent {
  threadId: string;
  messages: Message[];
  state: unknown;
  subscribe(subscriber: AgentSubscriber): { unsubscribe: () => void };
  addMessage(message: Message): void;
  setMessages(messages: Message[]): void;
  setState(state: unknown): void;
  runAgent(params?: RunAgentParameters): Promise<unknown>;
  connectAgent(params?: RunAgentParameters): Promise<unknown>;
  abortRun(): void;
  detachActiveRun(): void | Promise<void>;
}

/**
 * Base parameters passed to all subscriber callbacks
 */
export interface AgentSubscriberParams {
  messages: Message[];
  state: unknown;
  agent: AbstractAgent;
  input?: { runId?: string; threadId?: string };
}

/**
 * Parameters for run started event
 */
export interface RunStartedEventParams extends AgentSubscriberParams {
  event: { runId: string; threadId: string };
}

/**
 * Parameters for run finished event
 */
export interface RunFinishedEventParams extends AgentSubscriberParams {
  event: { runId?: string; threadId?: string };
  result?: unknown;
}

/**
 * Parameters for run error event
 */
export interface RunErrorEventParams extends AgentSubscriberParams {
  event: { message: string; code?: string };
}

/**
 * Parameters for text message events (start, content, end)
 */
export interface TextMessageEventParams extends AgentSubscriberParams {
  event: { messageId: string; role?: string; delta?: string };
  textMessageBuffer?: string;
}

/**
 * Parameters for tool call events (start, args, end)
 */
export interface ToolCallEventParams extends AgentSubscriberParams {
  event: {
    toolCallId: string;
    toolCallName?: string;
    parentMessageId?: string;
    delta?: string;
  };
  toolCallBuffer?: string;
  toolCallName?: string;
  toolCallArgs?: Record<string, unknown>;
  partialToolCallArgs?: Record<string, unknown>;
}

/**
 * Parameters for tool call result event
 */
export interface ToolCallResultEventParams extends AgentSubscriberParams {
  event: { toolCallId: string; messageId: string; content: string };
}

/**
 * Parameters for state events (snapshot, delta)
 */
export interface StateEventParams extends AgentSubscriberParams {
  event: { snapshot?: Record<string, unknown>; delta?: unknown[] };
}

/**
 * Parameters for step events (started, finished)
 */
export interface StepEventParams extends AgentSubscriberParams {
  event: { stepName: string };
}

/**
 * Parameters for messages snapshot event
 */
export interface MessagesSnapshotEventParams extends AgentSubscriberParams {
  event: { messages: Message[] };
}

/**
 * Parameters for run failed event
 */
export interface RunFailedParams extends AgentSubscriberParams {
  error: Error;
}

/**
 * Subscriber interface for receiving agent events
 *
 * Implement this interface to receive callbacks for various agent events
 * during a run. All methods are optional.
 */
export interface AgentSubscriber {
  onRunStartedEvent?(params: RunStartedEventParams): void;
  onRunFinishedEvent?(params: RunFinishedEventParams): void;
  onRunErrorEvent?(params: RunErrorEventParams): void;
  onTextMessageStartEvent?(params: TextMessageEventParams): void;
  onTextMessageContentEvent?(params: TextMessageEventParams): void;
  onTextMessageEndEvent?(params: TextMessageEventParams): void;
  onToolCallStartEvent?(params: ToolCallEventParams): void;
  onToolCallArgsEvent?(params: ToolCallEventParams): void;
  onToolCallEndEvent?(params: ToolCallEventParams): void;
  onToolCallResultEvent?(params: ToolCallResultEventParams): void;
  onStateSnapshotEvent?(params: StateEventParams): void;
  onStateDeltaEvent?(params: StateEventParams): void;
  onMessagesSnapshotEvent?(params: MessagesSnapshotEventParams): void;
  onStepStartedEvent?(params: StepEventParams): void;
  onStepFinishedEvent?(params: StepEventParams): void;
  onRunFailed?(params: RunFailedParams): void;
}
