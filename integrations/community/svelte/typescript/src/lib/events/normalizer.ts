import type {
  AccumulatedState,
  NormalizedMessage,
  NormalizedToolCall,
  NormalizedActivity,
} from "./types";

/**
 * Event types enum (mirrors @ag-ui/core EventType)
 */
export const EventType = {
  TEXT_MESSAGE_START: "TEXT_MESSAGE_START",
  TEXT_MESSAGE_CONTENT: "TEXT_MESSAGE_CONTENT",
  TEXT_MESSAGE_END: "TEXT_MESSAGE_END",
  TEXT_MESSAGE_CHUNK: "TEXT_MESSAGE_CHUNK",
  TOOL_CALL_START: "TOOL_CALL_START",
  TOOL_CALL_ARGS: "TOOL_CALL_ARGS",
  TOOL_CALL_END: "TOOL_CALL_END",
  TOOL_CALL_CHUNK: "TOOL_CALL_CHUNK",
  TOOL_CALL_RESULT: "TOOL_CALL_RESULT",
  STATE_SNAPSHOT: "STATE_SNAPSHOT",
  STATE_DELTA: "STATE_DELTA",
  MESSAGES_SNAPSHOT: "MESSAGES_SNAPSHOT",
  ACTIVITY_SNAPSHOT: "ACTIVITY_SNAPSHOT",
  ACTIVITY_DELTA: "ACTIVITY_DELTA",
  RUN_STARTED: "RUN_STARTED",
  RUN_FINISHED: "RUN_FINISHED",
  RUN_ERROR: "RUN_ERROR",
  STEP_STARTED: "STEP_STARTED",
  STEP_FINISHED: "STEP_FINISHED",
} as const;

export type EventTypeValue = (typeof EventType)[keyof typeof EventType];

/**
 * Base event interface
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
 * Create initial accumulated state
 */
export function createInitialState(): AccumulatedState {
  return {
    messages: [],
    toolCalls: new Map(),
    activities: new Map(),
    agentState: {},
    run: {
      runId: null,
      threadId: null,
      isRunning: false,
      currentStep: null,
    },
  };
}

/**
 * Find or create a message by ID
 */
function findOrCreateMessage(
  state: AccumulatedState,
  messageId: string,
  role: NormalizedMessage["role"] = "assistant"
): NormalizedMessage {
  let message = state.messages.find((m) => m.id === messageId);
  if (!message) {
    message = {
      id: messageId,
      role,
      content: "",
      isStreaming: true,
    };
    state.messages.push(message);
  }
  return message;
}

/**
 * Update message in state
 */
function updateMessage(
  state: AccumulatedState,
  messageId: string,
  updates: Partial<NormalizedMessage>
): void {
  const idx = state.messages.findIndex((m) => m.id === messageId);
  const existingMessage = state.messages[idx];
  if (idx !== -1 && existingMessage) {
    state.messages[idx] = { ...existingMessage, ...updates };
  }
}

/**
 * Helper to safely get a string property from event
 */
function getString(event: BaseEvent, key: string): string {
  const value = event[key];
  return typeof value === "string" ? value : "";
}

/**
 * Helper to safely get an optional string property from event
 */
function getOptionalString(event: BaseEvent, key: string): string | undefined {
  const value = event[key];
  return typeof value === "string" ? value : undefined;
}

/**
 * Helper to safely get an array from event
 */
function getArray<T>(event: BaseEvent, key: string): T[] {
  const value = event[key];
  return Array.isArray(value) ? value : [];
}

/**
 * Helper to safely get an object from event
 */
function getObject(event: BaseEvent, key: string): Record<string, unknown> {
  const value = event[key];
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

/**
 * Process a single event and update accumulated state
 * Returns the updated state (mutates in place for efficiency)
 */
export function processEvent(
  state: AccumulatedState,
  event: BaseEvent
): AccumulatedState {
  switch (event.type) {
    case EventType.RUN_STARTED: {
      state.run = {
        runId: getString(event, "runId"),
        threadId: getString(event, "threadId"),
        isRunning: true,
        currentStep: null,
      };
      break;
    }

    case EventType.RUN_FINISHED: {
      state.run.isRunning = false;
      // Mark all streaming messages as complete
      state.messages.forEach((msg) => {
        if (msg.isStreaming) {
          msg.isStreaming = false;
        }
      });
      // Mark all pending tool calls as complete
      state.toolCalls.forEach((tc) => {
        if (tc.status === "streaming" || tc.status === "pending") {
          tc.status = "completed";
        }
      });
      break;
    }

    case EventType.RUN_ERROR: {
      state.run.isRunning = false;
      break;
    }

    case EventType.STEP_STARTED: {
      state.run.currentStep = getString(event, "stepName");
      break;
    }

    case EventType.STEP_FINISHED: {
      state.run.currentStep = null;
      break;
    }

    case EventType.TEXT_MESSAGE_START: {
      const messageId = getString(event, "messageId");
      const role = (getOptionalString(event, "role") as NormalizedMessage["role"]) || "assistant";
      findOrCreateMessage(state, messageId, role);
      break;
    }

    case EventType.TEXT_MESSAGE_CONTENT: {
      const messageId = getString(event, "messageId");
      const delta = getString(event, "delta");
      const message = findOrCreateMessage(state, messageId);
      message.content += delta;
      message.isStreaming = true;
      break;
    }

    case EventType.TEXT_MESSAGE_END: {
      const messageId = getString(event, "messageId");
      updateMessage(state, messageId, { isStreaming: false });
      break;
    }

    case EventType.TEXT_MESSAGE_CHUNK: {
      const messageId = getOptionalString(event, "messageId");
      const delta = getOptionalString(event, "delta");
      if (messageId && delta) {
        const role = (getOptionalString(event, "role") as NormalizedMessage["role"]) || "assistant";
        const message = findOrCreateMessage(state, messageId, role);
        message.content += delta;
        message.isStreaming = true;
      }
      break;
    }

    case EventType.TOOL_CALL_START: {
      const toolCallId = getString(event, "toolCallId");
      const toolCallName = getString(event, "toolCallName");
      const parentMessageId = getOptionalString(event, "parentMessageId");

      const toolCall: NormalizedToolCall = {
        id: toolCallId,
        name: toolCallName,
        arguments: "",
        status: "pending",
        parentMessageId,
      };
      state.toolCalls.set(toolCallId, toolCall);

      // Link to parent message if specified
      if (parentMessageId) {
        const parentMsg = state.messages.find((m) => m.id === parentMessageId);
        if (parentMsg) {
          parentMsg.toolCalls = parentMsg.toolCalls || [];
          parentMsg.toolCalls.push(toolCall);
        }
      }
      break;
    }

    case EventType.TOOL_CALL_ARGS: {
      const toolCallId = getString(event, "toolCallId");
      const delta = getString(event, "delta");
      const toolCall = state.toolCalls.get(toolCallId);
      if (toolCall) {
        toolCall.arguments += delta;
        toolCall.status = "streaming";
      }
      break;
    }

    case EventType.TOOL_CALL_END: {
      const toolCallId = getString(event, "toolCallId");
      const toolCall = state.toolCalls.get(toolCallId);
      if (toolCall) {
        toolCall.status = "completed";
        // Try to parse arguments
        try {
          toolCall.parsedArguments = JSON.parse(toolCall.arguments);
        } catch {
          // Arguments may not be valid JSON
        }
      }
      break;
    }

    case EventType.TOOL_CALL_RESULT: {
      const toolCallId = getString(event, "toolCallId");
      const messageId = getString(event, "messageId");
      const content = getString(event, "content");

      const toolCall = state.toolCalls.get(toolCallId);
      if (toolCall) {
        toolCall.result = content;
        toolCall.status = "completed";
      }
      // Also create a tool message
      const toolMsg: NormalizedMessage = {
        id: messageId,
        role: "tool",
        content,
        isStreaming: false,
      };
      state.messages.push(toolMsg);
      break;
    }

    case EventType.TOOL_CALL_CHUNK: {
      const toolCallId = getOptionalString(event, "toolCallId");
      const toolCallName = getOptionalString(event, "toolCallName");
      const delta = getOptionalString(event, "delta");
      const parentMessageId = getOptionalString(event, "parentMessageId");

      if (toolCallId) {
        let toolCall = state.toolCalls.get(toolCallId);
        if (!toolCall && toolCallName) {
          toolCall = {
            id: toolCallId,
            name: toolCallName,
            arguments: "",
            status: "pending",
            parentMessageId,
          };
          state.toolCalls.set(toolCallId, toolCall);
        }
        if (toolCall && delta) {
          toolCall.arguments += delta;
          toolCall.status = "streaming";
        }
      }
      break;
    }

    case EventType.STATE_SNAPSHOT: {
      state.agentState = getObject(event, "snapshot");
      break;
    }

    case EventType.STATE_DELTA: {
      const delta = getArray<unknown>(event, "delta");
      state.agentState = applyJsonPatch(state.agentState, delta);
      break;
    }

    case EventType.MESSAGES_SNAPSHOT: {
      const messages = getArray<Message>(event, "messages");
      state.messages = messages.map((m) => ({
        id: m.id,
        role: m.role as NormalizedMessage["role"],
        content: m.content || "",
        isStreaming: false,
      }));
      break;
    }

    case EventType.ACTIVITY_SNAPSHOT: {
      const messageId = getString(event, "messageId");
      const activityType = getString(event, "activityType");
      const content = getObject(event, "content");

      const activity: NormalizedActivity = {
        id: messageId,
        type: activityType,
        content,
        messageId,
        timestamp: event.timestamp,
      };
      state.activities.set(messageId, activity);

      // Also add as activity message
      const activityMsg: NormalizedMessage = {
        id: messageId,
        role: "activity",
        content: JSON.stringify(content),
        isStreaming: false,
        timestamp: event.timestamp,
      };
      state.messages.push(activityMsg);
      break;
    }

    case EventType.ACTIVITY_DELTA: {
      const messageId = getString(event, "messageId");
      const patch = getArray<unknown>(event, "patch");
      const activity = state.activities.get(messageId);
      if (activity) {
        activity.content = applyJsonPatch(activity.content, patch);
      }
      break;
    }
  }

  // Return a new object reference to ensure Svelte reactivity
  return { ...state };
}

/**
 * Forbidden keys that could enable prototype pollution attacks
 */
const FORBIDDEN_KEYS = new Set(["__proto__", "constructor", "prototype"]);

/**
 * Simple JSON Patch (RFC 6902) implementation for state updates.
 *
 * @remarks
 * This is a minimal implementation with the following limitations:
 * - **Supported operations:** `add`, `replace`, `remove` only
 * - **Unsupported operations:** `move`, `copy`, `test` are not implemented
 * - **No array index support:** Paths like `/items/0` will treat `0` as an object key,
 *   not an array index. Use STATE_SNAPSHOT for array modifications.
 * - **No nested array support:** Arrays within objects cannot be patched
 * - **Path format:** Uses `/` separator (e.g., `/user/name`), leading `/` is optional
 *
 * For complex state updates involving arrays, use STATE_SNAPSHOT instead of STATE_DELTA.
 *
 * @param target - The object to apply patches to
 * @param patches - Array of JSON Patch operations
 * @returns A new object with patches applied (does not mutate original)
 */
function applyJsonPatch<T extends Record<string, unknown>>(
  target: T,
  patches: unknown[]
): T {
  const result = { ...target };

  for (const patch of patches) {
    const op = patch as { op: string; path: string; value?: unknown };
    const pathParts = op.path.split("/").filter(Boolean);

    if (pathParts.length === 0) continue;

    // SECURITY: Block prototype pollution attacks by throwing an error
    // This makes attacks visible and prevents silent failures
    if (pathParts.some((part) => FORBIDDEN_KEYS.has(part))) {
      throw new Error(
        `[AG-UI Svelte] Blocked prototype pollution attempt: ${op.path}`
      );
    }

    let current: Record<string, unknown> = result;
    for (let i = 0; i < pathParts.length - 1; i++) {
      const key = pathParts[i];
      if (key === undefined) continue;
      if (!(key in current)) {
        current[key] = {};
      }
      current = current[key] as Record<string, unknown>;
    }

    const lastKey = pathParts[pathParts.length - 1];
    if (lastKey === undefined) continue;

    switch (op.op) {
      case "add":
      case "replace":
        current[lastKey] = op.value;
        break;
      case "remove":
        delete current[lastKey];
        break;
    }
  }

  return result;
}

/**
 * Convert accumulated state to message array (for compatibility)
 */
export function getMessages(state: AccumulatedState): NormalizedMessage[] {
  return [...state.messages];
}

/**
 * Get active (non-completed) tool calls
 */
export function getActiveToolCalls(
  state: AccumulatedState
): NormalizedToolCall[] {
  return Array.from(state.toolCalls.values()).filter(
    (tc) => tc.status === "pending" || tc.status === "streaming"
  );
}

/**
 * Get all tool calls
 */
export function getAllToolCalls(
  state: AccumulatedState
): NormalizedToolCall[] {
  return Array.from(state.toolCalls.values());
}
