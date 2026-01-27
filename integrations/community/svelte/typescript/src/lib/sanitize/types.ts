/**
 * Configuration for content sanitization
 */
export interface SanitizeConfig {
  /** Allow these HTML tags (default: none) */
  allowedTags?: string[];
  /** Allow these attributes on any allowed tag */
  allowedAttributes?: string[];
  /** Allow URLs with these protocols (default: http, https) */
  allowedProtocols?: string[];
  /** Maximum content length (0 = no limit) */
  maxLength?: number;
  /** Replace content over maxLength with this */
  truncationSuffix?: string;
  /** Strip all HTML (overrides allowedTags) */
  stripHtml?: boolean;
}

/**
 * Result of content sanitization
 */
export interface SanitizeResult {
  /** The sanitized content */
  content: string;
  /** Whether content was modified during sanitization */
  wasModified: boolean;
  /** Whether content was truncated */
  wasTruncated: boolean;
  /** Original length before truncation */
  originalLength: number;
}

/**
 * Options for URL sanitization
 */
export interface UrlSanitizeOptions {
  /** Allowed URL protocols */
  allowedProtocols?: string[];
  /** Whether to allow relative URLs */
  allowRelative?: boolean;
}
