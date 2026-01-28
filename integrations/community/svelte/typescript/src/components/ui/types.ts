/**
 * Common props for UI components
 */
export interface BaseComponentProps {
  /** Additional CSS class names */
  class?: string;
}

/**
 * Props for ErrorBanner component
 */
export interface ErrorBannerProps extends BaseComponentProps {
  /** The error to display */
  error: Error | null;
  /** Whether the error can be dismissed */
  dismissible?: boolean;
  /** Callback when error is dismissed */
  onDismiss?: () => void;
}

/**
 * Props for Loading component
 */
export interface LoadingProps extends BaseComponentProps {
  /** Loading message to display */
  message?: string;
  /** Size of the loading indicator */
  size?: "small" | "medium" | "large";
}

/**
 * Props for EmptyState component
 */
export interface EmptyStateProps extends BaseComponentProps {
  /** Title for empty state */
  title?: string;
  /** Description for empty state */
  description?: string;
}
