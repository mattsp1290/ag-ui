import type { NormalizedMessage } from "../../lib/events/types";
import type { AgentStore } from "../../stores/run/agent-store.svelte";
import type { HITLStore } from "../hitl/types";

/**
 * Configuration for the AgenticChat template
 */
export interface AgenticChatConfig {
  /** Placeholder text for the composer */
  placeholder?: string;
  /** Whether to show timestamps on messages */
  showTimestamps?: boolean;
  /** Whether to show avatars */
  showAvatars?: boolean;
  /** Whether to auto-scroll to new messages */
  autoScroll?: boolean;
  /** Whether to show tool calls inline */
  showToolCalls?: boolean;
  /** Whether to enable HITL (requires hitlStore) */
  enableHITL?: boolean;
  /** Whether to show typing indicator when streaming */
  showTypingIndicator?: boolean;
  /** Custom welcome message */
  welcomeMessage?: string;
  /** Whether to allow multi-line input */
  multilineInput?: boolean;
  /** Maximum message length */
  maxMessageLength?: number;
  /** Custom header content */
  headerContent?: unknown;
  /** Custom footer content */
  footerContent?: unknown;
}

/**
 * Props for AgenticChat container component
 */
export interface AgenticChatProps {
  /** The agent store */
  store: AgentStore;
  /** Optional HITL store for approval flows */
  hitlStore?: HITLStore;
  /** Configuration options */
  config?: AgenticChatConfig;
  /** Additional CSS classes */
  class?: string;
}

/**
 * State for the AgenticChat template
 */
export interface AgenticChatState {
  /** Current input text */
  inputText: string;
  /** Whether the input is focused */
  inputFocused: boolean;
  /** Whether we're at the bottom of the scroll */
  isAtBottom: boolean;
  /** Scroll container element reference */
  scrollContainer?: HTMLElement;
}

/**
 * Event handlers for chat interactions
 */
export interface ChatEventHandlers {
  /** Called when a message is submitted */
  onSubmit?: (text: string) => void;
  /** Called when input text changes */
  onInputChange?: (text: string) => void;
  /** Called when a message is clicked */
  onMessageClick?: (message: NormalizedMessage) => void;
  /** Called when cancel is requested */
  onCancel?: () => void;
  /** Called when retry is requested */
  onRetry?: () => void;
}

/**
 * Typing indicator configuration
 */
export interface TypingIndicatorConfig {
  /** Text to show (e.g., "AI is typing...") */
  text?: string;
  /** Dot animation style */
  animationStyle?: "bounce" | "pulse" | "fade";
  /** Animation speed in ms */
  animationSpeed?: number;
}

/**
 * Default agentic chat configuration
 */
export const defaultAgenticChatConfig: Required<AgenticChatConfig> = {
  placeholder: "Type a message...",
  showTimestamps: false,
  showAvatars: true,
  autoScroll: true,
  showToolCalls: true,
  enableHITL: false,
  showTypingIndicator: true,
  welcomeMessage: "",
  multilineInput: true,
  maxMessageLength: 4000,
  headerContent: null,
  footerContent: null,
};
