# @ag-ui/svelte Architecture & Component API

## Overview

The @ag-ui/svelte package provides a reactive Svelte integration for the AG-UI protocol. It offers stores and utilities for building agent-powered UIs with Svelte 4 and 5.

## Architecture

```
@ag-ui/svelte
├── stores/
│   └── run/
│       ├── agent-store.ts    # Main reactive store
│       └── types.ts          # Store interfaces
├── lib/
│   ├── errors.ts             # Error classes
│   └── events/
│       ├── normalizer.ts     # Event processing
│       └── types.ts          # Event types
├── components/
│   ├── ui/                   # UI utilities
│   └── tools/                # Tool call utilities
└── features/
    ├── shared-state/         # State viewer utilities
    └── hitl/                  # Human-in-the-loop
```

## Core Concepts

### Agent Store

The `createAgentStore` function creates a reactive store wrapping an AG-UI agent:

```typescript
import { HttpAgent } from "@ag-ui/client";
import { createAgentStore } from "@ag-ui/svelte";

const agent = new HttpAgent({ url: "/api/agent" });
const store = createAgentStore(agent);
```

The store exposes:
- **Readable stores**: `messages`, `state`, `isRunning`, `error`, `toolCalls`, etc.
- **Actions**: `start()`, `cancel()`, `reconnect()`, `reset()`

### Event Normalization

AG-UI events are normalized into consistent models:

| Event Type | Normalized Model |
|------------|------------------|
| TEXT_MESSAGE_* | NormalizedMessage |
| TOOL_CALL_* | NormalizedToolCall |
| ACTIVITY_* | NormalizedActivity |
| STATE_* | Record<string, unknown> |

### Error Handling

Custom error classes provide context:

```typescript
import {
  AgentStoreError,
  RunStartError,
  RunCancelledError,
  AgentRunError
} from "@ag-ui/svelte";
```

## Component API

### Agent Store API

```typescript
interface AgentStore {
  // Readable stores
  messages: Readable<NormalizedMessage[]>;
  state: Readable<Record<string, unknown>>;
  isRunning: Readable<boolean>;
  status: Readable<RunStatus>;
  error: Readable<Error | null>;
  toolCalls: Readable<NormalizedToolCall[]>;
  activeToolCalls: Readable<NormalizedToolCall[]>;
  activities: Readable<NormalizedActivity[]>;
  runId: Readable<string | null>;
  threadId: Readable<string | null>;
  currentStep: Readable<string | null>;

  // Actions
  start(input: StartRunInput): Promise<void>;
  cancel(): void;
  reconnect(): Promise<void>;
  addMessage(message: Message): void;
  reset(): void;
  clearError(): void;
  destroy(): void;
}
```

### StartRunInput

```typescript
interface StartRunInput {
  text?: string;           // User message text
  message?: Message;       // Full message object
  tools?: Tool[];          // Available tools
  context?: Context[];     // Context for agent
  runId?: string;          // Custom run ID
  forwardedProps?: Record<string, unknown>;
}
```

### NormalizedMessage

```typescript
interface NormalizedMessage {
  id: string;
  role: "user" | "assistant" | "system" | "developer" | "tool" | "activity";
  content: string;
  isStreaming: boolean;
  toolCalls?: NormalizedToolCall[];
  timestamp?: number;
}
```

### NormalizedToolCall

```typescript
interface NormalizedToolCall {
  id: string;
  name: string;
  arguments: string;
  parsedArguments?: Record<string, unknown>;
  result?: string;
  error?: string;
  status: "pending" | "streaming" | "completed" | "error";
  parentMessageId?: string;
}
```

## Features

### Human-in-the-Loop (HITL)

```typescript
import { createHITLStore, withHITL } from "@ag-ui/svelte";

const hitlStore = createHITLStore({
  requireApproval: ["dangerousTool"],
  autoApprove: ["safeTool"],
});

const store = withHITL(createAgentStore(agent), hitlStore);
```

### Shared State Viewer

Utilities for displaying agent state:

```typescript
import {
  flattenState,
  formatValue,
  getValuePreview
} from "@ag-ui/svelte";

// Create tree nodes for rendering
const nodes = flattenState(state, expandedPaths);
```

## Usage Examples

### Basic Chat

```svelte
<script lang="ts">
  import { HttpAgent } from "@ag-ui/client";
  import { createAgentStore } from "@ag-ui/svelte";

  const store = createAgentStore(new HttpAgent({ url: "/api/agent" }));
  const { messages, isRunning, start, cancel } = store;

  let input = "";
  async function send() {
    await start({ text: input });
    input = "";
  }
</script>

{#each $messages as msg}
  <div class={msg.role}>{msg.content}</div>
{/each}

<input bind:value={input} disabled={$isRunning} />
<button on:click={send} disabled={$isRunning}>Send</button>
{#if $isRunning}
  <button on:click={cancel}>Cancel</button>
{/if}
```

### With Tool Calls

```svelte
<script lang="ts">
  const { toolCalls, activeToolCalls } = store;
</script>

{#each $toolCalls as tc}
  <div class="tool-call">
    <strong>{tc.name}</strong>
    <span class="status">{tc.status}</span>
    {#if tc.result}
      <pre>{tc.result}</pre>
    {/if}
  </div>
{/each}
```

### Error Handling

```svelte
<script lang="ts">
  import { isRunCancelled, formatError } from "@ag-ui/svelte";

  const { error, clearError } = store;
</script>

{#if $error && !isRunCancelled($error)}
  <div class="error">
    {formatError($error)}
    <button on:click={clearError}>Dismiss</button>
  </div>
{/if}
```

## Best Practices

1. **Always clean up**: Call `store.destroy()` when component unmounts
2. **Handle errors**: Subscribe to the error store and display appropriately
3. **Show streaming state**: Use `isStreaming` on messages to show loading indicators
4. **Cancel long runs**: Provide a cancel button for user control
5. **Type safety**: Use TypeScript for full type inference
