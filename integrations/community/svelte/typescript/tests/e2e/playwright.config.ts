import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright configuration for Svelte example app E2E tests
 *
 * To run:
 * 1. Start the example app: cd examples/basic-chat && pnpm dev
 * 2. Run tests: pnpm playwright test
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
  // Run local dev server before tests if not in CI
  webServer: process.env.CI
    ? undefined
    : {
        command: "cd ../../examples/basic-chat && pnpm dev",
        url: "http://localhost:5173",
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
});
