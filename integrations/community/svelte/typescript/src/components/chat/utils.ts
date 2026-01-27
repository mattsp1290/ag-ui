import type { NormalizedMessage, NormalizedToolCall } from "../../lib/events/types";
import type { MessageGroup, MessageGroupConfig } from "./types";

/**
 * Default message grouping configuration
 */
export const defaultGroupConfig: Required<MessageGroupConfig> = {
  maxTimeGap: 60000, // 1 minute
  groupByRole: true,
};

/**
 * Group messages by role and time proximity
 */
export function groupMessages(
  messages: NormalizedMessage[],
  config: MessageGroupConfig = {}
): MessageGroup[] {
  const { maxTimeGap, groupByRole } = { ...defaultGroupConfig, ...config };
  const groups: MessageGroup[] = [];

  for (const message of messages) {
    const lastGroup = groups[groups.length - 1];

    // Check if we should start a new group
    const shouldStartNewGroup =
      !lastGroup ||
      (groupByRole && lastGroup.role !== message.role) ||
      (message.timestamp &&
        lastGroup.endTime &&
        message.timestamp - lastGroup.endTime > maxTimeGap);

    if (shouldStartNewGroup) {
      groups.push({
        messages: [message],
        role: message.role,
        startTime: message.timestamp,
        endTime: message.timestamp,
      });
    } else {
      lastGroup.messages.push(message);
      if (message.timestamp) {
        lastGroup.endTime = message.timestamp;
      }
    }
  }

  return groups;
}

/**
 * Check if a message is from the user
 */
export function isUserMessage(message: NormalizedMessage): boolean {
  return message.role === "user";
}

/**
 * Check if a message is from the assistant
 */
export function isAssistantMessage(message: NormalizedMessage): boolean {
  return message.role === "assistant";
}

/**
 * Check if a message is a system message
 */
export function isSystemMessage(message: NormalizedMessage): boolean {
  return message.role === "system";
}

/**
 * Check if a message is a tool response
 */
export function isToolMessage(message: NormalizedMessage): boolean {
  return message.role === "tool";
}

/**
 * Check if a message is an activity
 */
export function isActivityMessage(message: NormalizedMessage): boolean {
  return message.role === "activity";
}

/**
 * Get display name for a message role
 */
export function getRoleDisplayName(role: NormalizedMessage["role"]): string {
  switch (role) {
    case "user":
      return "You";
    case "assistant":
      return "Assistant";
    case "system":
      return "System";
    case "tool":
      return "Tool";
    case "activity":
      return "Activity";
    default:
      return role;
  }
}

/**
 * Get CSS class modifier for message role
 */
export function getRoleClassName(role: NormalizedMessage["role"]): string {
  return `message--${role}`;
}

/**
 * Format message timestamp for display
 */
export function formatMessageTime(timestamp?: number): string {
  if (!timestamp) return "";
  const date = new Date(timestamp);
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * Get tool calls associated with a message
 */
export function getMessageToolCalls(
  message: NormalizedMessage,
  allToolCalls: Map<string, NormalizedToolCall>
): NormalizedToolCall[] {
  if (!message.toolCalls) return [];
  return message.toolCalls.filter((tc) =>
    allToolCalls.has(tc.id)
  );
}

/**
 * Check if message content is empty
 */
export function isMessageEmpty(message: NormalizedMessage): boolean {
  return !message.content || message.content.trim() === "";
}

/**
 * Get preview of message content
 */
export function getMessagePreview(
  message: NormalizedMessage,
  maxLength = 100
): string {
  if (!message.content) return "";
  const trimmed = message.content.trim();
  if (trimmed.length <= maxLength) return trimmed;
  return trimmed.substring(0, maxLength - 3) + "...";
}

/**
 * Filter messages by role
 */
export function filterMessagesByRole(
  messages: NormalizedMessage[],
  role: NormalizedMessage["role"]
): NormalizedMessage[] {
  return messages.filter((m) => m.role === role);
}

/**
 * Get the last message in a list
 */
export function getLastMessage(
  messages: NormalizedMessage[]
): NormalizedMessage | undefined {
  return messages[messages.length - 1];
}

/**
 * Check if we should auto-scroll based on current scroll position
 * Returns true if user is near the bottom
 */
export function shouldAutoScroll(
  scrollContainer: HTMLElement,
  threshold = 100
): boolean {
  const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
  return scrollHeight - scrollTop - clientHeight < threshold;
}
