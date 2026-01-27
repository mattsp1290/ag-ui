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
