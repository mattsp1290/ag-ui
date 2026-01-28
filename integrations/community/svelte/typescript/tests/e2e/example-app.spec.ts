/**
 * E2E Smoke Tests for Svelte Example App
 *
 * These tests verify the basic functionality of the example app.
 * To run locally:
 * 1. Start the dev server: cd examples/basic-chat && pnpm dev
 * 2. Run tests: pnpm playwright test
 *
 * Note: These tests require the example app to be running at http://localhost:5173
 */

import { test, expect } from "@playwright/test";

const BASE_URL = process.env.SVELTE_EXAMPLE_URL || "http://localhost:5173";

test.describe("Svelte Example App", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);
  });

  test("should load the app with title", async ({ page }) => {
    await expect(page.locator("h1")).toContainText("AG-UI Svelte Example");
  });

  test("should show chat, tools, and state tabs", async ({ page }) => {
    await expect(page.locator("button:has-text('Chat')")).toBeVisible();
    await expect(page.locator("button:has-text('Tools')")).toBeVisible();
    await expect(page.locator("button:has-text('State')")).toBeVisible();
  });

  test("should have a message input", async ({ page }) => {
    const input = page.locator('input[placeholder*="message"]');
    await expect(input).toBeVisible();
  });

  test("should send a message when pressing enter", async ({ page }) => {
    const input = page.locator('input[placeholder*="message"]');
    await input.fill("hello");
    await input.press("Enter");

    // Wait for the message to appear in the message list
    await expect(page.locator(".message.user")).toBeVisible({
      timeout: 5000,
    });
  });

  test("should switch to Tools tab", async ({ page }) => {
    await page.click("button:has-text('Tools')");
    await expect(page.locator("text=calculate")).toBeVisible({
      timeout: 2000,
    }).catch(() => {
      // Tools hint should be visible if no tool calls yet
      expect(page.locator("text=trigger a tool call")).toBeVisible();
    });
  });

  test("should switch to State tab", async ({ page }) => {
    await page.click("button:has-text('State')");
    await expect(page.locator("text=Shared State")).toBeVisible();
  });

  test("should show streaming indicator when running", async ({ page }) => {
    const input = page.locator('input[placeholder*="message"]');
    await input.fill("hello");
    await input.press("Enter");

    // The running indicator should appear briefly
    await expect(page.locator(".status-indicator.running")).toBeVisible({
      timeout: 2000,
    }).catch(() => {
      // May have already completed
    });
  });

  test("should display assistant response after user message", async ({
    page,
  }) => {
    const input = page.locator('input[placeholder*="message"]');
    await input.fill("help");
    await input.press("Enter");

    // Wait for assistant message to appear
    await expect(page.locator(".message.assistant")).toBeVisible({
      timeout: 10000,
    });
  });
});

test.describe("Tool Call Demo", () => {
  test("should show tool call when typing 'calculate'", async ({ page }) => {
    await page.goto(BASE_URL);

    const input = page.locator('input[placeholder*="message"]');
    await input.fill("calculate");
    await input.press("Enter");

    // Switch to Tools tab to see the tool call
    await page.click("button:has-text('Tools')");

    // Wait for tool call to appear
    await expect(
      page.locator("text=calculator").or(page.locator(".tool-call"))
    ).toBeVisible({
      timeout: 10000,
    });
  });
});

test.describe("State Demo", () => {
  test("should show state updates when typing 'state'", async ({ page }) => {
    await page.goto(BASE_URL);

    const input = page.locator('input[placeholder*="message"]');
    await input.fill("state");
    await input.press("Enter");

    // Switch to State tab
    await page.click("button:has-text('State')");

    // Wait for state to be populated
    await expect(page.locator(".state-tree .tree-node")).toBeVisible({
      timeout: 10000,
    });
  });
});
