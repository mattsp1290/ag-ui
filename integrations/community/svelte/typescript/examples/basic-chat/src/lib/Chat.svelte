<script lang="ts">
  import { createAgentStore, createHITLStore, type NormalizedToolCall } from "@ag-ui/svelte";
  import { MockAgent } from "./mock-agent";
  import MessageList from "./MessageList.svelte";
  import ChatInput from "./ChatInput.svelte";
  import ToolCallPanel from "./ToolCallPanel.svelte";
  import StatePanel from "./StatePanel.svelte";

  // Create the mock agent for demo
  const agent = new MockAgent({
    typingDelay: 15,
    simulateToolCalls: true,
    simulateStateUpdates: true,
  });

  // Create the reactive store
  const store = createAgentStore(agent as any, {
    debug: true,
  });

  // Create HITL store for approval workflow demo
  const hitlStore = createHITLStore({
    requireApproval: ["dangerous_action"],
    autoApprove: ["calculator"],
  });

  const { messages, isRunning, error, toolCalls, status, state: agentState, start, cancel, clearError } = store;
  const { pendingApprovals } = hitlStore;

  // Track active tab for demo sections
  let activeTab: "chat" | "tools" | "state" = "chat";

  // Handle sending a message
  async function handleSend(text: string) {
    await start({ text });
  }

  // Handle approval decision
  function handleApproval(toolCall: NormalizedToolCall, approved: boolean) {
    if (approved) {
      hitlStore.approve(toolCall.id);
    } else {
      hitlStore.reject(toolCall.id, "User rejected");
    }
  }
</script>

<div class="chat-container">
  <div class="tabs">
    <button
      class="tab"
      class:active={activeTab === "chat"}
      on:click={() => (activeTab = "chat")}
    >
      Chat
    </button>
    <button
      class="tab"
      class:active={activeTab === "tools"}
      on:click={() => (activeTab = "tools")}
    >
      Tools {#if $toolCalls.length > 0}<span class="badge">{$toolCalls.length}</span>{/if}
    </button>
    <button
      class="tab"
      class:active={activeTab === "state"}
      on:click={() => (activeTab = "state")}
    >
      State
    </button>
  </div>

  <div class="tab-content">
    {#if activeTab === "chat"}
      <div class="chat-main">
        <MessageList messages={$messages} isStreaming={$isRunning} />

        {#if $error}
          <div class="error-banner">
            <span>{$error.message}</span>
            <button on:click={clearError}>Dismiss</button>
          </div>
        {/if}

        {#if $pendingApprovals.length > 0}
          <div class="approval-banner">
            <strong>Pending Approvals:</strong>
            {#each $pendingApprovals as toolCall (toolCall.id)}
              <div class="approval-item">
                <span>{toolCall.name}</span>
                <button on:click={() => handleApproval(toolCall, true)}>Approve</button>
                <button on:click={() => handleApproval(toolCall, false)}>Reject</button>
              </div>
            {/each}
          </div>
        {/if}

        <div class="status-bar">
          <span class="status-indicator" class:running={$isRunning}></span>
          <span class="status-text">{$status}</span>
        </div>

        <ChatInput
          onSend={handleSend}
          disabled={$isRunning}
          onCancel={cancel}
          isRunning={$isRunning}
        />
      </div>
    {:else if activeTab === "tools"}
      <div class="tools-main">
        <ToolCallPanel toolCalls={$toolCalls} />
        {#if $toolCalls.length === 0}
          <p class="hint">Type "calculate" in the chat to trigger a tool call demo.</p>
        {/if}
      </div>
    {:else if activeTab === "state"}
      <div class="state-main">
        <StatePanel state={$agentState} />
        {#if Object.keys($agentState).length === 0}
          <p class="hint">Type "state" in the chat to trigger a state update demo.</p>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .chat-container {
    display: flex;
    flex-direction: column;
    height: 600px;
    background: white;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    overflow: hidden;
  }

  .tabs {
    display: flex;
    border-bottom: 1px solid #eee;
    background: #f8f9fa;
  }

  .tab {
    flex: 1;
    padding: 0.75rem 1rem;
    border: none;
    background: none;
    cursor: pointer;
    font-size: 0.875rem;
    color: #666;
    transition: all 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
  }

  .tab:hover {
    background: #e9ecef;
  }

  .tab.active {
    color: #007bff;
    border-bottom: 2px solid #007bff;
    margin-bottom: -1px;
  }

  .badge {
    background: #007bff;
    color: white;
    font-size: 0.75rem;
    padding: 0.125rem 0.5rem;
    border-radius: 10px;
  }

  .tab-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .chat-main,
  .tools-main,
  .state-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .tools-main,
  .state-main {
    padding: 1rem;
    overflow: auto;
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

  .approval-banner {
    padding: 0.75rem 1rem;
    background: #fff3cd;
    border-left: 4px solid #ffc107;
    margin: 0 1rem;
    border-radius: 4px;
  }

  .approval-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }

  .approval-item button {
    padding: 0.25rem 0.5rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.75rem;
  }

  .approval-item button:first-of-type {
    background: #28a745;
    color: white;
  }

  .approval-item button:last-of-type {
    background: #dc3545;
    color: white;
  }

  .status-bar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: #f8f9fa;
    font-size: 0.75rem;
    color: #666;
  }

  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #6c757d;
  }

  .status-indicator.running {
    background: #28a745;
    animation: pulse 1s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .hint {
    color: #999;
    font-style: italic;
    text-align: center;
    margin-top: 1rem;
  }
</style>
