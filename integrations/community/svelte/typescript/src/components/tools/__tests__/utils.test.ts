import { describe, it, expect } from "vitest";
import type { NormalizedToolCall } from "../../../lib/events/types";
import {
  getToolStatus,
  formatToolArguments,
  isToolCallActive,
  getToolArgumentsSummary,
  parseToolResult,
  formatToolResult,
} from "../utils";

describe("tool utils", () => {
  const createToolCall = (
    overrides: Partial<NormalizedToolCall> = {}
  ): NormalizedToolCall => ({
    id: "tc-1",
    name: "test_tool",
    arguments: '{"key": "value"}',
    status: "completed",
    ...overrides,
  });

  describe("getToolStatus", () => {
    it("returns config for pending status", () => {
      const toolCall = createToolCall({ status: "pending" });
      const status = getToolStatus(toolCall);
      expect(status.label).toBe("Pending");
    });

    it("returns config for streaming status", () => {
      const toolCall = createToolCall({ status: "streaming" });
      const status = getToolStatus(toolCall);
      expect(status.label).toBe("Running");
    });

    it("returns config for completed status", () => {
      const toolCall = createToolCall({ status: "completed" });
      const status = getToolStatus(toolCall);
      expect(status.label).toBe("Completed");
    });
  });

  describe("formatToolArguments", () => {
    it("formats JSON arguments with indentation", () => {
      const toolCall = createToolCall({ arguments: '{"name":"test"}' });
      const formatted = formatToolArguments(toolCall, 2);
      expect(formatted).toContain("\n");
      expect(formatted).toContain('"name"');
    });

    it("uses parsedArguments if available", () => {
      const toolCall = createToolCall({
        arguments: '{"name":"test"}',
        parsedArguments: { name: "test", extra: "value" },
      });
      const formatted = formatToolArguments(toolCall, 2);
      expect(formatted).toContain("extra");
    });

    it("returns raw arguments if parsing fails", () => {
      const toolCall = createToolCall({ arguments: "not valid json" });
      const formatted = formatToolArguments(toolCall);
      expect(formatted).toBe("not valid json");
    });
  });

  describe("isToolCallActive", () => {
    it("returns true for pending status", () => {
      const toolCall = createToolCall({ status: "pending" });
      expect(isToolCallActive(toolCall)).toBe(true);
    });

    it("returns true for streaming status", () => {
      const toolCall = createToolCall({ status: "streaming" });
      expect(isToolCallActive(toolCall)).toBe(true);
    });

    it("returns false for completed status", () => {
      const toolCall = createToolCall({ status: "completed" });
      expect(isToolCallActive(toolCall)).toBe(false);
    });
  });

  describe("getToolArgumentsSummary", () => {
    it("returns full arguments if under limit", () => {
      const toolCall = createToolCall({ arguments: '{"a":1}' });
      const summary = getToolArgumentsSummary(toolCall, 50);
      expect(summary).toBe('{"a":1}');
    });

    it("truncates long arguments", () => {
      const toolCall = createToolCall({
        arguments: '{"name":"very long value that exceeds the limit"}',
      });
      const summary = getToolArgumentsSummary(toolCall, 20);
      expect(summary.length).toBe(20);
      expect(summary.endsWith("...")).toBe(true);
    });
  });

  describe("parseToolResult", () => {
    it("parses JSON result", () => {
      const result = parseToolResult('{"status":"ok"}');
      expect(result).toEqual({ status: "ok" });
    });

    it("returns string for non-JSON result", () => {
      const result = parseToolResult("Plain text result");
      expect(result).toBe("Plain text result");
    });
  });

  describe("formatToolResult", () => {
    it("formats JSON result with indentation", () => {
      const formatted = formatToolResult('{"status":"ok"}', 2);
      expect(formatted).toContain("\n");
      expect(formatted).toContain('"status"');
    });

    it("returns raw string for non-JSON", () => {
      const formatted = formatToolResult("Plain text");
      expect(formatted).toBe("Plain text");
    });
  });
});
