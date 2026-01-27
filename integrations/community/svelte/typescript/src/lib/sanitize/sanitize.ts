import type { SanitizeConfig, SanitizeResult, UrlSanitizeOptions } from "./types";

/**
 * Default configuration for content sanitization
 */
export const defaultSanitizeConfig: Required<SanitizeConfig> = {
  allowedTags: [],
  allowedAttributes: [],
  allowedProtocols: ["http", "https", "mailto"],
  maxLength: 0,
  truncationSuffix: "...",
  stripHtml: true,
};

/**
 * HTML entity map for escaping
 */
const HTML_ENTITIES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#x27;",
  "/": "&#x2F;",
};

/**
 * Regex for matching HTML tags
 */
const HTML_TAG_REGEX = /<\/?([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>/gi;

/**
 * Regex for matching HTML attributes
 */
const HTML_ATTR_REGEX = /\s([a-zA-Z][a-zA-Z0-9-]*)\s*=\s*["']([^"']*)["']/gi;

/**
 * Regex for matching potentially dangerous URL protocols
 */
const DANGEROUS_PROTOCOL_REGEX = /^(javascript|vbscript|data):/i;

/**
 * Escape HTML special characters
 */
export function escapeHtml(text: string): string {
  return text.replace(/[&<>"'\/]/g, (char) => HTML_ENTITIES[char] || char);
}

/**
 * Strip all HTML tags from content
 */
export function stripHtml(html: string): string {
  return html
    .replace(HTML_TAG_REGEX, "")
    .replace(/<!--[\s\S]*?-->/g, "") // Remove comments
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#x27;/gi, "'")
    .replace(/&#x2F;/gi, "/");
}

/**
 * Check if a URL is safe
 */
export function isSafeUrl(
  url: string,
  options: UrlSanitizeOptions = {}
): boolean {
  const { allowedProtocols = ["http", "https"], allowRelative = true } = options;

  // Check for dangerous protocols
  if (DANGEROUS_PROTOCOL_REGEX.test(url)) {
    return false;
  }

  // Check if URL is relative
  if (url.startsWith("/") || url.startsWith("./") || url.startsWith("../")) {
    return allowRelative;
  }

  // Check protocol
  try {
    const urlObj = new URL(url, "https://placeholder.example");
    const protocol = urlObj.protocol.replace(":", "").toLowerCase();
    return allowedProtocols.includes(protocol);
  } catch {
    // If URL parsing fails but it's not a dangerous protocol, allow if relative URLs are allowed
    return allowRelative && !url.includes(":");
  }
}

/**
 * Sanitize a URL string
 */
export function sanitizeUrl(
  url: string,
  options: UrlSanitizeOptions = {}
): string {
  if (isSafeUrl(url, options)) {
    return url;
  }
  return "";
}

/**
 * Sanitize content by removing potentially unsafe elements
 */
export function sanitizeContent(
  content: string,
  config: SanitizeConfig = {}
): SanitizeResult {
  const fullConfig = { ...defaultSanitizeConfig, ...config };
  let result = content;
  let wasModified = false;
  let wasTruncated = false;
  const originalLength = content.length;

  // Strip HTML if configured or no tags allowed
  if (fullConfig.stripHtml || fullConfig.allowedTags.length === 0) {
    const stripped = stripHtml(result);
    if (stripped !== result) {
      wasModified = true;
      result = stripped;
    }
  } else {
    // Filter to allowed tags only
    result = result.replace(HTML_TAG_REGEX, (match, tagName) => {
      if (fullConfig.allowedTags.includes(tagName.toLowerCase())) {
        // Remove disallowed attributes
        return match.replace(HTML_ATTR_REGEX, (attrMatch, attrName) => {
          if (fullConfig.allowedAttributes.includes(attrName.toLowerCase())) {
            return attrMatch;
          }
          wasModified = true;
          return "";
        });
      }
      wasModified = true;
      return "";
    });
  }

  // Apply length limit
  if (fullConfig.maxLength > 0 && result.length > fullConfig.maxLength) {
    result =
      result.substring(0, fullConfig.maxLength - fullConfig.truncationSuffix.length) +
      fullConfig.truncationSuffix;
    wasTruncated = true;
    wasModified = true;
  }

  return {
    content: result,
    wasModified,
    wasTruncated,
    originalLength,
  };
}

/**
 * Sanitize message content specifically
 * More permissive for messages - preserves newlines and basic formatting
 */
export function sanitizeMessageContent(
  content: string,
  maxLength = 0
): SanitizeResult {
  return sanitizeContent(content, {
    stripHtml: true,
    maxLength,
  });
}

/**
 * Sanitize tool output which may contain JSON or other structured data
 */
export function sanitizeToolOutput(output: string, maxLength = 0): SanitizeResult {
  // For tool output, we escape HTML but preserve structure
  const escaped = escapeHtml(output);
  const originalLength = output.length;
  let wasTruncated = false;
  let result = escaped;

  if (maxLength > 0 && result.length > maxLength) {
    result = result.substring(0, maxLength - 3) + "...";
    wasTruncated = true;
  }

  return {
    content: result,
    wasModified: escaped !== output || wasTruncated,
    wasTruncated,
    originalLength,
  };
}

/**
 * Check if content contains potentially unsafe HTML
 */
export function containsUnsafeHtml(content: string): boolean {
  // Check for script tags
  if (/<script\b/i.test(content)) return true;

  // Check for event handlers
  if (/\bon\w+\s*=/i.test(content)) return true;

  // Check for dangerous URLs in attributes
  if (/(?:href|src|action)\s*=\s*["']?\s*javascript:/i.test(content)) return true;

  // Check for data URIs that might execute code
  if (/(?:href|src)\s*=\s*["']?\s*data:/i.test(content)) return true;

  return false;
}
