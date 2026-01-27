import type { Readable } from "svelte/store";
import type { AgentStore } from "../stores/run/types";

/**
 * Configuration for the AG-UI context provider
 */
export interface AgentContextConfig {
  /** Enable debug logging */
  debug?: boolean;
  /** Initial messages to populate the store */
  initialMessages?: Array<{ id: string; role: string; content?: string }>;
  /** Initial agent state */
  initialState?: Record<string, unknown>;
}

/**
 * Context value provided to child components
 */
export interface AgentContextValue {
  /** The agent store instance */
  store: AgentStore;
  /** Configuration used to create the context */
  config: AgentContextConfig;
}

/**
 * Context key type
 */
export const AGENT_CONTEXT_KEY = Symbol("ag-ui-agent-context");
