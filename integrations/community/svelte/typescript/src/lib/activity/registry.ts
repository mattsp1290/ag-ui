import type { NormalizedActivity } from "../events/types";
import type {
  ActivityRenderer,
  ActivityRendererRegistration,
  ActivityRegistryOptions,
  RendererLookupResult,
} from "./types";

/**
 * Default fallback component that renders activity as JSON
 * This is a placeholder that returns null - consumers should provide their own fallback
 */
const defaultFallbackComponent: ActivityRenderer = null as any;

/**
 * Registry for mapping activity types to Svelte components
 */
export class ActivityRegistry {
  private renderers: Map<string, ActivityRendererRegistration[]> = new Map();
  private fallbackComponent: ActivityRenderer;
  private warnOnMissing: boolean;

  constructor(options: ActivityRegistryOptions = {}) {
    this.fallbackComponent = options.fallbackComponent ?? defaultFallbackComponent;
    this.warnOnMissing = options.warnOnMissing ?? true;
  }

  /**
   * Register a renderer for an activity type
   */
  register<T = unknown>(registration: ActivityRendererRegistration<T>): () => void {
    const existing = this.renderers.get(registration.type) ?? [];
    const newRegistration = { ...registration, priority: registration.priority ?? 0 };

    // Insert in priority order (higher priority first)
    const index = existing.findIndex((r) => (r.priority ?? 0) < newRegistration.priority);
    if (index === -1) {
      existing.push(newRegistration as ActivityRendererRegistration);
    } else {
      existing.splice(index, 0, newRegistration as ActivityRendererRegistration);
    }

    this.renderers.set(registration.type, existing);

    // Return unregister function
    return () => this.unregister(registration.type, registration.component);
  }

  /**
   * Register multiple renderers at once
   */
  registerAll(registrations: ActivityRendererRegistration[]): () => void {
    const unregisterFns = registrations.map((r) => this.register(r));
    return () => unregisterFns.forEach((fn) => fn());
  }

  /**
   * Unregister a renderer
   */
  unregister(type: string, component: ActivityRenderer): boolean {
    const existing = this.renderers.get(type);
    if (!existing) return false;

    const index = existing.findIndex((r) => r.component === component);
    if (index === -1) return false;

    existing.splice(index, 1);
    if (existing.length === 0) {
      this.renderers.delete(type);
    }

    return true;
  }

  /**
   * Get a renderer for an activity
   */
  getRenderer<T = unknown>(activity: NormalizedActivity): RendererLookupResult<T> {
    const registrations = this.renderers.get(activity.type);

    if (registrations && registrations.length > 0) {
      // Find first renderer whose validator passes (or has no validator)
      for (const registration of registrations) {
        if (!registration.validate || registration.validate(activity.content)) {
          return {
            component: registration.component as ActivityRenderer<T>,
            isFallback: false,
            registration: registration as ActivityRendererRegistration<T>,
          };
        }
      }
    }

    if (this.warnOnMissing && this.fallbackComponent) {
      console.warn(
        `[ActivityRegistry] No renderer found for activity type "${activity.type}"`
      );
    }

    return {
      component: this.fallbackComponent as ActivityRenderer<T>,
      isFallback: true,
    };
  }

  /**
   * Check if a renderer exists for an activity type
   */
  hasRenderer(type: string): boolean {
    return this.renderers.has(type) && (this.renderers.get(type)?.length ?? 0) > 0;
  }

  /**
   * Get all registered activity types
   */
  getRegisteredTypes(): string[] {
    return Array.from(this.renderers.keys());
  }

  /**
   * Set the fallback component
   */
  setFallback(component: ActivityRenderer): void {
    this.fallbackComponent = component;
  }

  /**
   * Clear all registrations
   */
  clear(): void {
    this.renderers.clear();
  }
}

/**
 * Create a new activity registry
 */
export function createActivityRegistry(
  options: ActivityRegistryOptions = {}
): ActivityRegistry {
  return new ActivityRegistry(options);
}

/**
 * Default global activity registry instance
 */
export const defaultActivityRegistry = createActivityRegistry();

/**
 * Register a renderer on the default registry
 */
export function registerActivityRenderer<T = unknown>(
  registration: ActivityRendererRegistration<T>
): () => void {
  return defaultActivityRegistry.register(registration);
}

/**
 * Get a renderer from the default registry
 */
export function getActivityRenderer<T = unknown>(
  activity: NormalizedActivity
): RendererLookupResult<T> {
  return defaultActivityRegistry.getRenderer(activity);
}
