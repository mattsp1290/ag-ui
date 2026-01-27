import { writable, derived, get, type Readable } from "svelte/store";
import { v4 as uuidv4 } from "uuid";
import type {
  AgentStore,
  AgentStoreConfig,
  StartRunInput,
  RunStatus,
} from "./types";
import type {
  NormalizedMessage,
  NormalizedToolCall,
  NormalizedActivity,
  AccumulatedState,
} from "../../lib/events/types";
import {
  createInitialState,
  processEvent,
  getActiveToolCalls,
  getAllToolCalls,
} from "../../lib/events/normalizer";
import {
  AgentRunError,
  RunCancelledError,
  RunStartError,
  ReconnectError,
} from "../../lib/errors";

// Define the types inline to avoid import issues with @ag-ui/client
interface Message {
  id: string;
  role: string;
  content?: string;
}

interface UserMessage {
  id: string;
  role: "user";
  content: string;
}

interface Tool {
  name: string;
  description: string;
  parameters?: unknown;
}

interface Context {
  description: string;
  value: string;
}

interface RunAgentParameters {
  tools?: Tool[];
  context?: Context[];
  runId?: string;
  forwardedProps?: Record<string, unknown>;
}

interface AgentSubscriberParams {
  messages: Message[];
  state: unknown;
  agent: AbstractAgent;
  input?: { runId?: string; threadId?: string };
}

interface RunStartedEventParams extends AgentSubscriberParams {
  event: { runId: string; threadId: string };
}

interface RunFinishedEventParams extends AgentSubscriberParams {
  event: { runId?: string; threadId?: string };
  result?: unknown;
}

interface RunErrorEventParams extends AgentSubscriberParams {
  event: { message: string; code?: string };
}

interface TextMessageEventParams extends AgentSubscriberParams {
  event: { messageId: string; role?: string; delta?: string };
  textMessageBuffer?: string;
}

interface ToolCallEventParams extends AgentSubscriberParams {
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

interface ToolCallResultEventParams extends AgentSubscriberParams {
  event: { toolCallId: string; messageId: string; content: string };
}

interface StateEventParams extends AgentSubscriberParams {
  event: { snapshot?: Record<string, unknown>; delta?: unknown[] };
}

interface StepEventParams extends AgentSubscriberParams {
  event: { stepName: string };
}

interface MessagesSnapshotEventParams extends AgentSubscriberParams {
  event: { messages: Message[] };
}

interface RunFailedParams extends AgentSubscriberParams {
  error: Error;
}

interface AgentSubscriber {
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

interface AbstractAgent {
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
 * Create a reactive Svelte store for managing agent interactions
 */
export function createAgentStore(
  agent: AbstractAgent,
  config: AgentStoreConfig = {}
): AgentStore {
  const { debug = false } = config;

  // Internal state
  const accumulatedState = writable<AccumulatedState>(createInitialState());
  const status = writable<RunStatus>("idle");
  const error = writable<Error | null>(null);
  const subscription = writable<{ unsubscribe: () => void } | null>(null);

  // Initialize with initial messages if provided
  if (config.initialMessages) {
    accumulatedState.update((state) => ({
      ...state,
      messages: config.initialMessages!.map((m) => ({
        id: m.id,
        role: m.role as NormalizedMessage["role"],
        content: m.content || "",
        isStreaming: false,
      })),
    }));
  }

  if (config.initialState) {
    accumulatedState.update((state) => ({
      ...state,
      agentState: config.initialState!,
    }));
  }

  // Derived stores
  const messages: Readable<NormalizedMessage[]> = derived(
    accumulatedState,
    ($state) => $state.messages
  );

  const agentState: Readable<Record<string, unknown>> = derived(
    accumulatedState,
    ($state) => $state.agentState
  );

  const isRunning: Readable<boolean> = derived(
    status,
    ($status) => $status === "running" || $status === "starting"
  );

  const activeToolCalls: Readable<NormalizedToolCall[]> = derived(
    accumulatedState,
    ($state) => getActiveToolCalls($state)
  );

  const toolCalls: Readable<NormalizedToolCall[]> = derived(
    accumulatedState,
    ($state) => getAllToolCalls($state)
  );

  const activities: Readable<NormalizedActivity[]> = derived(
    accumulatedState,
    ($state) => Array.from($state.activities.values())
  );

  const runId: Readable<string | null> = derived(
    accumulatedState,
    ($state) => $state.run.runId
  );

  const threadId: Readable<string | null> = derived(
    accumulatedState,
    ($state) => $state.run.threadId
  );

  const currentStep: Readable<string | null> = derived(
    accumulatedState,
    ($state) => $state.run.currentStep
  );

  // Create the subscriber that processes events
  const createSubscriber = (): AgentSubscriber => ({
    onRunStartedEvent: (params: RunStartedEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Run started:", params.event.runId);
      }
      status.set("running");
      accumulatedState.update((state) => ({
        ...state,
        run: {
          ...state.run,
          runId: params.event.runId,
          threadId: params.event.threadId,
          isRunning: true,
        },
      }));
    },

    onRunFinishedEvent: () => {
      if (debug) {
        console.debug("[AgentStore] Run finished");
      }
      status.set("idle");
      accumulatedState.update((state) => ({
        ...state,
        run: {
          ...state.run,
          isRunning: false,
        },
      }));
    },

    onRunErrorEvent: (params: RunErrorEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Run error:", params.event.message);
      }
      const agentError = new AgentRunError(
        params.event.message,
        params.event.code
      );
      error.set(agentError);
      status.set("error");
    },

    onTextMessageStartEvent: (params: TextMessageEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Message start:", params.event.messageId);
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TEXT_MESSAGE_START" as const,
          messageId: params.event.messageId,
          role: params.event.role,
        } as any)
      );
    },

    onTextMessageContentEvent: (params: TextMessageEventParams) => {
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TEXT_MESSAGE_CONTENT" as const,
          messageId: params.event.messageId,
          delta: params.event.delta,
        } as any)
      );
    },

    onTextMessageEndEvent: (params: TextMessageEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Message end:", params.event.messageId);
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TEXT_MESSAGE_END" as const,
          messageId: params.event.messageId,
        } as any)
      );
    },

    onToolCallStartEvent: (params: ToolCallEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call start:", params.event.toolCallName);
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TOOL_CALL_START" as const,
          toolCallId: params.event.toolCallId,
          toolCallName: params.event.toolCallName,
          parentMessageId: params.event.parentMessageId,
        } as any)
      );
    },

    onToolCallArgsEvent: (params: ToolCallEventParams) => {
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TOOL_CALL_ARGS" as const,
          toolCallId: params.event.toolCallId,
          delta: params.event.delta,
        } as any)
      );
    },

    onToolCallEndEvent: (params: ToolCallEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call end:", params.event.toolCallId);
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TOOL_CALL_END" as const,
          toolCallId: params.event.toolCallId,
        } as any)
      );
    },

    onToolCallResultEvent: (params: ToolCallResultEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call result:", params.event.toolCallId);
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "TOOL_CALL_RESULT" as const,
          toolCallId: params.event.toolCallId,
          messageId: params.event.messageId,
          content: params.event.content,
        } as any)
      );
    },

    onStateSnapshotEvent: (params: StateEventParams) => {
      if (debug) {
        console.debug("[AgentStore] State snapshot");
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "STATE_SNAPSHOT" as const,
          snapshot: params.event.snapshot,
        } as any)
      );
    },

    onStateDeltaEvent: (params: StateEventParams) => {
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "STATE_DELTA" as const,
          delta: params.event.delta,
        } as any)
      );
    },

    onMessagesSnapshotEvent: (params: MessagesSnapshotEventParams) => {
      if (debug) {
        console.debug(
          "[AgentStore] Messages snapshot:",
          params.event.messages.length
        );
      }
      accumulatedState.update((state) =>
        processEvent(state, {
          type: "MESSAGES_SNAPSHOT" as const,
          messages: params.event.messages,
        } as any)
      );
    },

    onStepStartedEvent: (params: StepEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Step started:", params.event.stepName);
      }
      accumulatedState.update((state) => ({
        ...state,
        run: {
          ...state.run,
          currentStep: params.event.stepName,
        },
      }));
    },

    onStepFinishedEvent: () => {
      accumulatedState.update((state) => ({
        ...state,
        run: {
          ...state.run,
          currentStep: null,
        },
      }));
    },

    onRunFailed: (params: RunFailedParams) => {
      if (debug) {
        console.debug("[AgentStore] Run failed:", params.error);
      }
      error.set(params.error);
      status.set("error");
    },
  });

  /**
   * Start a new agent run
   */
  async function start(input: StartRunInput): Promise<void> {
    // Clear previous error
    error.set(null);
    status.set("starting");

    try {
      // Create user message if text is provided
      if (input.text) {
        const userMessage: UserMessage = {
          id: uuidv4(),
          role: "user",
          content: input.text,
        };
        agent.addMessage(userMessage);

        // Also add to accumulated state
        accumulatedState.update((state) => ({
          ...state,
          messages: [
            ...state.messages,
            {
              id: userMessage.id,
              role: "user" as const,
              content: userMessage.content,
              isStreaming: false,
            },
          ],
        }));
      } else if (input.message) {
        agent.addMessage(input.message as Message);
      }

      const subscriber = createSubscriber();
      const sub = agent.subscribe(subscriber);
      subscription.set(sub);

      await agent.runAgent({
        tools: input.tools as Tool[],
        context: input.context as Context[],
        runId: input.runId,
        forwardedProps: input.forwardedProps,
      });
    } catch (err) {
      const runError =
        err instanceof Error
          ? new RunStartError(err.message, err)
          : new RunStartError("Failed to start run");
      error.set(runError);
      status.set("error");
      throw runError;
    } finally {
      const sub = get(subscription);
      if (sub) {
        sub.unsubscribe();
        subscription.set(null);
      }
    }
  }

  /**
   * Cancel the current run
   */
  function cancel(): void {
    if (debug) {
      console.debug("[AgentStore] Cancelling run");
    }

    agent.abortRun();
    agent.detachActiveRun();

    const sub = get(subscription);
    if (sub) {
      sub.unsubscribe();
      subscription.set(null);
    }

    error.set(new RunCancelledError());
    status.set("cancelled");

    accumulatedState.update((state) => ({
      ...state,
      run: {
        ...state.run,
        isRunning: false,
      },
    }));
  }

  /**
   * Reconnect to an existing run
   */
  async function reconnect(): Promise<void> {
    error.set(null);
    status.set("starting");

    try {
      const subscriber = createSubscriber();
      const sub = agent.subscribe(subscriber);
      subscription.set(sub);

      await agent.connectAgent();
    } catch (err) {
      const reconnectError =
        err instanceof Error
          ? new ReconnectError(err.message, err)
          : new ReconnectError("Failed to reconnect");
      error.set(reconnectError);
      status.set("error");
      throw reconnectError;
    } finally {
      const sub = get(subscription);
      if (sub) {
        sub.unsubscribe();
        subscription.set(null);
      }
    }
  }

  /**
   * Add a message without starting a run
   */
  function addMessage(message: Message): void {
    agent.addMessage(message);
    accumulatedState.update((state) => ({
      ...state,
      messages: [
        ...state.messages,
        {
          id: message.id,
          role: message.role as NormalizedMessage["role"],
          content: message.content || "",
          isStreaming: false,
        },
      ],
    }));
  }

  /**
   * Reset the store
   */
  function reset(): void {
    agent.setMessages([]);
    agent.setState({});
    accumulatedState.set(createInitialState());
    error.set(null);
    status.set("idle");
  }

  /**
   * Clear the current error
   */
  function clearError(): void {
    error.set(null);
    if (get(status) === "error") {
      status.set("idle");
    }
  }

  /**
   * Destroy the store
   */
  function destroy(): void {
    const sub = get(subscription);
    if (sub) {
      sub.unsubscribe();
      subscription.set(null);
    }
    cancel();
  }

  return {
    agent: agent as any,
    messages,
    state: agentState,
    isRunning,
    status,
    error,
    activeToolCalls,
    toolCalls,
    activities,
    runId,
    threadId,
    currentStep,
    start,
    cancel,
    reconnect,
    addMessage: addMessage as any,
    reset,
    clearError,
    destroy,
  };
}
