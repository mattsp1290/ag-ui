import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright configuration for Svelte example app E2E tests
 *
 * The webServer configuration automatically starts the example app dev server
 * before running tests. This works in both local development and CI.
 *
 * To run locally:
 *   pnpm test:e2e
 *
 * To run in CI:
 *   pnpm test:e2e
 *
 * Environment variables:
 * - SVELTE_EXAMPLE_URL: Override the base URL (defaults to http://localhost:5173)
 * - CI: When set, enables stricter test settings (retries, single worker)
 */
export default defineConfig({
  testDir: ".",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  use: {
    baseURL: process.env.SVELTE_EXAMPLE_URL || "http://localhost:5173",
    headless: true,
    viewport: { width: 1280, height: 720 },
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  reporter: process.env.CI ? [["github"], ["html"]] : [["list"]],
  // Auto-start dev server before tests (works in both local and CI)
  webServer: {
    command: "cd ../../examples/basic-chat && pnpm dev",
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    // In CI, stdout/stderr are captured for debugging
    stdout: process.env.CI ? "pipe" : "ignore",
    stderr: process.env.CI ? "pipe" : "ignore",
  },
});
