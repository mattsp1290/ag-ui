<script lang="ts">
  import {
    flattenState,
    formatValue,
    isExpandable,
    type StateTreeNode,
  } from "@ag-ui/svelte";

  export let state: Record<string, unknown> = {};

  // Track expanded paths
  let expandedPaths = new Set<string>([""]); // Root expanded by default

  // Compute tree nodes
  $: nodes = flattenState(state, expandedPaths, 10);

  function toggleExpand(node: StateTreeNode) {
    const pathKey = node.path.join(".");
    if (expandedPaths.has(pathKey)) {
      expandedPaths.delete(pathKey);
    } else {
      expandedPaths.add(pathKey);
    }
    expandedPaths = new Set(expandedPaths); // Trigger reactivity
  }

  function isExpanded(node: StateTreeNode): boolean {
    return expandedPaths.has(node.path.join("."));
  }

  function getTypeClass(type: StateTreeNode["type"]): string {
    switch (type) {
      case "string":
        return "type-string";
      case "number":
        return "type-number";
      case "boolean":
        return "type-boolean";
      case "null":
      case "undefined":
        return "type-null";
      default:
        return "type-object";
    }
  }
</script>

<div class="state-panel">
  <h3>Shared State</h3>

  {#if Object.keys(state).length === 0}
    <p class="empty">No state data</p>
  {:else}
    <div class="state-tree">
      {#each nodes as node (node.path.join("."))}
        <div
          class="tree-node"
          style="padding-left: {node.depth * 16}px"
        >
          {#if node.expandable}
            <button
              class="expand-btn"
              class:expanded={isExpanded(node)}
              on:click={() => toggleExpand(node)}
            >
              {isExpanded(node) ? "▼" : "▶"}
            </button>
          {:else}
            <span class="expand-placeholder"></span>
          {/if}

          <span class="node-key">{node.key}:</span>

          {#if !node.expandable || !isExpanded(node)}
            <span class="node-value {getTypeClass(node.type)}">
              {formatValue(node.value)}
            </span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .state-panel {
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

  .state-tree {
    font-family: monospace;
    font-size: 0.8125rem;
  }

  .tree-node {
    display: flex;
    align-items: center;
    padding: 0.25rem 0;
    gap: 0.25rem;
  }

  .expand-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 0;
    width: 16px;
    font-size: 0.625rem;
    color: #666;
  }

  .expand-btn:hover {
    color: #333;
  }

  .expand-placeholder {
    width: 16px;
  }

  .node-key {
    color: #881391;
  }

  .node-value {
    margin-left: 0.25rem;
  }

  .type-string {
    color: #a31515;
  }

  .type-number {
    color: #098658;
  }

  .type-boolean {
    color: #0000ff;
  }

  .type-null {
    color: #666;
    font-style: italic;
  }

  .type-object {
    color: #333;
  }
</style>
