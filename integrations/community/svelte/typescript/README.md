# @ag-ui/svelte

Svelte integration for the AG-UI protocol. Provides reactive stores and components for building agent-powered user interfaces.

## Installation

```bash
npm install @ag-ui/svelte @ag-ui/client @ag-ui/core rxjs
# or
pnpm add @ag-ui/svelte @ag-ui/client @ag-ui/core rxjs
```

## Quick Start

```svelte
<script>
  import { createAgentStore } from '@ag-ui/svelte';
  import { HttpAgent } from '@ag-ui/client';

  const agent = new HttpAgent({ url: '/api/agent' });
  const store = createAgentStore(agent);

  const { messages, isRunning, error, start, cancel } = store;
</script>

<div>
  {#each $messages as message}
    <div class="message {message.role}">
      {message.content}
    </div>
  {/each}

  {#if $isRunning}
    <button on:click={cancel}>Cancel</button>
  {:else}
    <button on:click={() => start({ text: 'Hello!' })}>Send</button>
  {/if}

  {#if $error}
    <div class="error">{$error.message}</div>
  {/if}
</div>
```

## Features

- **Reactive Stores**: Svelte stores that automatically update from AG-UI event streams
- **Run Management**: Start, cancel, and reconnect agent runs with built-in state management
- **Event Normalization**: AG-UI events normalized into message, tool call, and activity models
- **Error Handling**: Comprehensive error propagation and recovery
- **TypeScript Support**: Full type definitions for all APIs

## API Reference

### `createAgentStore(agent)`

Creates a reactive store for managing agent interactions.

#### Returns

- `messages` - Readable store of Message[]
- `state` - Readable store of agent state
- `isRunning` - Readable store of boolean
- `error` - Readable store of Error | null
- `toolCalls` - Readable store of active tool calls
- `start(input)` - Start a new agent run
- `cancel()` - Cancel the current run
- `reconnect()` - Reconnect to an existing run

### Event Models

The package normalizes AG-UI events into consistent models:

- **Messages**: Text messages from user/assistant/system
- **Tool Calls**: Function calls with arguments and results
- **Activities**: Custom activity events with structured content

## SSR Considerations

The agent store should only be used client-side. In SvelteKit:

```svelte
<script>
  import { browser } from '$app/environment';
  import { createAgentStore } from '@ag-ui/svelte';
  import { HttpAgent } from '@ag-ui/client';

  let store;
  if (browser) {
    const agent = new HttpAgent({ url: '/api/agent' });
    store = createAgentStore(agent);
  }
</script>
```

## Security

### XSS Protection

The package includes built-in XSS protection via the sanitization utilities. All message content is sanitized before display.

### Content Security Policy

For defense-in-depth, configure your server with appropriate CSP headers:

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
```

## License

MIT
