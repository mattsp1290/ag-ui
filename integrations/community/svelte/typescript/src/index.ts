/**
 * @ag-ui/svelte - Svelte integration for AG-UI protocol
 *
 * Provides reactive stores and utilities for building agent-powered UIs with Svelte.
 */

// Core agent store
export { createAgentStore } from "./stores/run/agent-store";
export type {
  AgentStore,
  AgentStoreConfig,
  StartRunInput,
  RunStatus,
  AbstractAgent,
  Message,
  Tool,
  Context,
} from "./stores/run/types";

// Event normalization
export {
  createInitialState,
  processEvent,
  getMessages,
  getActiveToolCalls,
  getAllToolCalls,
  EventType,
} from "./lib/events/normalizer";
export type {
  NormalizedMessage,
  NormalizedToolCall,
  NormalizedActivity,
  AccumulatedState,
  RunState,
} from "./lib/events/types";
export type { BaseEvent, EventTypeValue } from "./lib/events/normalizer";

// Error types
export {
  AgentStoreError,
  RunStartError,
  RunCancelledError,
  ConnectionError,
  ReconnectError,
  AgentRunError,
  isAgentStoreError,
  isRunCancelled,
} from "./lib/errors";

// UI utilities
export { classNames, formatError, getErrorCode } from "./components/ui";
export type {
  BaseComponentProps,
  ErrorBannerProps,
  LoadingProps,
  EmptyStateProps,
} from "./components/ui/types";

// Tool components utilities
export {
  getToolStatus,
  formatToolArguments,
  isToolCallActive,
  getToolArgumentsSummary,
  parseToolResult,
  formatToolResult,
} from "./components/tools/utils";
export type {
  BaseToolProps,
  ToolCallCardProps,
  ToolCallListProps,
  ToolResultProps,
  ToolStatusConfig,
} from "./components/tools/types";
export { defaultToolStatusConfig } from "./components/tools/types";

// Shared state feature
export {
  getValueType,
  isExpandable,
  getValueAtPath,
  createTreeNode,
  flattenState,
  formatValue,
  getValuePreview,
} from "./features/shared-state/utils";
export type {
  SharedStateConfig,
  StateViewerProps,
  SharedStateStore,
  StateTreeNode,
} from "./features/shared-state/types";

// Human-in-the-loop feature
export { createHITLStore, withHITL } from "./features/hitl/store";
export type {
  ApprovalDecision,
  ApprovalResult,
  ApprovalCardProps,
  ApprovalQueueProps,
  HITLConfig,
  HITLStore,
} from "./features/hitl/types";

// Context provider and hooks
export {
  provideAgentContext,
  useAgentContext,
  useAgentStore,
  hasAgentContext,
  AGENT_CONTEXT_KEY,
} from "./context";
export type { AgentContextConfig, AgentContextValue } from "./context";

// Content sanitization
export {
  defaultSanitizeConfig,
  escapeHtml,
  stripHtml,
  isSafeUrl,
  sanitizeUrl,
  sanitizeContent,
  sanitizeMessageContent,
  sanitizeToolOutput,
  containsUnsafeHtml,
} from "./lib/sanitize";
export type {
  SanitizeConfig,
  SanitizeResult,
  UrlSanitizeOptions,
} from "./lib/sanitize";

// Activity renderer registry
export {
  ActivityRegistry,
  createActivityRegistry,
  defaultActivityRegistry,
  registerActivityRenderer,
  getActivityRenderer,
} from "./lib/activity";
export type {
  ActivityRenderer,
  ActivityRendererProps,
  ActivityRendererRegistration,
  ActivityRegistryOptions,
  RendererLookupResult,
} from "./lib/activity";

// Chat component utilities
export {
  defaultGroupConfig,
  groupMessages,
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isToolMessage,
  isActivityMessage,
  getRoleDisplayName,
  getRoleClassName,
  formatMessageTime,
  getMessageToolCalls,
  isMessageEmpty,
  getMessagePreview,
  filterMessagesByRole,
  getLastMessage,
  shouldAutoScroll,
} from "./components/chat";
export type {
  BaseChatProps,
  ChatRootProps,
  MessageListProps,
  MessageItemProps,
  ComposerProps,
  MessageGroupConfig,
  MessageGroup,
} from "./components/chat";

// Activity component utilities
export {
  getActivityStatus,
  getActivityStatusClass,
  extractActivityMetadata,
  formatActivityType,
  getActivityTitle,
  hasActivityContentChanged,
  getActivityPreview,
  hasVisualContent,
  getActivityIcon,
} from "./components/activity";
export type {
  BaseActivityProps,
  ActivityHostProps,
  ActivitySlotProps,
  ActivityRendererWrapperProps,
  ActivityStatus,
  ActivityMetadata,
} from "./components/activity";

// State viewer component utilities
export {
  deepEqual,
  calculateDiff,
  formatPath,
  getDiffOperationClass,
  getDiffOperationSymbol,
  getValueDisplayClass,
  formatValueCompact,
  isCollapsibleValue,
  getItemCount,
  copyToClipboard,
  serializeForCopy,
} from "./components/state";
export type {
  BaseStateProps,
  StatePanelProps,
  StateDiffProps,
  JsonViewerProps,
  DiffOperation,
  DiffEntry,
  ViewerNodeState,
} from "./components/state";

// HITL component utilities
export {
  getStatusConfig,
  getStatusClass,
  isStatusActive,
  getStatusText,
  isHighRiskToolCall,
  getToolCallRiskLevel,
  getRiskLevelClass,
  formatToolCallForApproval,
  getActionShortcut,
  createRejectionReason,
  sortPendingByPriority,
  defaultStatusConfigs,
} from "./components/hitl";
export type {
  BaseHITLProps,
  ApprovalPromptProps,
  ActionButtonsProps,
  RunStatusProps,
  ApprovalAction,
  StatusConfig,
} from "./components/hitl";

// Tool-based UI feature
export {
  createToolUIState,
  groupToolCallsByStatus,
  groupToolCallsByName,
  filterToolCalls,
  sortToolCalls,
  buildTimeline,
  getToolCallStats,
  toggleToolCallExpanded,
  toggleActivityExpanded,
  expandAllToolCalls,
  collapseAllToolCalls,
} from "./features/tool-ui";
export type {
  ToolBasedUIConfig,
  ToolBasedUIProps,
  ToolBasedUIState,
  ToolCallGroup,
  TimelineEntry,
} from "./features/tool-ui";

// Agentic chat feature
export {
  createAgenticChatState,
  mergeAgenticChatConfig,
  getDisplayMessages,
  getStreamingMessage,
  shouldShowTypingIndicator,
  validateInput,
  getCharacterCountDisplay,
  handleComposerKeyDown,
  autoResizeTextarea,
  scrollToBottom,
  isScrolledToBottom,
  formatMessageTimestamp,
  getRoleInitials,
  defaultAgenticChatConfig,
} from "./features/agentic-chat";
export type {
  AgenticChatConfig,
  AgenticChatProps,
  AgenticChatState,
  ChatEventHandlers,
  TypingIndicatorConfig,
} from "./features/agentic-chat";
