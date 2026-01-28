<script lang="ts">
  import Chat from "./lib/Chat.svelte";
</script>

<main>
  <div class="container">
    <header>
      <h1>AG-UI Svelte Example</h1>
      <p class="subtitle">Interactive Demo with Mock Agent</p>
      <p class="instructions">
        Try these commands: <code>help</code> | <code>calculate</code> | <code>state</code> | <code>error</code>
      </p>
    </header>
    <Chat />
    <footer>
      <p>
        Powered by <a href="https://docs.ag-ui.com" target="_blank" rel="noopener">AG-UI Protocol</a>
        &middot;
        <a href="https://github.com/ag-ui-protocol/ag-ui/tree/main/integrations/community/svelte/typescript" target="_blank" rel="noopener">View Source</a>
      </p>
    </footer>
  </div>
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    background: #f5f5f5;
  }

  main {
    min-height: 100vh;
    padding: 1rem;
  }

  .container {
    max-width: 800px;
    margin: 0 auto;
  }

  header {
    text-align: center;
    margin-bottom: 2rem;
  }

  h1 {
    margin: 0;
    color: #1a1a1a;
  }

  .subtitle {
    color: #666;
    margin: 0.5rem 0 0;
  }

  .instructions {
    margin: 1rem 0 0;
    color: #666;
    font-size: 0.875rem;
  }

  .instructions code {
    background: #e9ecef;
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
    font-size: 0.8125rem;
  }

  footer {
    text-align: center;
    margin-top: 2rem;
    color: #999;
    font-size: 0.875rem;
  }

  footer a {
    color: #007bff;
    text-decoration: none;
  }

  footer a:hover {
    text-decoration: underline;
  }
</style>
