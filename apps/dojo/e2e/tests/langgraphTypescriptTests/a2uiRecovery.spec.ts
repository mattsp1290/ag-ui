import { test, expect } from "../../test-isolation-helper";
import { A2UIPage } from "../../featurePages/A2UIPage";

// OSS-162 A2UI error-recovery showcase. The aimock fixtures
// (apps/dojo/e2e/a2ui-recovery-fixtures.ts) drive the sub-agent's render_a2ui:
// the first attempt is a Row whose repeated child references a `card` template
// the model "forgot" to include (structural "unresolved child"); the loop feeds
// the error back and the second attempt is valid.

test("[LangGraph TS] A2UI recovery — invalid render recovers to a valid surface", async ({
  page,
}) => {
  await page.goto("/langgraph-typescript/feature/a2ui_recovery");

  const a2ui = new A2UIPage(page);
  await a2ui.openChat();
  await a2ui.sendMessage("Compare 3 luxury hotels with ratings and prices.");

  // The faulty first attempt is suppressed (no wipe); the regenerated valid
  // surface paints.
  await a2ui.assertSurfaceWithIdVisible("hotel-comparison");
  await a2ui.assertSurfaceContainsAll(["The Ritz", "Holiday Inn", "Boutique Loft"]);
});

test("[LangGraph TS] A2UI recovery — exhaustion never paints a faulty surface, chat stays usable", async ({
  page,
}) => {
  await page.goto("/langgraph-typescript/feature/a2ui_recovery");

  const a2ui = new A2UIPage(page);
  await a2ui.openChat();
  await a2ui.sendMessage("Compare 3 broken hotels with ratings and prices.");

  // Every attempt is invalid → no faulty surface ever paints. The no-wipe invariant
  // holds even under total exhaustion. This is the server-side guarantee (middleware
  // gate + adapter loop) and is independent of the client renderer.
  await expect(a2ui.surface("hotel-comparison")).toHaveCount(0);

  // Conversation remains usable after the hard failure.
  await a2ui.sendMessage("Thanks anyway.");
});

// TEMPORARY GATE (OSS-162): the tasteful hard-failure MESSAGE is rendered by
// createA2UIRecoveryRenderer in @copilotkit/react-core. Until that ships in a published
// release, the dojo runs the published renderer (which lacks it), so this assertion can't
// pass here. It runs only when the dojo is linked against a local CopilotKit build that
// includes the renderer (set A2UI_LOCAL_RENDERER=1).
// REMOVE this skip once @copilotkit/react-core publishes the recovery renderer.
test("[LangGraph TS] A2UI recovery — exhaustion shows the hard-failure UI (needs local @copilotkit renderer)", async ({
  page,
}) => {
  test.skip(
    !process.env.A2UI_LOCAL_RENDERER,
    "requires the local @copilotkit recovery renderer; set A2UI_LOCAL_RENDERER=1 when the dojo is linked against a local CopilotKit build",
  );

  await page.goto("/langgraph-typescript/feature/a2ui_recovery");

  const a2ui = new A2UIPage(page);
  await a2ui.openChat();
  await a2ui.sendMessage("Compare 3 broken hotels with ratings and prices.");

  // No faulty surface ever paints...
  await expect(a2ui.surface("hotel-comparison")).toHaveCount(0);
  // ...and the tasteful hard-failure message is shown.
  await expect(page.getByText(/Couldn't generate|went wrong/i)).toBeVisible({ timeout: 30_000 });

  // Conversation remains usable after the hard failure.
  await a2ui.sendMessage("Thanks anyway.");
});
