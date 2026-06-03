/**
 * A2UI recovery agent (OSS-162) — DRAFT showcase, verify before wiring.
 *
 * Mirrors `a2ui_dynamic_schema` but enables the error-recovery loop: passes a
 * `catalog` (so the sub-agent's output is validated against component schemas)
 * and a `recovery` config to `getA2UITools`. When the sub-agent emits an invalid
 * A2UI surface (e.g. a HotelCard missing its required `rating`), the validation
 * errors are fed back and it regenerates, up to `maxAttempts`; the middleware
 * suppresses faulty attempts (no wipe) and surfaces an `a2ui_recovery` status.
 *
 * In the dojo demo the sub-agent's render_a2ui output is driven deterministically
 * by aimock (invalid → valid), so recovery is reproducible without a real LLM.
 *
 * ⚠️ For the gate to fire on SEMANTIC errors, the SAME `catalog` must also reach
 * `@ag-ui/a2ui-middleware` (its `schema` option). See INTEGRATION-CHECKLIST.
 */

import { createAgent } from "langchain";
import { copilotkitMiddleware } from "@copilotkit/sdk-js/langgraph";
import { ChatOpenAI } from "@langchain/openai";
import { getA2UITools } from "@ag-ui/langgraph";
import type { A2UIValidationCatalog } from "@ag-ui/a2ui-toolkit";

const CUSTOM_CATALOG_ID = "https://a2ui.org/demos/dojo/dynamic_catalog.json";

// Catalog (component name → required props) used to validate the sub-agent's
// output. Must match the dojo dynamic catalog (apps/dojo/src/a2ui-catalog) and
// the `schema` handed to A2UIMiddleware.
const RECOVERY_CATALOG: A2UIValidationCatalog = {
  components: {
    Row: { required: ["children"] },
    HotelCard: { required: ["name", "location", "rating", "pricePerNight", "action"] },
    ProductCard: { required: ["name", "price", "rating", "action"] },
    TeamMemberCard: { required: ["name", "role", "action"] },
  },
};

const COMPOSITION_GUIDE = `
## Available Pre-made Components

You have 4 components. Use Row as the root with structural children to repeat a card per item.

### Row
Layout container. Use structural children to repeat a card template:
  {"id":"root","component":"Row","children":{"componentId":"card","path":"/items"}}

### HotelCard
Props (ALL required unless noted): name, location, rating (number 0-5), pricePerNight, action; amenities (optional)

### ProductCard
Props: name, price, rating (number 0-5), action; description (optional), badge (optional)

### TeamMemberCard
Props: name, role, action; department (optional), email (optional), avatarUrl (optional)

## RULES
- Root is ALWAYS a Row with structural children: {"componentId":"<card-id>","path":"/items"}
- Inside templates, use RELATIVE paths (no leading slash): {"path":"name"} not {"path":"/name"}
- Always provide data in the "data" argument as {"items":[...]}
- Include EVERY required prop on each card.
- Generate 3-4 realistic items with diverse data.
`;

const a2uiTool = getA2UITools(new ChatOpenAI({ model: "gpt-4o" }), {
  defaultCatalogId: CUSTOM_CATALOG_ID,
  compositionGuide: COMPOSITION_GUIDE,
  // OSS-162: enable catalog-aware recovery.
  catalog: RECOVERY_CATALOG,
  recovery: { maxAttempts: 3 },
  onA2UIAttempt: (rec) => {
    // Dev observability: each attempt (incl. rejected ones) is logged.
    // eslint-disable-next-line no-console
    console.log(`[a2ui recovery] attempt ${rec.attempt}: ${rec.ok ? "valid" : "invalid"}`, rec.errors);
  },
});

export const a2uiRecoveryGraph = createAgent({
  model: "openai:gpt-4o",
  // Cast: tool typed against @ag-ui/langgraph's own @langchain/core peer.
  tools: [a2uiTool as any],
  middleware: [copilotkitMiddleware],
  systemPrompt: `You are a helpful assistant that creates rich visual UI on the fly.

When the user asks for visual content (hotel/product comparisons, team rosters, lists, cards, etc.),
use the generate_a2ui tool to create a dynamic A2UI surface.
IMPORTANT: After calling the tool, do NOT repeat the data in your text response. The tool renders UI automatically. Just confirm what was rendered.`,
});
