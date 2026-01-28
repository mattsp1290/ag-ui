/**
 * Svelte 5 Agent Store
 *
 * Provides a reactive agent store using Svelte 5's runes ($state, $derived).
 * Properties are directly reactive - no need for the $ prefix in templates.
 *
 * @example
 * ```svelte
 * <script>
 *   import { createAgentStore } from '@ag-ui/svelte';
 *   import { HttpAgent } from '@ag-ui/client';
 *
 *   const agent = new HttpAgent({ url: '/api/agent' });
 *   const store = createAgentStore(agent);
 * </script>
 *
 * <p>Status: {store.status}</p>
 * <p>Running: {store.isRunning}</p>
 * {#each store.messages as message}
 *   <div>{message.content}</div>
 * {/each}
 * ```
 *
 * @module
 */
import { v4 as uuidv4 } from "uuid";
import type {
  AgentStoreConfig,
  StartRunInput,
  RunStatus,
  ReconnectRetryOptions,
} from "./types";
import type {
  NormalizedMessage,
  NormalizedToolCall,
  NormalizedActivity,
  AccumulatedState,
} from "../../lib/events/types";
import type { BaseEvent } from "../../lib/events/normalizer";
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
import { assertValidAgentStoreConfig } from "./validation";
import type {
  Message,
  Tool,
  Context,
  AbstractAgent,
  AgentSubscriber,
  RunStartedEventParams,
  RunFinishedEventParams,
  RunErrorEventParams,
  TextMessageEventParams,
  ToolCallEventParams,
  ToolCallResultEventParams,
  StateEventParams,
  StepEventParams,
  MessagesSnapshotEventParams,
  RunFailedParams,
} from "./subscriber-types";

interface UserMessage {
  id: string;
  role: "user";
  content: string;
}

/**
 * Agent store interface
 *
 * Returns plain reactive state that can be accessed directly in Svelte 5 components.
 */
export interface AgentStore {
  /** The underlying agent instance */
  readonly agent: AbstractAgent;

  /** Reactive array of normalized messages */
  readonly messages: NormalizedMessage[];

  /** Reactive agent state object */
  readonly state: Record<string, unknown>;

  /** Whether a run is currently active */
  readonly isRunning: boolean;

  /** Current run status */
  readonly status: RunStatus;

  /** Current error (if any) */
  readonly error: Error | null;

  /** Active (non-completed) tool calls */
  readonly activeToolCalls: NormalizedToolCall[];

  /** All tool calls */
  readonly toolCalls: NormalizedToolCall[];

  /** Activities */
  readonly activities: NormalizedActivity[];

  /** Current run ID */
  readonly runId: string | null;

  /** Current thread ID */
  readonly threadId: string | null;

  /** Current step name */
  readonly currentStep: string | null;

  /** Start a new agent run */
  start(input: StartRunInput): Promise<void>;

  /** Cancel the current run */
  cancel(): void;

  /** Reconnect to an existing run */
  reconnect(): Promise<void>;

  /** Reconnect with exponential backoff */
  reconnectWithRetry(options?: ReconnectRetryOptions): Promise<void>;

  /** Add a message without starting a run */
  addMessage(message: Message): void;

  /** Clear all messages and reset state */
  reset(): void;

  /** Clear the current error */
  clearError(): void;

  /** Destroy the store and clean up subscriptions */
  destroy(): void;
}

/**
 * High-frequency event types that benefit from batching
 */
const BATCHABLE_EVENTS = new Set([
  "TEXT_MESSAGE_CONTENT",
  "TOOL_CALL_ARGS",
  "STATE_DELTA",
]);

/**
 * Event batcher for high-frequency events
 */
class EventBatcher {
  private queue: BaseEvent[] = [];
  private flushTimer: ReturnType<typeof setTimeout> | null = null;
  private flushCallback: (events: BaseEvent[]) => void;
  private batchIntervalMs: number;
  private maxBatchSize: number;

  constructor(
    onFlush: (events: BaseEvent[]) => void,
    batchIntervalMs = 16,
    maxBatchSize = 100
  ) {
    this.flushCallback = onFlush;
    this.batchIntervalMs = batchIntervalMs;
    this.maxBatchSize = maxBatchSize;
  }

  add(event: BaseEvent): void {
    this.queue.push(event);

    if (this.queue.length >= this.maxBatchSize) {
      this.flush();
      return;
    }

    if (!this.flushTimer) {
      this.flushTimer = setTimeout(() => this.flush(), this.batchIntervalMs);
    }
  }

  flush(): void {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }

    if (this.queue.length > 0) {
      const events = this.queue;
      this.queue = [];
      this.flushCallback(events);
    }
  }

  destroy(): void {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
    this.queue = [];
  }
}

/**
 * Create a reactive agent store for Svelte 5
 *
 * Uses Svelte 5's runes ($state, $derived) for reactivity. The returned
 * object has reactive properties that can be accessed directly without
 * the $ prefix in your templates.
 *
 * @param agent - The agent instance to wrap
 * @param config - Configuration options
 * @returns A reactive agent store
 *
 * @example
 * ```svelte
 * <script>
 *   import { createAgentStore } from '@ag-ui/svelte';
 *   import { HttpAgent } from '@ag-ui/client';
 *
 *   const agent = new HttpAgent({ url: '/api/agent' });
 *   const store = createAgentStore(agent, { debug: true });
 *
 *   async function sendMessage() {
 *     await store.start({ text: 'Hello!' });
 *   }
 * </script>
 *
 * <button onclick={sendMessage} disabled={store.isRunning}>
 *   Send
 * </button>
 *
 * {#if store.error}
 *   <p class="error">{store.error.message}</p>
 * {/if}
 *
 * {#each store.messages as message}
 *   <div class={message.role}>{message.content}</div>
 * {/each}
 * ```
 */
export function createAgentStore(
  agent: AbstractAgent,
  config: AgentStoreConfig = {}
): AgentStore {
  // Validate config at runtime
  assertValidAgentStoreConfig(config);

  const {
    debug = false,
    enableBatching = true,
    batchIntervalMs = 16,
    maxBatchSize = 100,
  } = config;

  // Reactive state using $state
  let accumulatedState = $state<AccumulatedState>(createInitialState());
  let status = $state<RunStatus>("idle");
  let error = $state<Error | null>(null);
  let subscription: { unsubscribe: () => void } | null = null;

  // Initialize with initial values if provided
  if (config.initialMessages) {
    accumulatedState = {
      ...accumulatedState,
      messages: config.initialMessages.map((m) => ({
        id: m.id,
        role: m.role as NormalizedMessage["role"],
        content: m.content || "",
        isStreaming: false,
      })),
    };
  }

  if (config.initialState) {
    accumulatedState = {
      ...accumulatedState,
      agentState: config.initialState,
    };
  }

  // Derived state using $derived
  const messages = $derived(accumulatedState.messages);
  const agentState = $derived(accumulatedState.agentState);
  const isRunning = $derived(status === "running" || status === "starting");
  const activeToolCalls = $derived(getActiveToolCalls(accumulatedState));
  const toolCalls = $derived(getAllToolCalls(accumulatedState));
  const activities = $derived(Array.from(accumulatedState.activities.values()));
  const runId = $derived(accumulatedState.run.runId);
  const threadId = $derived(accumulatedState.run.threadId);
  const currentStep = $derived(accumulatedState.run.currentStep);

  // Event batcher
  let eventBatcher: EventBatcher | null = null;
  if (enableBatching) {
    eventBatcher = new EventBatcher(
      (events) => {
        let newState = accumulatedState;
        for (const event of events) {
          newState = processEvent(newState, event);
        }
        accumulatedState = newState;
      },
      batchIntervalMs,
      maxBatchSize
    );
  }

  function handleEvent(event: BaseEvent): void {
    if (eventBatcher && BATCHABLE_EVENTS.has(event.type)) {
      eventBatcher.add(event);
    } else {
      if (eventBatcher) {
        eventBatcher.flush();
      }
      accumulatedState = processEvent(accumulatedState, event);
    }
  }

  const createSubscriber = (): AgentSubscriber => ({
    onRunStartedEvent: (params: RunStartedEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Run started:", params.event.runId);
      }
      status = "running";
      accumulatedState = {
        ...accumulatedState,
        run: {
          ...accumulatedState.run,
          runId: params.event.runId,
          threadId: params.event.threadId,
          isRunning: true,
        },
      };
    },

    onRunFinishedEvent: () => {
      if (debug) {
        console.debug("[AgentStore] Run finished");
      }
      if (eventBatcher) {
        eventBatcher.flush();
      }
      status = "idle";
      accumulatedState = {
        ...accumulatedState,
        run: {
          ...accumulatedState.run,
          isRunning: false,
        },
      };
    },

    onRunErrorEvent: (params: RunErrorEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Run error:", params.event.message);
      }
      error = new AgentRunError(params.event.message, params.event.code);
      status = "error";
    },

    onTextMessageStartEvent: (params: TextMessageEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Message start:", params.event.messageId);
      }
      handleEvent({
        type: "TEXT_MESSAGE_START",
        messageId: params.event.messageId,
        role: params.event.role,
      });
    },

    onTextMessageContentEvent: (params: TextMessageEventParams) => {
      handleEvent({
        type: "TEXT_MESSAGE_CONTENT",
        messageId: params.event.messageId,
        delta: params.event.delta,
      });
    },

    onTextMessageEndEvent: (params: TextMessageEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Message end:", params.event.messageId);
      }
      handleEvent({
        type: "TEXT_MESSAGE_END",
        messageId: params.event.messageId,
      });
    },

    onToolCallStartEvent: (params: ToolCallEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call start:", params.event.toolCallName);
      }
      handleEvent({
        type: "TOOL_CALL_START",
        toolCallId: params.event.toolCallId,
        toolCallName: params.event.toolCallName,
        parentMessageId: params.event.parentMessageId,
      });
    },

    onToolCallArgsEvent: (params: ToolCallEventParams) => {
      handleEvent({
        type: "TOOL_CALL_ARGS",
        toolCallId: params.event.toolCallId,
        delta: params.event.delta,
      });
    },

    onToolCallEndEvent: (params: ToolCallEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call end:", params.event.toolCallId);
      }
      handleEvent({
        type: "TOOL_CALL_END",
        toolCallId: params.event.toolCallId,
      });
    },

    onToolCallResultEvent: (params: ToolCallResultEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Tool call result:", params.event.toolCallId);
      }
      handleEvent({
        type: "TOOL_CALL_RESULT",
        toolCallId: params.event.toolCallId,
        messageId: params.event.messageId,
        content: params.event.content,
      });
    },

    onStateSnapshotEvent: (params: StateEventParams) => {
      if (debug) {
        console.debug("[AgentStore] State snapshot");
      }
      handleEvent({
        type: "STATE_SNAPSHOT",
        snapshot: params.event.snapshot,
      });
    },

    onStateDeltaEvent: (params: StateEventParams) => {
      handleEvent({
        type: "STATE_DELTA",
        delta: params.event.delta,
      });
    },

    onMessagesSnapshotEvent: (params: MessagesSnapshotEventParams) => {
      if (debug) {
        console.debug(
          "[AgentStore] Messages snapshot:",
          params.event.messages.length
        );
      }
      handleEvent({
        type: "MESSAGES_SNAPSHOT",
        messages: params.event.messages,
      });
    },

    onStepStartedEvent: (params: StepEventParams) => {
      if (debug) {
        console.debug("[AgentStore] Step started:", params.event.stepName);
      }
      accumulatedState = {
        ...accumulatedState,
        run: {
          ...accumulatedState.run,
          currentStep: params.event.stepName,
        },
      };
    },

    onStepFinishedEvent: () => {
      accumulatedState = {
        ...accumulatedState,
        run: {
          ...accumulatedState.run,
          currentStep: null,
        },
      };
    },

    onRunFailed: (params: RunFailedParams) => {
      if (debug) {
        console.debug("[AgentStore] Run failed:", params.error);
      }
      error = params.error;
      status = "error";
    },
  });

  async function start(input: StartRunInput): Promise<void> {
    error = null;
    status = "starting";

    try {
      if (input.text) {
        const userMessage: UserMessage = {
          id: uuidv4(),
          role: "user",
          content: input.text,
        };
        agent.addMessage(userMessage);

        accumulatedState = {
          ...accumulatedState,
          messages: [
            ...accumulatedState.messages,
            {
              id: userMessage.id,
              role: "user" as const,
              content: userMessage.content,
              isStreaming: false,
            },
          ],
        };
      } else if (input.message) {
        agent.addMessage(input.message as Message);
      }

      const subscriber = createSubscriber();
      subscription = agent.subscribe(subscriber);

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
      error = runError;
      status = "error";
      throw runError;
    } finally {
      if (subscription) {
        subscription.unsubscribe();
        subscription = null;
      }
    }
  }

  function cancel(): void {
    if (debug) {
      console.debug("[AgentStore] Cancelling run");
    }

    agent.abortRun();
    agent.detachActiveRun();

    if (subscription) {
      subscription.unsubscribe();
      subscription = null;
    }

    error = new RunCancelledError();
    status = "cancelled";

    accumulatedState = {
      ...accumulatedState,
      run: {
        ...accumulatedState.run,
        isRunning: false,
      },
    };
  }

  async function reconnect(): Promise<void> {
    error = null;
    status = "starting";

    try {
      const subscriber = createSubscriber();
      subscription = agent.subscribe(subscriber);

      await agent.connectAgent();
    } catch (err) {
      const reconnectError =
        err instanceof Error
          ? new ReconnectError(err.message, err)
          : new ReconnectError("Failed to reconnect");
      error = reconnectError;
      status = "error";
      throw reconnectError;
    } finally {
      if (subscription) {
        subscription.unsubscribe();
        subscription = null;
      }
    }
  }

  async function reconnectWithRetry(options: ReconnectRetryOptions = {}): Promise<void> {
    const {
      maxRetries = 3,
      baseDelayMs = 1000,
      maxDelayMs = 30000,
      onRetry,
    } = options;

    let lastError: Error | undefined;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        await reconnect();
        return;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));

        if (attempt < maxRetries) {
          const delay = Math.min(
            baseDelayMs * Math.pow(2, attempt),
            maxDelayMs
          );

          if (debug) {
            console.debug(
              `[AgentStore] Reconnect attempt ${attempt + 1} failed, retrying in ${delay}ms`
            );
          }

          onRetry?.(attempt + 1, delay);

          await new Promise((resolve) => setTimeout(resolve, delay));
        }
      }
    }

    const finalError = new ReconnectError(
      `Failed to reconnect after ${maxRetries + 1} attempts: ${lastError?.message ?? "Unknown error"}`,
      lastError
    );
    error = finalError;
    status = "error";
    throw finalError;
  }

  function addMessage(message: Message): void {
    agent.addMessage(message);
    accumulatedState = {
      ...accumulatedState,
      messages: [
        ...accumulatedState.messages,
        {
          id: message.id,
          role: message.role as NormalizedMessage["role"],
          content: message.content || "",
          isStreaming: false,
        },
      ],
    };
  }

  function reset(): void {
    agent.setMessages([]);
    agent.setState({});
    accumulatedState = createInitialState();
    error = null;
    status = "idle";
  }

  function clearError(): void {
    error = null;
    if (status === "error") {
      status = "idle";
    }
  }

  function destroy(): void {
    if (eventBatcher) {
      eventBatcher.destroy();
    }
    if (subscription) {
      subscription.unsubscribe();
      subscription = null;
    }
    cancel();
  }

  // Return object with getters for reactive access
  return {
    get agent() {
      return agent;
    },
    get messages() {
      return messages;
    },
    get state() {
      return agentState;
    },
    get isRunning() {
      return isRunning;
    },
    get status() {
      return status;
    },
    get error() {
      return error;
    },
    get activeToolCalls() {
      return activeToolCalls;
    },
    get toolCalls() {
      return toolCalls;
    },
    get activities() {
      return activities;
    },
    get runId() {
      return runId;
    },
    get threadId() {
      return threadId;
    },
    get currentStep() {
      return currentStep;
    },
    start,
    cancel,
    reconnect,
    reconnectWithRetry,
    addMessage,
    reset,
    clearError,
    destroy,
  };
}
