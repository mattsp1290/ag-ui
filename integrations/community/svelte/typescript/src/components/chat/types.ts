import type { NormalizedMessage, NormalizedToolCall } from "../../lib/events/types";

/**
 * Base props for all chat components
 */
export interface BaseChatProps {
  /** Additional CSS classes */
  class?: string;
}

/**
 * Props for ChatRoot container component
 */
export interface ChatRootProps extends BaseChatProps {
  /** Whether to auto-scroll to bottom on new messages */
  autoScroll?: boolean;
  /** Maximum height for the chat container */
  maxHeight?: string;
}

/**
 * Props for MessageList component
 */
export interface MessageListProps extends BaseChatProps {
  /** Messages to display */
  messages: NormalizedMessage[];
  /** Whether the agent is currently generating */
  isStreaming?: boolean;
  /** Custom renderer for messages */
  renderMessage?: (message: NormalizedMessage) => unknown;
  /** Empty state element */
  emptyState?: unknown;
}

/**
 * Props for MessageItem component
 */
export interface MessageItemProps extends BaseChatProps {
  /** The message to display */
  message: NormalizedMessage;
  /** Whether this message is currently streaming */
  isStreaming?: boolean;
  /** Whether to show avatar */
  showAvatar?: boolean;
  /** Whether to show timestamp */
  showTimestamp?: boolean;
  /** Associated tool calls */
  toolCalls?: NormalizedToolCall[];
}

/**
 * Props for Composer component
 */
export interface ComposerProps extends BaseChatProps {
  /** Placeholder text */
  placeholder?: string;
  /** Whether the input is disabled */
  disabled?: boolean;
  /** Whether to auto-focus the input */
  autoFocus?: boolean;
  /** Maximum input length */
  maxLength?: number;
  /** Whether to allow multi-line input */
  multiline?: boolean;
  /** Number of rows for multiline input */
  rows?: number;
  /** Callback when message is submitted */
  onSubmit?: (text: string) => void;
  /** Callback when input changes */
  onChange?: (text: string) => void;
}

/**
 * Message grouping configuration
 */
export interface MessageGroupConfig {
  /** Maximum time gap (ms) to group messages */
  maxTimeGap?: number;
  /** Whether to group consecutive messages from same role */
  groupByRole?: boolean;
}

/**
 * A group of related messages
 */
export interface MessageGroup {
  /** Messages in this group */
  messages: NormalizedMessage[];
  /** The role of messages in this group */
  role: NormalizedMessage["role"];
  /** Timestamp of first message */
  startTime?: number;
  /** Timestamp of last message */
  endTime?: number;
}
