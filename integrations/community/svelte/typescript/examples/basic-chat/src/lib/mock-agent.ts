/**
 * Mock agent for demonstration purposes
 * Simulates AG-UI events including tool calls and state updates
 */

import { v4 as uuidv4 } from "uuid";

// Subscriber type matching AbstractAgent interface
interface AgentSubscriber {
  onRunStartedEvent?(params: { event: { runId: string; threadId: string } }): void;
  onRunFinishedEvent?(params: { event: { runId?: string; threadId?: string } }): void;
  onRunErrorEvent?(params: { event: { message: string; code?: string } }): void;
  onTextMessageStartEvent?(params: { event: { messageId: string; role?: string } }): void;
  onTextMessageContentEvent?(params: { event: { messageId: string; delta?: string } }): void;
  onTextMessageEndEvent?(params: { event: { messageId: string } }): void;
  onToolCallStartEvent?(params: {
    event: { toolCallId: string; toolCallName?: string; parentMessageId?: string };
  }): void;
  onToolCallArgsEvent?(params: { event: { toolCallId: string; delta?: string } }): void;
  onToolCallEndEvent?(params: { event: { toolCallId: string } }): void;
  onToolCallResultEvent?(params: {
    event: { toolCallId: string; messageId: string; content: string };
  }): void;
  onStateSnapshotEvent?(params: { event: { snapshot?: Record<string, unknown> } }): void;
  onStateDeltaEvent?(params: { event: { delta?: unknown[] } }): void;
  onStepStartedEvent?(params: { event: { stepName: string } }): void;
  onStepFinishedEvent?(params: { event: { stepName: string } }): void;
  onRunFailed?(params: { error: Error }): void;
}

interface Message {
  id: string;
  role: string;
  content?: string;
}

export interface MockAgentConfig {
  /** Simulate typing delay between characters (ms) */
  typingDelay?: number;
  /** Simulate tool calls */
  simulateToolCalls?: boolean;
  /** Simulate state updates */
  simulateStateUpdates?: boolean;
  /** Predefined responses for specific messages */
  responses?: Map<string, string>;
  /** Simulate errors randomly (0-1 probability) */
  errorProbability?: number;
}

const DEFAULT_RESPONSES: [string, string][] = [
  ["hello", "Hello! I'm a demo agent powered by AG-UI. How can I help you today?"],
  ["help", "I can demonstrate various AG-UI features:\n\n- **Chat**: Send messages and see streaming responses\n- **Tool Calls**: Type 'calculate' to see a tool call demo\n- **State**: Type 'state' to see shared state updates\n- **Error**: Type 'error' to see error handling"],
  ["calculate", "Let me perform a calculation for you..."],
  ["state", "Let me update the shared state..."],
  ["error", "Simulating an error condition..."],
];

export class MockAgent {
  threadId: string;
  messages: Message[] = [];
  state: Record<string, unknown> = {};

  private config: Required<MockAgentConfig>;
  private subscribers: Set<AgentSubscriber> = new Set();
  private abortController: AbortController | null = null;
  private responses: Map<string, string>;

  constructor(config: MockAgentConfig = {}) {
    this.threadId = uuidv4();
    this.config = {
      typingDelay: config.typingDelay ?? 20,
      simulateToolCalls: config.simulateToolCalls ?? true,
      simulateStateUpdates: config.simulateStateUpdates ?? true,
      responses: config.responses ?? new Map(DEFAULT_RESPONSES),
      errorProbability: config.errorProbability ?? 0,
    };
    this.responses = this.config.responses;
  }

  subscribe(subscriber: AgentSubscriber): { unsubscribe: () => void } {
    this.subscribers.add(subscriber);
    return {
      unsubscribe: () => {
        this.subscribers.delete(subscriber);
      },
    };
  }

  addMessage(message: Message): void {
    this.messages.push(message);
  }

  setMessages(messages: Message[]): void {
    this.messages = [...messages];
  }

  setState(state: unknown): void {
    this.state = state as Record<string, unknown>;
  }

  abortRun(): void {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
  }

  detachActiveRun(): void {
    this.abortController = null;
  }

  async runAgent(): Promise<void> {
    this.abortController = new AbortController();
    const signal = this.abortController.signal;

    const runId = uuidv4();

    // Notify run started
    this.emit("onRunStartedEvent", { event: { runId, threadId: this.threadId } });

    try {
      // Check for error simulation
      if (Math.random() < this.config.errorProbability) {
        await this.delay(500, signal);
        this.emit("onRunErrorEvent", { event: { message: "Simulated random error", code: "RANDOM_ERROR" } });
        return;
      }

      // Get last user message
      const lastMessage = this.messages[this.messages.length - 1];
      const userText = lastMessage?.content?.toLowerCase() || "";

      // Check for special commands
      if (userText.includes("error")) {
        await this.simulateError(signal);
        return;
      }

      if (userText.includes("calculate") && this.config.simulateToolCalls) {
        await this.simulateToolCall(signal, runId);
      }

      if (userText.includes("state") && this.config.simulateStateUpdates) {
        await this.simulateStateUpdate(signal);
      }

      // Generate response
      const response = this.getResponse(userText);
      await this.streamResponse(response, signal);

      // Notify run finished
      this.emit("onRunFinishedEvent", { event: { runId, threadId: this.threadId } });
    } catch (err) {
      if ((err as Error).name === "AbortError") {
        // Run was cancelled, don't emit error
        return;
      }
      this.emit("onRunFailed", { error: err as Error });
    } finally {
      this.abortController = null;
    }
  }

  async connectAgent(): Promise<void> {
    // For mock agent, connecting just starts a new run
    return this.runAgent();
  }

  private emit<K extends keyof AgentSubscriber>(
    event: K,
    params: Parameters<NonNullable<AgentSubscriber[K]>>[0]
  ): void {
    this.subscribers.forEach((subscriber) => {
      const handler = subscriber[event];
      if (handler) {
        (handler as (params: unknown) => void)(params);
      }
    });
  }

  private getResponse(userText: string): string {
    // Check for matching response
    for (const [trigger, response] of this.responses.entries()) {
      if (userText.includes(trigger.toLowerCase())) {
        return response;
      }
    }

    // Default response
    return `I received your message: "${userText}"\n\nThis is a demo response from the mock agent. Try typing 'help' to see what I can do!`;
  }

  private async delay(ms: number, signal: AbortSignal): Promise<void> {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(resolve, ms);
      signal.addEventListener("abort", () => {
        clearTimeout(timeout);
        reject(new DOMException("Aborted", "AbortError"));
      });
    });
  }

  private async streamResponse(text: string, signal: AbortSignal): Promise<void> {
    const messageId = uuidv4();

    // Start message
    this.emit("onTextMessageStartEvent", {
      event: { messageId, role: "assistant" },
    });

    // Stream content character by character
    let buffer = "";
    for (const char of text) {
      if (signal.aborted) return;

      buffer += char;
      this.emit("onTextMessageContentEvent", {
        event: { messageId, delta: char },
      });

      await this.delay(this.config.typingDelay, signal);
    }

    // End message
    this.emit("onTextMessageEndEvent", { event: { messageId } });

    // Add to internal messages
    this.messages.push({
      id: messageId,
      role: "assistant",
      content: buffer,
    });
  }

  private async simulateError(signal: AbortSignal): Promise<void> {
    await this.delay(500, signal);
    this.emit("onRunErrorEvent", {
      event: { message: "This is a simulated error for demonstration", code: "DEMO_ERROR" },
    });
  }

  private async simulateToolCall(signal: AbortSignal, _runId: string): Promise<void> {
    const toolCallId = uuidv4();
    const messageId = uuidv4();

    // Step: Analyzing
    this.emit("onStepStartedEvent", { event: { stepName: "Analyzing request" } });
    await this.delay(500, signal);
    this.emit("onStepFinishedEvent", { event: { stepName: "Analyzing request" } });

    // Tool call start
    this.emit("onToolCallStartEvent", {
      event: {
        toolCallId,
        toolCallName: "calculator",
        parentMessageId: messageId,
      },
    });

    // Stream tool args
    const args = '{"operation": "multiply", "a": 7, "b": 6}';
    for (const char of args) {
      if (signal.aborted) return;
      this.emit("onToolCallArgsEvent", {
        event: { toolCallId, delta: char },
      });
      await this.delay(10, signal);
    }

    // Tool call end
    this.emit("onToolCallEndEvent", { event: { toolCallId } });

    // Simulate processing
    await this.delay(800, signal);

    // Tool result
    this.emit("onToolCallResultEvent", {
      event: {
        toolCallId,
        messageId: uuidv4(),
        content: JSON.stringify({ result: 42 }),
      },
    });
  }

  private async simulateStateUpdate(signal: AbortSignal): Promise<void> {
    // Send state snapshot
    this.emit("onStateSnapshotEvent", {
      event: {
        snapshot: {
          counter: 0,
          lastUpdated: new Date().toISOString(),
          items: [],
        },
      },
    });

    await this.delay(300, signal);

    // Send state deltas (JSON Patch format)
    this.emit("onStateDeltaEvent", {
      event: {
        delta: [
          { op: "replace", path: "/counter", value: 1 },
          { op: "add", path: "/items/-", value: "First item" },
        ],
      },
    });

    await this.delay(300, signal);

    this.emit("onStateDeltaEvent", {
      event: {
        delta: [
          { op: "replace", path: "/counter", value: 2 },
          { op: "add", path: "/items/-", value: "Second item" },
          { op: "replace", path: "/lastUpdated", value: new Date().toISOString() },
        ],
      },
    });
  }
}
