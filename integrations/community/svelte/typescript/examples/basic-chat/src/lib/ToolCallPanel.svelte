<script lang="ts">
  import type { NormalizedToolCall } from "@ag-ui/svelte";
  import { formatToolArguments, getToolStatus } from "@ag-ui/svelte";

  export let toolCalls: NormalizedToolCall[] = [];

  function getStatusColor(status: string): string {
    switch (status) {
      case "pending":
        return "#ffc107";
      case "streaming":
        return "#17a2b8";
      case "completed":
        return "#28a745";
      case "error":
        return "#dc3545";
      default:
        return "#6c757d";
    }
  }
</script>

<div class="tool-panel">
  <h3>Tool Calls</h3>
  {#if toolCalls.length === 0}
    <p class="empty">No tool calls</p>
  {:else}
    {#each toolCalls as toolCall (toolCall.id)}
      <div class="tool-call">
        <div class="tool-header">
          <span class="tool-name">{toolCall.name}</span>
          <span
            class="tool-status"
            style="background-color: {getStatusColor(toolCall.status)}"
          >
            {getToolStatus(toolCall).label}
          </span>
        </div>
        {#if toolCall.arguments}
          <details class="tool-details">
            <summary>Arguments</summary>
            <pre>{formatToolArguments(toolCall)}</pre>
          </details>
        {/if}
        {#if toolCall.result}
          <details class="tool-details" open>
            <summary>Result</summary>
            <pre>{toolCall.result}</pre>
          </details>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<style>
  .tool-panel {
    font-size: 0.875rem;
  }

  h3 {
    margin: 0 0 1rem;
    font-size: 1rem;
    color: #333;
  }

  .empty {
    color: #999;
    font-style: italic;
  }

  .tool-call {
    background: #f8f9fa;
    border-radius: 6px;
    padding: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .tool-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }

  .tool-name {
    font-weight: 600;
    font-family: monospace;
  }

  .tool-status {
    font-size: 0.75rem;
    padding: 0.125rem 0.5rem;
    border-radius: 10px;
    color: white;
  }

  .tool-details {
    margin-top: 0.5rem;
  }

  .tool-details summary {
    cursor: pointer;
    color: #666;
    font-size: 0.75rem;
    margin-bottom: 0.25rem;
  }

  .tool-details pre {
    background: white;
    padding: 0.5rem;
    border-radius: 4px;
    overflow-x: auto;
    font-size: 0.75rem;
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
