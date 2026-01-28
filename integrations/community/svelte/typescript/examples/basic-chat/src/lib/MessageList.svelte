<script lang="ts">
  import type { NormalizedMessage } from "@ag-ui/svelte";

  export let messages: NormalizedMessage[] = [];
  export let isStreaming = false;

  function getRoleLabel(role: string): string {
    switch (role) {
      case "user":
        return "You";
      case "assistant":
        return "Agent";
      case "system":
        return "System";
      case "tool":
        return "Tool";
      default:
        return role;
    }
  }

  function getRoleClass(role: string): string {
    return `message-${role}`;
  }
</script>

<div class="message-list">
  {#if messages.length === 0}
    <div class="empty-state">
      <p>No messages yet. Start a conversation!</p>
    </div>
  {:else}
    {#each messages as message (message.id)}
      <div class="message {getRoleClass(message.role)}">
        <div class="message-header">
          <span class="role">{getRoleLabel(message.role)}</span>
          {#if message.isStreaming}
            <span class="streaming-indicator">...</span>
          {/if}
        </div>
        <div class="message-content">
          {message.content}
        </div>
      </div>
    {/each}
  {/if}

  {#if isStreaming}
    <div class="typing-indicator">
      <span></span>
      <span></span>
      <span></span>
    </div>
  {/if}
</div>

<style>
  .message-list {
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #999;
  }

  .message {
    padding: 0.75rem 1rem;
    border-radius: 8px;
    max-width: 80%;
  }

  .message-user {
    background: #007bff;
    color: white;
    align-self: flex-end;
  }

  .message-assistant {
    background: #f0f0f0;
    color: #333;
    align-self: flex-start;
  }

  .message-system {
    background: #fff3cd;
    color: #856404;
    align-self: center;
    font-size: 0.875rem;
  }

  .message-tool {
    background: #e7f3ff;
    color: #004085;
    align-self: flex-start;
    font-family: monospace;
    font-size: 0.875rem;
  }

  .message-header {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.25rem;
    font-size: 0.75rem;
    opacity: 0.7;
  }

  .streaming-indicator {
    animation: pulse 1s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.4; }
    50% { opacity: 1; }
  }

  .message-content {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .typing-indicator {
    display: flex;
    gap: 4px;
    padding: 0.5rem;
    align-self: flex-start;
  }

  .typing-indicator span {
    width: 8px;
    height: 8px;
    background: #999;
    border-radius: 50%;
    animation: bounce 1.4s ease-in-out infinite both;
  }

  .typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
  .typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

  @keyframes bounce {
    0%, 80%, 100% {
      transform: scale(0);
    }
    40% {
      transform: scale(1);
    }
  }
</style>
