import DOMPurify, { type Config as DOMPurifyConfig } from "dompurify";
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
 * Strip all HTML tags from content using DOMPurify
 */
export function stripHtml(html: string): string {
  // Use DOMPurify with no allowed tags to strip all HTML
  const stripped = DOMPurify.sanitize(html, { ALLOWED_TAGS: [], ALLOWED_ATTR: [] });
  // Decode common HTML entities
  return stripped
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

  // Trim whitespace and remove control characters to prevent bypass attacks
  const normalizedUrl = url.trim().replace(/[\x00-\x1f\x7f]/g, "");

  // Check for dangerous protocols
  if (DANGEROUS_PROTOCOL_REGEX.test(normalizedUrl)) {
    return false;
  }

  // Check if URL is relative
  if (normalizedUrl.startsWith("/") || normalizedUrl.startsWith("./") || normalizedUrl.startsWith("../")) {
    return allowRelative;
  }

  // Check protocol
  try {
    const urlObj = new URL(normalizedUrl, "https://placeholder.example");
    const protocol = urlObj.protocol.replace(":", "").toLowerCase();
    return allowedProtocols.includes(protocol);
  } catch {
    // If URL parsing fails but it's not a dangerous protocol, allow if relative URLs are allowed
    return allowRelative && !normalizedUrl.includes(":");
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
    return url.trim();
  }
  return "";
}

/**
 * Sanitize content using DOMPurify for robust XSS protection
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

  // Configure DOMPurify options
  const purifyConfig: DOMPurifyConfig = {
    RETURN_DOM: false,
    RETURN_DOM_FRAGMENT: false,
  };

  if (fullConfig.stripHtml || fullConfig.allowedTags.length === 0) {
    // Strip all HTML
    purifyConfig.ALLOWED_TAGS = [];
    purifyConfig.ALLOWED_ATTR = [];
  } else {
    // Allow only specified tags and attributes
    purifyConfig.ALLOWED_TAGS = fullConfig.allowedTags;
    purifyConfig.ALLOWED_ATTR = fullConfig.allowedAttributes;
  }

  // Add allowed protocols
  purifyConfig.ALLOWED_URI_REGEXP = new RegExp(
    `^(?:(?:${fullConfig.allowedProtocols.join("|")}):|[^a-z]|[a-z+.\\-]+(?:[^a-z+.\\-:]|$))`,
    "i"
  );

  // Sanitize using DOMPurify (cast to string to handle TrustedHTML)
  const sanitized = DOMPurify.sanitize(content, purifyConfig) as string;

  // Check if content was modified
  if (sanitized !== content) {
    wasModified = true;
    result = sanitized;
  }

  // Decode HTML entities if we stripped HTML
  if (fullConfig.stripHtml || fullConfig.allowedTags.length === 0) {
    const decoded = result
      .replace(/&nbsp;/gi, " ")
      .replace(/&amp;/gi, "&")
      .replace(/&lt;/gi, "<")
      .replace(/&gt;/gi, ">")
      .replace(/&quot;/gi, '"')
      .replace(/&#x27;/gi, "'")
      .replace(/&#x2F;/gi, "/");
    if (decoded !== result) {
      wasModified = true;
      result = decoded;
    }
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
 * Uses DOMPurify's sanitization to detect if content would be modified
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
