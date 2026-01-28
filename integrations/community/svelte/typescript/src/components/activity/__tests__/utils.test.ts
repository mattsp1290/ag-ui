import { describe, it, expect } from "vitest";
import type { NormalizedActivity } from "../../../lib/events/types";
import {
  getActivityStatus,
  getActivityStatusClass,
  extractActivityMetadata,
  formatActivityType,
  getActivityTitle,
  hasActivityContentChanged,
  getActivityPreview,
  hasVisualContent,
  getActivityIcon,
} from "../utils";

describe("activity utils", () => {
  const createActivity = (
    type: string,
    content: Record<string, unknown> = {}
  ): NormalizedActivity => ({
    id: "test-1",
    type,
    content,
    messageId: "msg-1",
  });

  describe("getActivityStatus", () => {
    it("returns 'updating' when isUpdating is true", () => {
      const activity = createActivity("test");
      expect(getActivityStatus(activity, true)).toBe("updating");
    });

    it("returns 'complete' by default", () => {
      const activity = createActivity("test");
      expect(getActivityStatus(activity, false)).toBe("complete");
    });
  });

  describe("getActivityStatusClass", () => {
    it("returns correct class for status", () => {
      expect(getActivityStatusClass("pending")).toBe("activity--pending");
      expect(getActivityStatusClass("updating")).toBe("activity--updating");
      expect(getActivityStatusClass("complete")).toBe("activity--complete");
      expect(getActivityStatusClass("error")).toBe("activity--error");
    });
  });

  describe("extractActivityMetadata", () => {
    it("extracts metadata from content", () => {
      const activity = createActivity("test", {
        title: "Test Activity",
        description: "A test activity",
        icon: "test-icon",
        expandable: true,
        defaultExpanded: false,
      });

      const metadata = extractActivityMetadata(activity);
      expect(metadata.title).toBe("Test Activity");
      expect(metadata.description).toBe("A test activity");
      expect(metadata.icon).toBe("test-icon");
      expect(metadata.expandable).toBe(true);
      expect(metadata.defaultExpanded).toBe(false);
    });

    it("returns undefined for missing fields", () => {
      const activity = createActivity("test", {});
      const metadata = extractActivityMetadata(activity);
      expect(metadata.title).toBeUndefined();
      expect(metadata.description).toBeUndefined();
    });
  });

  describe("formatActivityType", () => {
    it("formats snake_case to Title Case", () => {
      expect(formatActivityType("weather_forecast")).toBe("Weather Forecast");
    });

    it("formats kebab-case to Title Case", () => {
      expect(formatActivityType("code-review")).toBe("Code Review");
    });

    it("handles single word", () => {
      expect(formatActivityType("chart")).toBe("Chart");
    });
  });

  describe("getActivityTitle", () => {
    it("uses title from metadata if available", () => {
      const activity = createActivity("test", { title: "Custom Title" });
      expect(getActivityTitle(activity)).toBe("Custom Title");
    });

    it("falls back to formatted type", () => {
      const activity = createActivity("weather_forecast");
      expect(getActivityTitle(activity)).toBe("Weather Forecast");
    });
  });

  describe("hasActivityContentChanged", () => {
    it("returns true when prev is undefined", () => {
      const activity = createActivity("test");
      expect(hasActivityContentChanged(undefined, activity)).toBe(true);
    });

    it("returns true when ids differ", () => {
      const prev = createActivity("test");
      const next = { ...createActivity("test"), id: "different" };
      expect(hasActivityContentChanged(prev, next)).toBe(true);
    });

    it("returns true when content differs", () => {
      const prev = createActivity("test", { value: 1 });
      const next = createActivity("test", { value: 2 });
      expect(hasActivityContentChanged(prev, next)).toBe(true);
    });

    it("returns false when content is the same", () => {
      const prev = createActivity("test", { value: 1 });
      const next = createActivity("test", { value: 1 });
      expect(hasActivityContentChanged(prev, next)).toBe(false);
    });
  });

  describe("getActivityPreview", () => {
    it("returns string content directly", () => {
      const activity: NormalizedActivity = {
        id: "1",
        type: "test",
        content: "Simple string",
        messageId: "msg-1",
      };
      expect(getActivityPreview(activity)).toBe("Simple string");
    });

    it("truncates long content", () => {
      const activity: NormalizedActivity = {
        id: "1",
        type: "test",
        content: "This is a very long string that should be truncated",
        messageId: "msg-1",
      };
      const preview = getActivityPreview(activity, 20);
      expect(preview.length).toBe(20);
      expect(preview.endsWith("...")).toBe(true);
    });

    it("serializes object content", () => {
      const activity = createActivity("test", { key: "value" });
      const preview = getActivityPreview(activity);
      expect(preview).toBe('{"key":"value"}');
    });
  });

  describe("hasVisualContent", () => {
    it("returns true for imageUrl in content", () => {
      const activity = createActivity("test", { imageUrl: "http://example.com/img.png" });
      expect(hasVisualContent(activity)).toBe(true);
    });

    it("returns true for chartData in content", () => {
      const activity = createActivity("test", { chartData: [] });
      expect(hasVisualContent(activity)).toBe(true);
    });

    it("returns true for chart type", () => {
      const activity = createActivity("line_chart");
      expect(hasVisualContent(activity)).toBe(true);
    });

    it("returns true for image type", () => {
      const activity = createActivity("generated_image");
      expect(hasVisualContent(activity)).toBe(true);
    });

    it("returns false for text-only activity", () => {
      const activity = createActivity("text_update", { text: "Hello" });
      expect(hasVisualContent(activity)).toBe(false);
    });
  });

  describe("getActivityIcon", () => {
    it("returns 'chart' for chart types", () => {
      expect(getActivityIcon("line_chart")).toBe("chart");
      expect(getActivityIcon("bar_graph")).toBe("chart");
    });

    it("returns 'image' for image types", () => {
      expect(getActivityIcon("generated_image")).toBe("image");
    });

    it("returns 'table' for table types", () => {
      expect(getActivityIcon("data_table")).toBe("table");
    });

    it("returns 'code' for code types", () => {
      expect(getActivityIcon("code_snippet")).toBe("code");
    });

    it("returns 'activity' as default", () => {
      expect(getActivityIcon("unknown_type")).toBe("activity");
    });
  });
});
