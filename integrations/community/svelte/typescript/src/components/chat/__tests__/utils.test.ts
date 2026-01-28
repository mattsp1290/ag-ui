import { describe, it, expect } from "vitest";
import type { NormalizedMessage } from "../../../lib/events/types";
import {
  groupMessages,
  isUserMessage,
  isAssistantMessage,
  isSystemMessage,
  isToolMessage,
  getRoleDisplayName,
  getRoleClassName,
  formatMessageTime,
  isMessageEmpty,
  getMessagePreview,
  filterMessagesByRole,
  getLastMessage,
} from "../utils";

describe("chat utils", () => {
  describe("isUserMessage", () => {
    it("returns true for user messages", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "Hello",
        isStreaming: false,
      };
      expect(isUserMessage(msg)).toBe(true);
    });

    it("returns false for non-user messages", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "assistant",
        content: "Hi",
        isStreaming: false,
      };
      expect(isUserMessage(msg)).toBe(false);
    });
  });

  describe("isAssistantMessage", () => {
    it("returns true for assistant messages", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "assistant",
        content: "Hello",
        isStreaming: false,
      };
      expect(isAssistantMessage(msg)).toBe(true);
    });
  });

  describe("isSystemMessage", () => {
    it("returns true for system messages", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "system",
        content: "System prompt",
        isStreaming: false,
      };
      expect(isSystemMessage(msg)).toBe(true);
    });
  });

  describe("isToolMessage", () => {
    it("returns true for tool messages", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "tool",
        content: '{"result": "ok"}',
        isStreaming: false,
      };
      expect(isToolMessage(msg)).toBe(true);
    });
  });

  describe("getRoleDisplayName", () => {
    it("returns 'You' for user role", () => {
      expect(getRoleDisplayName("user")).toBe("You");
    });

    it("returns 'Assistant' for assistant role", () => {
      expect(getRoleDisplayName("assistant")).toBe("Assistant");
    });

    it("returns 'System' for system role", () => {
      expect(getRoleDisplayName("system")).toBe("System");
    });

    it("returns 'Tool' for tool role", () => {
      expect(getRoleDisplayName("tool")).toBe("Tool");
    });
  });

  describe("getRoleClassName", () => {
    it("returns correct class name for role", () => {
      expect(getRoleClassName("user")).toBe("message--user");
      expect(getRoleClassName("assistant")).toBe("message--assistant");
    });
  });

  describe("formatMessageTime", () => {
    it("returns empty string for undefined timestamp", () => {
      expect(formatMessageTime(undefined)).toBe("");
    });

    it("formats timestamp correctly", () => {
      const timestamp = new Date(2024, 0, 15, 14, 30).getTime();
      const result = formatMessageTime(timestamp);
      // Result should contain hour and minute
      expect(result).toMatch(/\d{1,2}:\d{2}/);
    });
  });

  describe("isMessageEmpty", () => {
    it("returns true for empty content", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "",
        isStreaming: false,
      };
      expect(isMessageEmpty(msg)).toBe(true);
    });

    it("returns true for whitespace-only content", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "   ",
        isStreaming: false,
      };
      expect(isMessageEmpty(msg)).toBe(true);
    });

    it("returns false for non-empty content", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "Hello",
        isStreaming: false,
      };
      expect(isMessageEmpty(msg)).toBe(false);
    });
  });

  describe("getMessagePreview", () => {
    it("returns full content if under limit", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "Hello world",
        isStreaming: false,
      };
      expect(getMessagePreview(msg, 100)).toBe("Hello world");
    });

    it("truncates long content", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "This is a very long message that should be truncated",
        isStreaming: false,
      };
      const preview = getMessagePreview(msg, 20);
      expect(preview.length).toBe(20);
      expect(preview.endsWith("...")).toBe(true);
    });

    it("returns empty string for undefined content", () => {
      const msg: NormalizedMessage = {
        id: "1",
        role: "user",
        content: "",
        isStreaming: false,
      };
      expect(getMessagePreview(msg)).toBe("");
    });
  });

  describe("filterMessagesByRole", () => {
    it("filters messages by role", () => {
      const messages: NormalizedMessage[] = [
        { id: "1", role: "user", content: "Hi", isStreaming: false },
        { id: "2", role: "assistant", content: "Hello", isStreaming: false },
        { id: "3", role: "user", content: "How are you?", isStreaming: false },
      ];

      const userMessages = filterMessagesByRole(messages, "user");
      expect(userMessages).toHaveLength(2);
      expect(userMessages.every((m) => m.role === "user")).toBe(true);
    });
  });

  describe("getLastMessage", () => {
    it("returns the last message", () => {
      const messages: NormalizedMessage[] = [
        { id: "1", role: "user", content: "First", isStreaming: false },
        { id: "2", role: "assistant", content: "Second", isStreaming: false },
        { id: "3", role: "user", content: "Third", isStreaming: false },
      ];

      const last = getLastMessage(messages);
      expect(last?.content).toBe("Third");
    });

    it("returns undefined for empty array", () => {
      expect(getLastMessage([])).toBeUndefined();
    });
  });

  describe("groupMessages", () => {
    it("groups consecutive messages by role", () => {
      const messages: NormalizedMessage[] = [
        { id: "1", role: "user", content: "Hi", isStreaming: false },
        { id: "2", role: "user", content: "How are you?", isStreaming: false },
        { id: "3", role: "assistant", content: "I'm good", isStreaming: false },
      ];

      const groups = groupMessages(messages);
      expect(groups).toHaveLength(2);
      expect(groups[0].messages).toHaveLength(2);
      expect(groups[0].role).toBe("user");
      expect(groups[1].messages).toHaveLength(1);
      expect(groups[1].role).toBe("assistant");
    });

    it("creates separate groups for different roles", () => {
      const messages: NormalizedMessage[] = [
        { id: "1", role: "user", content: "Hi", isStreaming: false },
        { id: "2", role: "assistant", content: "Hello", isStreaming: false },
        { id: "3", role: "user", content: "Thanks", isStreaming: false },
      ];

      const groups = groupMessages(messages);
      expect(groups).toHaveLength(3);
    });

    it("respects time gap configuration", () => {
      const now = Date.now();
      const messages: NormalizedMessage[] = [
        { id: "1", role: "user", content: "Hi", isStreaming: false, timestamp: now },
        { id: "2", role: "user", content: "Hello", isStreaming: false, timestamp: now + 120000 },
      ];

      const groups = groupMessages(messages, { maxTimeGap: 60000 });
      expect(groups).toHaveLength(2);
    });
  });
});
