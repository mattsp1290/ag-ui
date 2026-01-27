<script lang="ts">
  import { HttpAgent } from "@ag-ui/client";
  import { createAgentStore } from "@ag-ui/svelte";
  import MessageList from "./MessageList.svelte";
  import ChatInput from "./ChatInput.svelte";
  import ToolCallPanel from "./ToolCallPanel.svelte";

  // Create the agent pointing to the API endpoint
  const agent = new HttpAgent({
    url: "/api/agent",
  });

  // Create the reactive store
  const store = createAgentStore(agent, {
    debug: true,
  });

  const { messages, isRunning, error, toolCalls, status, start, cancel, clearError } = store;

  // Handle sending a message
  async function handleSend(text: string) {
    await start({ text });
  }
</script>

<div class="chat-container">
  <div class="chat-main">
    <MessageList messages={$messages} isStreaming={$isRunning} />

    {#if $error}
      <div class="error-banner">
        <span>{$error.message}</span>
        <button on:click={clearError}>Dismiss</button>
      </div>
    {/if}

    <ChatInput
      onSend={handleSend}
      disabled={$isRunning}
      onCancel={cancel}
      isRunning={$isRunning}
    />
  </div>

  {#if $toolCalls.length > 0}
    <aside class="tool-panel">
      <ToolCallPanel toolCalls={$toolCalls} />
    </aside>
  {/if}
</div>

<style>
  .chat-container {
    display: flex;
    gap: 1rem;
    height: 600px;
  }

  .chat-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: white;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    overflow: hidden;
  }

  .tool-panel {
    width: 300px;
    background: white;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    overflow: auto;
    padding: 1rem;
  }

  .error-banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    background: #fee;
    border-left: 4px solid #c00;
    margin: 0 1rem;
    border-radius: 4px;
  }

  .error-banner button {
    background: none;
    border: 1px solid #c00;
    color: #c00;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
