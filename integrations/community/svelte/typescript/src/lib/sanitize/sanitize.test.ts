import { describe, it, expect } from "vitest";
import {
  escapeHtml,
  stripHtml,
  isSafeUrl,
  sanitizeUrl,
  sanitizeContent,
  sanitizeMessageContent,
  sanitizeToolOutput,
  containsUnsafeHtml,
} from "./sanitize";

describe("sanitization security tests", () => {
  describe("escapeHtml", () => {
    it("escapes HTML special characters", () => {
      expect(escapeHtml("<script>alert('xss')</script>")).toBe(
        "&lt;script&gt;alert(&#x27;xss&#x27;)&lt;&#x2F;script&gt;"
      );
    });

    it("escapes ampersands", () => {
      expect(escapeHtml("a & b")).toBe("a &amp; b");
    });

    it("escapes quotes", () => {
      expect(escapeHtml('"test"')).toBe("&quot;test&quot;");
    });

    it("handles empty strings", () => {
      expect(escapeHtml("")).toBe("");
    });

    it("preserves safe text", () => {
      expect(escapeHtml("Hello World")).toBe("Hello World");
    });
  });

  describe("stripHtml", () => {
    it("removes script tags and their content", () => {
      // DOMPurify removes script tags AND their content for security
      expect(stripHtml("<script>alert('xss')</script>")).toBe("");
    });

    it("removes all HTML tags", () => {
      expect(stripHtml("<div><p>Hello</p></div>")).toBe("Hello");
    });

    it("removes HTML comments", () => {
      expect(stripHtml("Hello <!-- comment --> World")).toBe("Hello  World");
    });

    it("decodes common HTML entities", () => {
      expect(stripHtml("&nbsp;&amp;&lt;&gt;")).toBe(" &<>");
    });

    it("handles nested tags", () => {
      expect(stripHtml("<div><span><b>text</b></span></div>")).toBe("text");
    });
  });

  describe("isSafeUrl", () => {
    it("allows http URLs", () => {
      expect(isSafeUrl("http://example.com")).toBe(true);
    });

    it("allows https URLs", () => {
      expect(isSafeUrl("https://example.com")).toBe(true);
    });

    it("handles mailto URLs based on configuration", () => {
      // mailto requires explicit protocol allowlist
      expect(isSafeUrl("mailto:test@example.com", { allowedProtocols: ["mailto"] })).toBe(true);
    });

    it("blocks javascript: URLs", () => {
      expect(isSafeUrl("javascript:alert('xss')")).toBe(false);
    });

    it("blocks javascript: URLs with encoding", () => {
      expect(isSafeUrl("JavaScript:alert('xss')")).toBe(false);
    });

    it("blocks vbscript: URLs", () => {
      expect(isSafeUrl("vbscript:msgbox('xss')")).toBe(false);
    });

    it("blocks data: URLs", () => {
      expect(isSafeUrl("data:text/html,<script>alert('xss')</script>")).toBe(false);
    });

    it("allows relative URLs when configured", () => {
      expect(isSafeUrl("/path/to/page", { allowRelative: true })).toBe(true);
      expect(isSafeUrl("./page.html", { allowRelative: true })).toBe(true);
      expect(isSafeUrl("../page.html", { allowRelative: true })).toBe(true);
    });

    it("blocks relative URLs when configured", () => {
      expect(isSafeUrl("/path/to/page", { allowRelative: false })).toBe(false);
    });

    it("allows only specified protocols", () => {
      expect(
        isSafeUrl("mailto:test@example.com", { allowedProtocols: ["https"] })
      ).toBe(false);
      expect(
        isSafeUrl("https://example.com", { allowedProtocols: ["https"] })
      ).toBe(true);
    });

    it("blocks URLs with leading/trailing whitespace (bypass prevention)", () => {
      expect(isSafeUrl("  javascript:alert('xss')")).toBe(false);
      expect(isSafeUrl("javascript:alert('xss')  ")).toBe(false);
      expect(isSafeUrl("\tjavascript:alert('xss')")).toBe(false);
      expect(isSafeUrl("\njavascript:alert('xss')")).toBe(false);
    });

    it("trims whitespace from valid URLs", () => {
      expect(isSafeUrl("  https://example.com  ")).toBe(true);
    });
  });

  describe("sanitizeUrl", () => {
    it("returns safe URLs unchanged", () => {
      expect(sanitizeUrl("https://example.com")).toBe("https://example.com");
    });

    it("returns empty string for unsafe URLs", () => {
      expect(sanitizeUrl("javascript:alert('xss')")).toBe("");
    });
  });

  describe("sanitizeContent", () => {
    it("strips HTML by default", () => {
      // DOMPurify removes script tags AND their content for security
      const result = sanitizeContent("<p>Hello <script>evil</script></p>");
      expect(result.content).toBe("Hello ");
      expect(result.wasModified).toBe(true);
    });

    it("truncates long content", () => {
      const longText = "a".repeat(100);
      const result = sanitizeContent(longText, { maxLength: 50 });
      expect(result.content.length).toBe(50);
      expect(result.content.endsWith("...")).toBe(true);
      expect(result.wasTruncated).toBe(true);
    });

    it("reports original length", () => {
      const result = sanitizeContent("Hello World");
      expect(result.originalLength).toBe(11);
    });

    it("reports unmodified content correctly", () => {
      const result = sanitizeContent("Hello World");
      expect(result.wasModified).toBe(false);
      expect(result.content).toBe("Hello World");
    });
  });

  describe("sanitizeMessageContent", () => {
    it("strips all HTML from messages", () => {
      // DOMPurify removes script tags AND their content for security
      const result = sanitizeMessageContent(
        "<b>Bold</b> and <script>alert('xss')</script>"
      );
      expect(result.content).toBe("Bold and ");
    });

    it("respects max length", () => {
      const result = sanitizeMessageContent("Short message", 10);
      expect(result.wasTruncated).toBe(true);
    });
  });

  describe("sanitizeToolOutput", () => {
    it("escapes HTML in tool output", () => {
      const result = sanitizeToolOutput("<error>Something went wrong</error>");
      expect(result.content).toBe(
        "&lt;error&gt;Something went wrong&lt;&#x2F;error&gt;"
      );
    });

    it("preserves JSON structure visually", () => {
      const json = '{"key": "value"}';
      const result = sanitizeToolOutput(json);
      expect(result.content).toBe('{&quot;key&quot;: &quot;value&quot;}');
    });

    it("truncates long output", () => {
      const longOutput = "x".repeat(100);
      const result = sanitizeToolOutput(longOutput, 50);
      expect(result.content.length).toBe(50);
      expect(result.wasTruncated).toBe(true);
    });
  });

  describe("containsUnsafeHtml", () => {
    it("detects script tags", () => {
      expect(containsUnsafeHtml("<script>alert('xss')</script>")).toBe(true);
    });

    it("detects inline event handlers", () => {
      expect(containsUnsafeHtml('<div onclick="alert()">Click</div>')).toBe(true);
      expect(containsUnsafeHtml("<img onerror='alert()'>")).toBe(true);
      expect(containsUnsafeHtml("<body onload='alert()'>")).toBe(true);
    });

    it("detects javascript: URLs in href", () => {
      expect(
        containsUnsafeHtml("<a href='javascript:alert()'>Click</a>")
      ).toBe(true);
    });

    it("detects javascript: URLs in src", () => {
      expect(containsUnsafeHtml("<img src='javascript:alert()'>")).toBe(true);
    });

    it("detects data: URIs", () => {
      expect(
        containsUnsafeHtml("<a href='data:text/html,<script>alert()</script>'>")
      ).toBe(true);
    });

    it("returns false for safe HTML", () => {
      expect(containsUnsafeHtml("<p>Hello <b>World</b></p>")).toBe(false);
      expect(containsUnsafeHtml("<a href='https://example.com'>Link</a>")).toBe(
        false
      );
    });

    it("returns false for plain text", () => {
      expect(containsUnsafeHtml("Just some text")).toBe(false);
    });
  });

  describe("XSS attack vectors", () => {
    // Vectors that containsUnsafeHtml is designed to detect
    const detectableVectors = [
      '<script>alert("XSS")</script>',
      "<img src=x onerror=alert('XSS')>",
      '<body onload=alert("XSS")>',
      '<a href="javascript:alert(1)">click</a>',
      "<input onfocus=alert(1) autofocus>",
      '<img src="x" onerror="alert(1)">',
      '<a href="data:text/html,test">test</a>',
    ];

    // Other vectors that sanitizeContent handles but containsUnsafeHtml may not detect
    const allXssVectors = [
      ...detectableVectors,
      "<svg/onload=alert('XSS')>",
      '<iframe src="javascript:alert(1)">',
      '<marquee onstart=alert(1)>',
      "<video><source onerror=alert(1)>",
      '<details open ontoggle=alert(1)>',
      "<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>",
      '"><script>alert(1)</script>',
      "'-alert(1)-'",
    ];

    it("containsUnsafeHtml detects common XSS vectors", () => {
      for (const vector of detectableVectors) {
        expect(containsUnsafeHtml(vector)).toBe(true);
      }
    });

    it("sanitizeContent removes XSS vectors", () => {
      for (const vector of allXssVectors) {
        const result = sanitizeContent(vector);
        expect(result.content).not.toContain("<script");
        expect(result.content).not.toContain("onerror=");
        expect(result.content).not.toContain("onclick=");
        expect(result.content).not.toContain("onload=");
        expect(result.content).not.toContain("javascript:");
      }
    });
  });
});
