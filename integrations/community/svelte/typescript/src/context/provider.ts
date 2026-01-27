import { setContext, getContext, onDestroy } from "svelte";
import type { AgentStore } from "../stores/run/types";
import { createAgentStore } from "../stores/run/agent-store";
import type { AgentContextConfig, AgentContextValue } from "./types";
import { AGENT_CONTEXT_KEY } from "./types";

/**
 * Abstract agent interface (matches @ag-ui/client AbstractAgent)
 */
interface AbstractAgentLike {
  threadId: string;
  messages: Array<{ id: string; role: string; content?: string }>;
  state: unknown;
  subscribe(subscriber: unknown): { unsubscribe: () => void };
  addMessage(message: { id: string; role: string; content?: string }): void;
  setMessages(messages: Array<{ id: string; role: string; content?: string }>): void;
  setState(state: unknown): void;
  runAgent(params?: unknown): Promise<unknown>;
  connectAgent(params?: unknown): Promise<unknown>;
  abortRun(): void;
  detachActiveRun(): void | Promise<void>;
}

/**
 * Set up the agent context for child components.
 * Call this in a parent component's script to provide the agent store to descendants.
 *
 * @example
 * ```svelte
 * <script>
 *   import { provideAgentContext } from '@ag-ui/svelte';
 *   import { HttpAgent } from '@ag-ui/client';
 *
 *   const agent = new HttpAgent({ url: '/api/agent' });
 *   const { store } = provideAgentContext(agent);
 * </script>
 * ```
 */
export function provideAgentContext(
  agent: AbstractAgentLike,
  config: AgentContextConfig = {}
): AgentContextValue {
  const store = createAgentStore(agent as any, {
    debug: config.debug,
    initialMessages: config.initialMessages,
    initialState: config.initialState,
  });

  const contextValue: AgentContextValue = {
    store,
    config,
  };

  setContext(AGENT_CONTEXT_KEY, contextValue);

  // Clean up on component destroy
  onDestroy(() => {
    store.destroy();
  });

  return contextValue;
}

/**
 * Get the agent context from a parent provider.
 * Throws if no context is available.
 *
 * @example
 * ```svelte
 * <script>
 *   import { useAgentContext } from '@ag-ui/svelte';
 *
 *   const { store } = useAgentContext();
 *   const { messages, isRunning, start, cancel } = store;
 * </script>
 * ```
 */
export function useAgentContext(): AgentContextValue {
  const context = getContext<AgentContextValue>(AGENT_CONTEXT_KEY);
  if (!context) {
    throw new Error(
      "useAgentContext must be used within a component that has called provideAgentContext"
    );
  }
  return context;
}

/**
 * Get the agent store from context.
 * Convenience function that returns just the store.
 *
 * @example
 * ```svelte
 * <script>
 *   import { useAgentStore } from '@ag-ui/svelte';
 *
 *   const store = useAgentStore();
 *   const { messages, start } = store;
 * </script>
 * ```
 */
export function useAgentStore(): AgentStore {
  return useAgentContext().store;
}

/**
 * Check if agent context is available.
 * Returns true if a parent component has called provideAgentContext.
 */
export function hasAgentContext(): boolean {
  try {
    const context = getContext<AgentContextValue>(AGENT_CONTEXT_KEY);
    return context !== undefined;
  } catch {
    return false;
  }
}
