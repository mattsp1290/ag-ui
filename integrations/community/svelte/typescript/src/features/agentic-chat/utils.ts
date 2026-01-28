import type { NormalizedMessage } from "../../lib/events/types";
import type { AgenticChatConfig, AgenticChatState } from "./types";
import { defaultAgenticChatConfig } from "./types";

/**
 * Create initial AgenticChat state
 */
export function createAgenticChatState(): AgenticChatState {
  return {
    inputText: "",
    inputFocused: false,
    isAtBottom: true,
  };
}

/**
 * Merge config with defaults
 */
export function mergeAgenticChatConfig(
  config?: Partial<AgenticChatConfig>
): Required<AgenticChatConfig> {
  return { ...defaultAgenticChatConfig, ...config };
}

/**
 * Get display-ready messages (filter out empty/system messages if needed)
 */
export function getDisplayMessages(
  messages: NormalizedMessage[],
  config: AgenticChatConfig
): NormalizedMessage[] {
  return messages.filter((msg) => {
    // Always show user and assistant messages with content
    if ((msg.role === "user" || msg.role === "assistant") && msg.content) {
      return true;
    }

    // Show tool messages if showToolCalls is enabled
    if (msg.role === "tool" && config.showToolCalls) {
      return true;
    }

    // Show activity messages
    if (msg.role === "activity") {
      return true;
    }

    return false;
  });
}

/**
 * Get the current streaming message (if any)
 */
export function getStreamingMessage(
  messages: NormalizedMessage[]
): NormalizedMessage | undefined {
  return messages.find((msg) => msg.isStreaming);
}

/**
 * Check if we should show typing indicator
 */
export function shouldShowTypingIndicator(
  messages: NormalizedMessage[],
  isRunning: boolean
): boolean {
  // Show if running but no streaming message yet
  if (!isRunning) return false;
  const streaming = getStreamingMessage(messages);
  return !streaming;
}

/**
 * Validate input text
 */
export function validateInput(
  text: string,
  maxLength: number
): { valid: boolean; error?: string } {
  const trimmed = text.trim();

  if (trimmed.length === 0) {
    return { valid: false, error: "Message cannot be empty" };
  }

  if (trimmed.length > maxLength) {
    return {
      valid: false,
      error: `Message too long (${trimmed.length}/${maxLength})`,
    };
  }

  return { valid: true };
}

/**
 * Get character count display
 */
export function getCharacterCountDisplay(
  text: string,
  maxLength: number
): { count: string; warning: boolean } {
  const length = text.length;
  const threshold = maxLength * 0.8;

  return {
    count: `${length}/${maxLength}`,
    warning: length > threshold,
  };
}

/**
 * Handle keyboard events for the composer
 */
export function handleComposerKeyDown(
  event: KeyboardEvent,
  config: { multiline: boolean; onSubmit: () => void }
): boolean {
  // Enter without shift submits (unless multiline mode allows newlines)
  if (event.key === "Enter" && !event.shiftKey) {
    if (!config.multiline) {
      event.preventDefault();
      config.onSubmit();
      return true;
    }
  }

  // Ctrl/Cmd + Enter always submits
  if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
    event.preventDefault();
    config.onSubmit();
    return true;
  }

  return false;
}

/**
 * Auto-resize textarea based on content
 */
export function autoResizeTextarea(
  textarea: HTMLTextAreaElement,
  maxHeight = 200
): void {
  // Reset height to auto to get the correct scrollHeight
  textarea.style.height = "auto";

  // Set to scrollHeight but cap at maxHeight
  const newHeight = Math.min(textarea.scrollHeight, maxHeight);
  textarea.style.height = `${newHeight}px`;
}

/**
 * Scroll to bottom of container
 */
export function scrollToBottom(container: HTMLElement, smooth = true): void {
  container.scrollTo({
    top: container.scrollHeight,
    behavior: smooth ? "smooth" : "auto",
  });
}

/**
 * Check if scrolled to bottom
 */
export function isScrolledToBottom(container: HTMLElement, threshold = 50): boolean {
  const { scrollTop, scrollHeight, clientHeight } = container;
  return scrollHeight - scrollTop - clientHeight < threshold;
}

/**
 * Format time for message display
 */
export function formatMessageTimestamp(timestamp?: number): string {
  if (!timestamp) return "";

  const date = new Date(timestamp);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();

  if (isToday) {
    return date.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * Get avatar initials from role
 */
export function getRoleInitials(role: NormalizedMessage["role"]): string {
  switch (role) {
    case "user":
      return "U";
    case "assistant":
      return "AI";
    case "system":
      return "S";
    case "tool":
      return "T";
    case "activity":
      return "A";
    default:
      return "?";
  }
}
