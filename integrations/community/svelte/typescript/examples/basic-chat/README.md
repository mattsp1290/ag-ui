# AG-UI Svelte Basic Chat Example

A simple chat application demonstrating the @ag-ui/svelte integration.

## Features

- Real-time message streaming with AG-UI protocol
- Tool call visualization
- Cancel running requests
- Error handling

## Getting Started

1. First, start an AG-UI compatible agent server on port 3001. You can use the server-starter example:

```bash
cd integrations/server-starter/typescript
pnpm install
pnpm dev
```

2. Then, in a new terminal, start this example:

```bash
cd integrations/community/svelte/typescript/examples/basic-chat
pnpm install
pnpm dev
```

3. Open http://localhost:5173 in your browser.

## Project Structure

```
src/
  App.svelte        - Main app component
  main.ts           - App entry point
  lib/
    Chat.svelte        - Chat container with agent store
    ChatInput.svelte   - Message input component
    MessageList.svelte - Message display component
    ToolCallPanel.svelte - Tool call visualization
```

## Using the Agent Store

```svelte
<script lang="ts">
  import { HttpAgent } from "@ag-ui/client";
  import { createAgentStore } from "@ag-ui/svelte";

  const agent = new HttpAgent({ url: "/api/agent" });
  const store = createAgentStore(agent);

  const { messages, isRunning, start, cancel } = store;
</script>

<button on:click={() => start({ text: "Hello!" })}>Send</button>
```
