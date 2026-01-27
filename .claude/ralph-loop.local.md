---
active: true
iteration: 1
max_iterations: 5000
completion_promise: "COMPLETE"
started_at: "2026-01-27T03:09:40Z"
---

# Ralph Loop: ag-ui

## Mode: Autopilot

## Project Context
- Branch: matt.spurlin/svelte-integration
- Status: 1 staged, 2 untracked
- Recent: ralph: add example app and documentation for @ag-ui/svelte [84 seconds ago]

## Current Objective
Work through the entire task graph autonomously.

### Ready Tasks
📋 Ready work (10 issues with no blockers):

1. [● P0] [task] ag-ui-0nt: Wire workspace/turbo paths for Svelte package inclusion | reserve: pnpm-workspace.yaml, turbo.json, package.json
2. [● P0] [task] ag-ui-bky: Add Svelte context provider + hooks for agent/store/config | reserve: integrations/community/typescript/svelte/src/context/**, integrations/community/typescript/svelte/src/index.ts
3. [● P1] [task] ag-ui-cpo: Implement activity renderer registry for generative UI | reserve: integrations/community/typescript/svelte/src/lib/activity/**
4. [● P1] [task] ag-ui-459: Add content sanitization utilities for messages/tool output | reserve: integrations/community/typescript/svelte/src/lib/sanitize/**
5. [● P2] [task] ag-ui-0rb: Build human-in-the-loop components (ApprovalPrompt, ActionButtons, RunStatus) | reserve: integrations/community/typescript/svelte/src/components/hitl/**
6. [● P2] [task] ag-ui-5cy: Create AgenticChat template component with defaults | reserve: integrations/community/typescript/svelte/src/features/agentic-chat/**
7. [● P2] [task] ag-ui-555: Performance pass: event batching + message list optimization | reserve: integrations/community/typescript/svelte/src/stores/**, integrations/community/typescript/svelte/src/components/chat/**, docs/**
8. [● P2] [task] ag-ui-dpf: Build shared state viewer components (StatePanel, StateDiff, JsonViewer) | reserve: integrations/community/typescript/svelte/src/components/state/**
9. [● P2] [task] ag-ui-t01: Create ToolBasedUI template with tool call + activity rendering | reserve: integrations/community/typescript/svelte/src/features/tool-ui/**
10. [● P3] [task] ag-ui-bg4: E2E smoke test for example app | reserve: integrations/community/typescript/svelte/examples/**, integrations/community/typescript/svelte/tests/e2e/**

Process tasks in priority order. After completing each task:
1. Mark it closed: 
2. Check for newly unblocked tasks
3. Continue with the next highest priority task

## Completion Requirements (CRITICAL)
Both conditions must be met for completion:

1. Verification signals must pass:
   pnpm test

2. Explicit completion promise:
   When the objective is fully complete, output: <promise>COMPLETE</promise>

## Checkpoint Commits
After each successful iteration [tests pass], create a checkpoint commit:
   git add -A && git commit -m ralph: iteration N - [brief summary]

## Iteration Protocol
1. ASSESS - Review current state and what is needed next
2. EXECUTE - Make one focused, incremental change
3. VERIFY - Run tests/build to confirm changes work
4. CHECKPOINT - Commit if tests pass
5. EVALUATE - Output <promise>COMPLETE</promise> when done, else continue

Begin working now.
