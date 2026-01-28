import { describe, it, expect, beforeEach } from "vitest";
import {
  createToolUIState,
  groupToolCallsByStatus,
  groupToolCallsByName,
  filterToolCalls,
  sortToolCalls,
  buildTimeline,
  getToolCallStats,
  toggleToolCallExpanded,
  expandAllToolCalls,
  collapseAllToolCalls,
} from "../tool-ui/utils";
import {
  createAgenticChatState,
  mergeAgenticChatConfig,
  getDisplayMessages,
  getStreamingMessage,
  shouldShowTypingIndicator,
  validateInput,
  getCharacterCountDisplay,
} from "../agentic-chat/utils";
import {
  createInitialState,
  processEvent,
  getMessages,
  getActiveToolCalls,
  getAllToolCalls,
  EventType,
} from "../../lib/events/normalizer";
import type { NormalizedToolCall, NormalizedActivity, AccumulatedState } from "../../lib/events/types";
import type { ToolBasedUIState } from "../tool-ui/types";

/**
 * Integration tests for template flows with mock agent events
 *
 * These tests verify that the ToolBasedUI and AgenticChat utilities
 * work correctly when processing real event sequences from an agent.
 */

// Mock agent event sequences
function createMockRunStartedEvent() {
  return {
    type: EventType.RUN_STARTED,
    runId: "test-run-1",
    threadId: "test-thread-1",
    timestamp: Date.now(),
  };
}

function createMockTextMessageStartEvent(messageId: string, role: string = "assistant") {
  return {
    type: EventType.TEXT_MESSAGE_START,
    messageId,
    role,
    timestamp: Date.now(),
  };
}

function createMockTextMessageContentEvent(messageId: string, delta: string) {
  return {
    type: EventType.TEXT_MESSAGE_CONTENT,
    messageId,
    delta,
    timestamp: Date.now(),
  };
}

function createMockTextMessageEndEvent(messageId: string) {
  return {
    type: EventType.TEXT_MESSAGE_END,
    messageId,
    timestamp: Date.now(),
  };
}

// Note: The normalizer uses "toolCallName" not "name" for TOOL_CALL_START events
function createMockToolCallStartEvent(toolCallId: string, toolCallName: string, parentMessageId?: string) {
  return {
    type: EventType.TOOL_CALL_START,
    toolCallId,
    toolCallName,
    parentMessageId,
    timestamp: Date.now(),
  };
}

function createMockToolCallArgsEvent(toolCallId: string, delta: string) {
  return {
    type: EventType.TOOL_CALL_ARGS,
    toolCallId,
    delta,
    timestamp: Date.now(),
  };
}

function createMockToolCallEndEvent(toolCallId: string) {
  return {
    type: EventType.TOOL_CALL_END,
    toolCallId,
    timestamp: Date.now(),
  };
}

function createMockToolCallResultEvent(toolCallId: string, result: string) {
  return {
    type: "TOOL_CALL_RESULT" as const,
    toolCallId,
    result,
    timestamp: Date.now(),
  };
}

function createMockRunFinishedEvent() {
  return {
    type: EventType.RUN_FINISHED,
    runId: "test-run-1",
    timestamp: Date.now(),
  };
}

describe("Template Integration Tests", () => {
  describe("ToolBasedUI with mock agent flow", () => {
    it("tracks tool calls through complete lifecycle", () => {
      // Simulate a complete agent run with tool calls
      let state = createInitialState();

      // Start the run
      state = processEvent(state, createMockRunStartedEvent());
      expect(state.run.isRunning).toBe(true);

      // Start a message
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));

      // Add some content
      state = processEvent(state, createMockTextMessageContentEvent("msg-1", "Let me search for that..."));

      // Start a tool call
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "search", "msg-1"));

      // Verify tool call is active
      const activeToolCalls = getActiveToolCalls(state);
      expect(activeToolCalls.length).toBe(1);
      expect(activeToolCalls[0].name).toBe("search");
      expect(activeToolCalls[0].status).toBe("pending");

      // Add arguments
      state = processEvent(state, createMockToolCallArgsEvent("tc-1", '{"query": "test"}'));

      // Complete the tool call
      state = processEvent(state, createMockToolCallEndEvent("tc-1"));

      // Verify tool call is now completed
      const allToolCalls = getAllToolCalls(state);
      expect(allToolCalls.length).toBe(1);
      expect(allToolCalls[0].status).toBe("completed");

      // Group by status
      const groups = groupToolCallsByStatus(allToolCalls);
      expect(groups.length).toBe(1);
      expect(groups[0].label).toBe("Completed");
      expect(groups[0].collapsed).toBe(true);
    });

    it("handles multiple concurrent tool calls", () => {
      let state = createInitialState();
      state = processEvent(state, createMockRunStartedEvent());
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));

      // Start multiple tool calls
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "read_file", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-2", "search", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-3", "read_file", "msg-1"));

      const activeToolCalls = getActiveToolCalls(state);
      expect(activeToolCalls.length).toBe(3);

      // Group by name
      const byName = groupToolCallsByName(activeToolCalls);
      expect(byName.length).toBe(2); // read_file and search

      const readFileGroup = byName.find(g => g.label === "read_file");
      expect(readFileGroup?.toolCalls.length).toBe(2);

      const searchGroup = byName.find(g => g.label === "search");
      expect(searchGroup?.toolCalls.length).toBe(1);

      // Complete one tool call
      state = processEvent(state, createMockToolCallEndEvent("tc-1"));

      // Group by status should now have both pending and completed
      const allToolCalls = getAllToolCalls(state);
      const byStatus = groupToolCallsByStatus(allToolCalls);
      expect(byStatus.length).toBe(2);

      const inProgress = byStatus.find(g => g.label === "In Progress");
      expect(inProgress?.toolCalls.length).toBe(2);

      const completed = byStatus.find(g => g.label === "Completed");
      expect(completed?.toolCalls.length).toBe(1);
    });

    it("filters tool calls by name pattern", () => {
      let state = createInitialState();
      state = processEvent(state, createMockRunStartedEvent());
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));

      // Create diverse tool calls
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "read_file", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-2", "write_file", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-3", "search", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-4", "file_search", "msg-1"));

      const allToolCalls = getAllToolCalls(state);

      // Filter for "file"
      const fileToolCalls = filterToolCalls(allToolCalls, "file");
      expect(fileToolCalls.length).toBe(3); // read_file, write_file, file_search

      // Filter for "search"
      const searchToolCalls = filterToolCalls(allToolCalls, "search");
      expect(searchToolCalls.length).toBe(2); // search, file_search

      // Filter for non-existent
      const noMatch = filterToolCalls(allToolCalls, "xyz");
      expect(noMatch.length).toBe(0);
    });

    it("sorts tool calls correctly", () => {
      let state = createInitialState();
      state = processEvent(state, createMockRunStartedEvent());
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));

      // Create tool calls in specific order
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "zebra", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-2", "alpha", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-3", "beta", "msg-1"));

      // Complete first one
      state = processEvent(state, createMockToolCallEndEvent("tc-1"));

      const allToolCalls = getAllToolCalls(state);

      // Sort by name
      const byName = sortToolCalls(allToolCalls, "name");
      expect(byName[0].name).toBe("alpha");
      expect(byName[1].name).toBe("beta");
      expect(byName[2].name).toBe("zebra");

      // Sort by status (pending first, then completed)
      const byStatus = sortToolCalls(allToolCalls, "status");
      expect(byStatus[0].status).toBe("pending");
      expect(byStatus[byStatus.length - 1].status).toBe("completed");
    });

    it("manages UI expansion state", () => {
      let state = createInitialState();
      state = processEvent(state, createMockRunStartedEvent());
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "test", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-2", "test2", "msg-1"));

      const toolCalls = getAllToolCalls(state);
      let uiState = createToolUIState();

      // Initially nothing expanded
      expect(uiState.expandedToolCalls.size).toBe(0);

      // Toggle one
      uiState = toggleToolCallExpanded(uiState, "tc-1");
      expect(uiState.expandedToolCalls.has("tc-1")).toBe(true);
      expect(uiState.expandedToolCalls.has("tc-2")).toBe(false);

      // Expand all
      uiState = expandAllToolCalls(uiState, toolCalls);
      expect(uiState.expandedToolCalls.size).toBe(2);

      // Collapse all
      uiState = collapseAllToolCalls(uiState);
      expect(uiState.expandedToolCalls.size).toBe(0);
    });

    it("calculates tool call statistics", () => {
      let state = createInitialState();
      state = processEvent(state, createMockRunStartedEvent());
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));

      // Create varied tool calls
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "read_file", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-2", "read_file", "msg-1"));
      state = processEvent(state, createMockToolCallStartEvent("tc-3", "search", "msg-1"));
      state = processEvent(state, createMockToolCallEndEvent("tc-1"));

      const stats = getToolCallStats(getAllToolCalls(state));

      expect(stats.total).toBe(3);
      expect(stats.pending).toBe(2); // tc-2 and tc-3
      expect(stats.completed).toBe(1); // tc-1
      expect(stats.uniqueTools).toBe(2); // read_file and search
    });
  });

  describe("AgenticChat with mock agent flow", () => {
    it("tracks message state through streaming", () => {
      let state = createInitialState();
      const config = mergeAgenticChatConfig({ showToolCalls: true });

      // Start run
      state = processEvent(state, createMockRunStartedEvent());

      // Start message
      state = processEvent(state, createMockTextMessageStartEvent("msg-1", "assistant"));

      // Check streaming state
      let messages = getMessages(state);
      expect(messages.length).toBe(1);
      expect(messages[0].isStreaming).toBe(true);

      let streamingMsg = getStreamingMessage(messages);
      expect(streamingMsg).toBeDefined();
      expect(streamingMsg?.id).toBe("msg-1");

      // Add content
      state = processEvent(state, createMockTextMessageContentEvent("msg-1", "Hello, "));
      state = processEvent(state, createMockTextMessageContentEvent("msg-1", "how can I help?"));

      messages = getMessages(state);
      expect(messages[0].content).toBe("Hello, how can I help?");

      // End message
      state = processEvent(state, createMockTextMessageEndEvent("msg-1"));

      messages = getMessages(state);
      expect(messages[0].isStreaming).toBe(false);

      streamingMsg = getStreamingMessage(messages);
      expect(streamingMsg).toBeUndefined();
    });

    it("shows typing indicator when appropriate", () => {
      let state = createInitialState();

      // Not running - no typing indicator
      let messages = getMessages(state);
      expect(shouldShowTypingIndicator(messages, false)).toBe(false);

      // Running but no messages yet - show typing
      state = processEvent(state, createMockRunStartedEvent());
      messages = getMessages(state);
      expect(shouldShowTypingIndicator(messages, true)).toBe(true);

      // Streaming message - no typing indicator
      state = processEvent(state, createMockTextMessageStartEvent("msg-1"));
      messages = getMessages(state);
      expect(shouldShowTypingIndicator(messages, true)).toBe(false);
    });

    it("filters messages for display", () => {
      let state = createInitialState();
      const showToolCallsConfig = mergeAgenticChatConfig({ showToolCalls: true });
      const hideToolCallsConfig = mergeAgenticChatConfig({ showToolCalls: false });

      state = processEvent(state, createMockRunStartedEvent());

      // Add user message
      state = processEvent(state, createMockTextMessageStartEvent("msg-1", "user"));
      state = processEvent(state, createMockTextMessageContentEvent("msg-1", "Hello"));
      state = processEvent(state, createMockTextMessageEndEvent("msg-1"));

      // Add assistant message
      state = processEvent(state, createMockTextMessageStartEvent("msg-2", "assistant"));
      state = processEvent(state, createMockTextMessageContentEvent("msg-2", "Hi there!"));
      state = processEvent(state, createMockTextMessageEndEvent("msg-2"));

      const messages = getMessages(state);

      // With showToolCalls enabled
      const displayWithTools = getDisplayMessages(messages, showToolCallsConfig);
      expect(displayWithTools.length).toBe(2);

      // With showToolCalls disabled
      const displayWithoutTools = getDisplayMessages(messages, hideToolCallsConfig);
      expect(displayWithoutTools.length).toBe(2);
    });

    it("validates user input correctly", () => {
      const maxLength = 100;

      // Empty input
      expect(validateInput("", maxLength).valid).toBe(false);
      expect(validateInput("   ", maxLength).valid).toBe(false);

      // Valid input
      expect(validateInput("Hello", maxLength).valid).toBe(true);

      // Too long
      const longText = "a".repeat(150);
      const result = validateInput(longText, maxLength);
      expect(result.valid).toBe(false);
      expect(result.error).toContain("too long");
    });

    it("calculates character count display", () => {
      const maxLength = 100;

      // Short text (under 80% threshold)
      const short = getCharacterCountDisplay("Hello", maxLength);
      expect(short.count).toBe("5/100");
      expect(short.warning).toBe(false);

      // Long text (over 80% threshold)
      const long = getCharacterCountDisplay("a".repeat(85), maxLength);
      expect(long.count).toBe("85/100");
      expect(long.warning).toBe(true);
    });
  });

  describe("Combined template flow", () => {
    it("processes a full agent conversation with tool calls", () => {
      let state = createInitialState();
      const chatConfig = mergeAgenticChatConfig({ showToolCalls: true });
      let uiState = createToolUIState();

      // Start run
      state = processEvent(state, createMockRunStartedEvent());
      expect(state.run.isRunning).toBe(true);

      // User asks a question (simulated by adding user message)
      state = processEvent(state, createMockTextMessageStartEvent("user-1", "user"));
      state = processEvent(state, createMockTextMessageContentEvent("user-1", "What files are in the project?"));
      state = processEvent(state, createMockTextMessageEndEvent("user-1"));

      // Assistant starts responding
      state = processEvent(state, createMockTextMessageStartEvent("assistant-1", "assistant"));
      state = processEvent(state, createMockTextMessageContentEvent("assistant-1", "Let me check that for you."));

      // Assistant makes a tool call
      state = processEvent(state, createMockToolCallStartEvent("tc-1", "list_files", "assistant-1"));
      state = processEvent(state, createMockToolCallArgsEvent("tc-1", '{"path": "/project"}'));

      // Verify we can see the active tool call (streaming after args added)
      let activeToolCalls = getActiveToolCalls(state);
      expect(activeToolCalls.length).toBe(1);

      const stats = getToolCallStats(activeToolCalls);
      // After args are added, status is "streaming", not "pending"
      expect(stats.streaming).toBe(1);
      expect(stats.completed).toBe(0);

      // Tool call completes
      state = processEvent(state, createMockToolCallEndEvent("tc-1"));

      activeToolCalls = getActiveToolCalls(state);
      expect(activeToolCalls.length).toBe(0);

      const allToolCalls = getAllToolCalls(state);
      expect(allToolCalls.length).toBe(1);
      expect(allToolCalls[0].status).toBe("completed");

      // Expand the tool call in UI
      uiState = toggleToolCallExpanded(uiState, "tc-1");
      expect(uiState.expandedToolCalls.has("tc-1")).toBe(true);

      // Assistant continues
      state = processEvent(state, createMockTextMessageContentEvent("assistant-1", " Here are the files I found..."));
      state = processEvent(state, createMockTextMessageEndEvent("assistant-1"));

      // Run finishes
      state = processEvent(state, createMockRunFinishedEvent());
      expect(state.run.isRunning).toBe(false);

      // Verify final state
      const messages = getMessages(state);
      const displayMessages = getDisplayMessages(messages, chatConfig);

      // Should have user message and assistant message
      const userMessages = displayMessages.filter(m => m.role === "user");
      const assistantMessages = displayMessages.filter(m => m.role === "assistant");

      expect(userMessages.length).toBe(1);
      expect(assistantMessages.length).toBe(1);
      expect(assistantMessages[0].content).toContain("Let me check");
    });

    it("builds timeline from tool calls and activities", () => {
      const toolCalls: NormalizedToolCall[] = [
        {
          id: "tc-1",
          name: "search",
          arguments: "{}",
          status: "completed",
          result: "Found results",
          parentMessageId: "msg-1",
        },
        {
          id: "tc-2",
          name: "read",
          arguments: "{}",
          status: "pending",
          parentMessageId: "msg-1",
        },
      ];

      // NormalizedActivity uses content as an object and requires messageId
      const activities: NormalizedActivity[] = [
        {
          id: "act-1",
          type: "thinking",
          messageId: "msg-act-1",
          timestamp: 1000,
          content: { status: "Processing..." },
        },
        {
          id: "act-2",
          type: "progress",
          messageId: "msg-act-2",
          timestamp: 2000,
          content: { status: "Almost done..." },
        },
      ];

      const timeline = buildTimeline(toolCalls, activities);

      // Should have entries for tool starts, tool end (for completed), and activities
      // tc-1 start + tc-1 end + tc-2 start + act-1 + act-2 = 5 entries
      expect(timeline.length).toBe(5);

      // Activities should be sorted by timestamp
      const activityEntries = timeline.filter(e => e.type === "activity");
      expect(activityEntries.length).toBe(2);
    });
  });
});
