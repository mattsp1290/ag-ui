#!/bin/bash
# Project: AG-UI Svelte Integration
# Generated: 2026-01-26

set -e

# Initialize beads if needed
if [ ! -d ".beads" ]; then
  bd init
fi

echo "Creating project beads..."

# ========================================
# Phase 1: Project Setup & Infrastructure
# ========================================

SETUP_SVELTE_PKG=$(bd create "Scaffold Svelte integration package structure with base configs | reserve: integrations/community/typescript/svelte/**" -p 0 --labels setup --silent)

SETUP_WORKSPACE=$(bd create "Wire workspace/turbo paths for Svelte package inclusion | reserve: pnpm-workspace.yaml, turbo.json, package.json" -p 0 --labels setup --silent)
bd dep add $SETUP_WORKSPACE $SETUP_SVELTE_PKG

SETUP_TOOLING=$(bd create "Configure Svelte build/test/lint tooling and packaging outputs | reserve: integrations/community/typescript/svelte/{package.json,svelte.config.js,tsconfig.json,vitest.config.*,tsup.config.*}" -p 0 --labels setup --silent)
bd dep add $SETUP_TOOLING $SETUP_SVELTE_PKG

SETUP_EXAMPLE_SHELL=$(bd create "Create Vite Svelte example app scaffold for manual QA | reserve: integrations/community/typescript/svelte/examples/**" -p 1 --labels setup --silent)
bd dep add $SETUP_EXAMPLE_SHELL $SETUP_TOOLING

# ========================================
# Phase 2: Core Architecture
# ========================================

CORE_AGENT_STORE=$(bd create "Implement Svelte agent store wrapping AbstractAgent lifecycle | reserve: integrations/community/typescript/svelte/src/stores/agent/**" -p 0 --labels core --silent)
bd dep add $CORE_AGENT_STORE $SETUP_TOOLING

CORE_RUN_UTILS=$(bd create "Add run helpers (start/cancel/reconnect) + error propagation | reserve: integrations/community/typescript/svelte/src/stores/run/**, integrations/community/typescript/svelte/src/lib/errors.ts" -p 0 --labels core --silent)
bd dep add $CORE_RUN_UTILS $CORE_AGENT_STORE

CORE_CONTEXT=$(bd create "Add Svelte context provider + hooks for agent/store/config | reserve: integrations/community/typescript/svelte/src/context/**, integrations/community/typescript/svelte/src/index.ts" -p 0 --labels core --silent)
bd dep add $CORE_CONTEXT $CORE_AGENT_STORE

CORE_EVENT_UTILS=$(bd create "Normalize AG-UI events into message/tool/activity models | reserve: integrations/community/typescript/svelte/src/lib/events/**" -p 1 --labels core --silent)
bd dep add $CORE_EVENT_UTILS $CORE_AGENT_STORE

CORE_STATE_STORE=$(bd create "Implement shared state store with JSON Patch apply | reserve: integrations/community/typescript/svelte/src/stores/state/**" -p 1 --labels core --silent)
bd dep add $CORE_STATE_STORE $CORE_AGENT_STORE

CORE_ACTIVITY_REGISTRY=$(bd create "Implement activity renderer registry for generative UI | reserve: integrations/community/typescript/svelte/src/lib/activity/**" -p 1 --labels core --silent)
bd dep add $CORE_ACTIVITY_REGISTRY $CORE_EVENT_UTILS

CORE_SECURITY_SANITIZE=$(bd create "Add content sanitization utilities for messages/tool output | reserve: integrations/community/typescript/svelte/src/lib/sanitize/**" -p 1 --labels core --silent)
bd dep add $CORE_SECURITY_SANITIZE $CORE_EVENT_UTILS

# ========================================
# Phase 3: UI Components
# ========================================

UI_CHAT_COMPONENTS=$(bd create "Build base chat components (ChatRoot, MessageList, MessageItem, Composer) | reserve: integrations/community/typescript/svelte/src/components/chat/**" -p 1 --labels ui --silent)
bd dep add $UI_CHAT_COMPONENTS $CORE_CONTEXT
bd dep add $UI_CHAT_COMPONENTS $CORE_EVENT_UTILS
bd dep add $UI_CHAT_COMPONENTS $CORE_SECURITY_SANITIZE

UI_TOOL_COMPONENTS=$(bd create "Build tool call components (ToolCallList, ToolCallCard, ToolResult) | reserve: integrations/community/typescript/svelte/src/components/tools/**" -p 1 --labels ui --silent)
bd dep add $UI_TOOL_COMPONENTS $CORE_EVENT_UTILS
bd dep add $UI_TOOL_COMPONENTS $CORE_SECURITY_SANITIZE

UI_ACTIVITY_COMPONENTS=$(bd create "Build activity/generative UI components (ActivityHost, ActivitySlot, ActivityRenderer) | reserve: integrations/community/typescript/svelte/src/components/activity/**" -p 1 --labels ui --silent)
bd dep add $UI_ACTIVITY_COMPONENTS $CORE_ACTIVITY_REGISTRY
bd dep add $UI_ACTIVITY_COMPONENTS $CORE_CONTEXT

UI_HITL_COMPONENTS=$(bd create "Build human-in-the-loop components (ApprovalPrompt, ActionButtons, RunStatus) | reserve: integrations/community/typescript/svelte/src/components/hitl/**" -p 2 --labels ui --silent)
bd dep add $UI_HITL_COMPONENTS $CORE_CONTEXT
bd dep add $UI_HITL_COMPONENTS $CORE_EVENT_UTILS

UI_STATE_COMPONENTS=$(bd create "Build shared state viewer components (StatePanel, StateDiff, JsonViewer) | reserve: integrations/community/typescript/svelte/src/components/state/**" -p 2 --labels ui --silent)
bd dep add $UI_STATE_COMPONENTS $CORE_STATE_STORE
bd dep add $UI_STATE_COMPONENTS $CORE_SECURITY_SANITIZE

UI_UTILITY_COMPONENTS=$(bd create "Add shared UI utilities (ErrorBanner, Loading, EmptyState) | reserve: integrations/community/typescript/svelte/src/components/ui/**" -p 3 --labels ui --silent)
bd dep add $UI_UTILITY_COMPONENTS $CORE_CONTEXT

# ========================================
# Phase 4: Feature Templates & Example App
# ========================================

FEATURE_AGENTIC_CHAT=$(bd create "Create AgenticChat template component with defaults | reserve: integrations/community/typescript/svelte/src/features/agentic-chat/**" -p 2 --labels feature-agentic-chat --silent)
bd dep add $FEATURE_AGENTIC_CHAT $UI_CHAT_COMPONENTS
bd dep add $FEATURE_AGENTIC_CHAT $UI_UTILITY_COMPONENTS

FEATURE_TOOL_UI=$(bd create "Create ToolBasedUI template with tool call + activity rendering | reserve: integrations/community/typescript/svelte/src/features/tool-ui/**" -p 2 --labels feature-tool-ui --silent)
bd dep add $FEATURE_TOOL_UI $UI_TOOL_COMPONENTS
bd dep add $FEATURE_TOOL_UI $UI_ACTIVITY_COMPONENTS

FEATURE_SHARED_STATE=$(bd create "Create SharedState template with state viewer | reserve: integrations/community/typescript/svelte/src/features/shared-state/**" -p 2 --labels feature-shared-state --silent)
bd dep add $FEATURE_SHARED_STATE $UI_STATE_COMPONENTS

FEATURE_HITL=$(bd create "Create HumanInLoop template with approval UI | reserve: integrations/community/typescript/svelte/src/features/hitl/**" -p 2 --labels feature-hitl --silent)
bd dep add $FEATURE_HITL $UI_HITL_COMPONENTS

FEATURE_EXAMPLE_APP=$(bd create "Wire example Svelte app to templates + mock agent | reserve: integrations/community/typescript/svelte/examples/**" -p 2 --labels feature-demo --silent)
bd dep add $FEATURE_EXAMPLE_APP $SETUP_EXAMPLE_SHELL
bd dep add $FEATURE_EXAMPLE_APP $FEATURE_AGENTIC_CHAT
bd dep add $FEATURE_EXAMPLE_APP $FEATURE_TOOL_UI
bd dep add $FEATURE_EXAMPLE_APP $FEATURE_SHARED_STATE
bd dep add $FEATURE_EXAMPLE_APP $FEATURE_HITL

# ========================================
# Phase 5: Testing
# ========================================

TEST_STORE=$(bd create "Unit tests for agent/store event handling | reserve: integrations/community/typescript/svelte/src/stores/**/__tests__/**" -p 1 --labels testing --silent)
bd dep add $TEST_STORE $CORE_AGENT_STORE
bd dep add $TEST_STORE $CORE_EVENT_UTILS

TEST_COMPONENTS=$(bd create "Component tests for chat/tool/activity components | reserve: integrations/community/typescript/svelte/src/components/**/__tests__/**" -p 1 --labels testing --silent)
bd dep add $TEST_COMPONENTS $UI_CHAT_COMPONENTS
bd dep add $TEST_COMPONENTS $UI_TOOL_COMPONENTS
bd dep add $TEST_COMPONENTS $UI_ACTIVITY_COMPONENTS

TEST_INTEGRATION=$(bd create "Integration tests for template flows with mock agent | reserve: integrations/community/typescript/svelte/tests/integration/**" -p 2 --labels testing --silent)
bd dep add $TEST_INTEGRATION $FEATURE_AGENTIC_CHAT
bd dep add $TEST_INTEGRATION $FEATURE_TOOL_UI
bd dep add $TEST_INTEGRATION $FEATURE_SHARED_STATE
bd dep add $TEST_INTEGRATION $FEATURE_HITL

TEST_SECURITY=$(bd create "Security tests for sanitization + unsafe content handling | reserve: integrations/community/typescript/svelte/src/lib/sanitize/**, integrations/community/typescript/svelte/tests/security/**" -p 2 --labels testing --silent)
bd dep add $TEST_SECURITY $CORE_SECURITY_SANITIZE
bd dep add $TEST_SECURITY $UI_CHAT_COMPONENTS

TEST_E2E=$(bd create "E2E smoke test for example app | reserve: integrations/community/typescript/svelte/examples/**, integrations/community/typescript/svelte/tests/e2e/**" -p 3 --labels testing --silent)
bd dep add $TEST_E2E $FEATURE_EXAMPLE_APP

# ========================================
# Phase 6: Documentation
# ========================================

DOCS_README=$(bd create "Write Svelte integration README with install/usage/API | reserve: integrations/community/typescript/svelte/README.md" -p 1 --labels docs --silent)
bd dep add $DOCS_README $FEATURE_AGENTIC_CHAT
bd dep add $DOCS_README $FEATURE_TOOL_UI

DOCS_PROMPTS=$(bd create "Add architecture + component API notes in proompts/docs | reserve: proompts/docs/**" -p 1 --labels docs --silent)
bd dep add $DOCS_PROMPTS $CORE_CONTEXT
bd dep add $DOCS_PROMPTS $UI_CHAT_COMPONENTS

DOCS_SITE=$(bd create "Update docs site to list Svelte integration + snippets | reserve: docs/**" -p 2 --labels docs --silent)
bd dep add $DOCS_SITE $DOCS_README

# ========================================
# Phase 7: Performance & Deployment
# ========================================

PERF_TUNING=$(bd create "Performance pass: event batching + message list optimization | reserve: integrations/community/typescript/svelte/src/stores/**, integrations/community/typescript/svelte/src/components/chat/**, docs/**" -p 2 --labels core --silent)
bd dep add $PERF_TUNING $UI_CHAT_COMPONENTS
bd dep add $PERF_TUNING $CORE_EVENT_UTILS

DEPLOY_PUBLISH=$(bd create "Update publish filters/versioning for Svelte package | reserve: package.json, pnpm-workspace.yaml" -p 2 --labels deploy --silent)
bd dep add $DEPLOY_PUBLISH $SETUP_WORKSPACE
bd dep add $DEPLOY_PUBLISH $DOCS_README

echo ""
echo "Bead graph created! View with:"
echo "  bv                    # Interactive TUI"
echo "  bv --robot-insights   # AI-friendly recommendations"
echo "  bd ready              # List unblocked tasks"
