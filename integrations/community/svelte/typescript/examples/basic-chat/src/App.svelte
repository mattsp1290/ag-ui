<script lang="ts">
  import Chat from "./lib/Chat.svelte";
</script>

<main>
  <div class="container">
    <header>
      <h1>AG-UI Svelte Example</h1>
      <p class="subtitle">Basic Chat with Agent</p>
    </header>
    <Chat />
  </div>
</main>

<style>
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
</style>
