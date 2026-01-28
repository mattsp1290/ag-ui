<script lang="ts">
  export let onSend: (text: string) => void;
  export let onCancel: () => void;
  export let disabled = false;
  export let isRunning = false;

  let inputText = "";

  function handleSubmit(e: Event) {
    e.preventDefault();
    if (inputText.trim() && !disabled) {
      onSend(inputText.trim());
      inputText = "";
    }
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      handleSubmit(e);
    }
  }
</script>

<form class="chat-input" on:submit={handleSubmit}>
  <input
    type="text"
    bind:value={inputText}
    placeholder={disabled ? "Waiting for response..." : "Type a message..."}
    {disabled}
    on:keydown={handleKeyDown}
  />
  {#if isRunning}
    <button type="button" class="cancel-btn" on:click={onCancel}>
      Cancel
    </button>
  {:else}
    <button type="submit" disabled={!inputText.trim()}>
      Send
    </button>
  {/if}
</form>

<style>
  .chat-input {
    display: flex;
    gap: 0.5rem;
    padding: 1rem;
    border-top: 1px solid #eee;
  }

  input {
    flex: 1;
    padding: 0.75rem 1rem;
    border: 1px solid #ddd;
    border-radius: 24px;
    font-size: 1rem;
    outline: none;
    transition: border-color 0.2s;
  }

  input:focus {
    border-color: #007bff;
  }

  input:disabled {
    background: #f5f5f5;
  }

  button {
    padding: 0.75rem 1.5rem;
    border: none;
    border-radius: 24px;
    font-size: 1rem;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  button[type="submit"] {
    background: #007bff;
    color: white;
  }

  button[type="submit"]:hover:not(:disabled) {
    background: #0056b3;
  }

  button[type="submit"]:disabled {
    background: #ccc;
    cursor: not-allowed;
  }

  .cancel-btn {
    background: #dc3545;
    color: white;
  }

  .cancel-btn:hover {
    background: #c82333;
  }
</style>
