/**
 * aimock fixtures for the A2UI recovery showcase (OSS-162) — DRAFT, verify before wiring.
 *
 * Deterministically drives the sub-agent's `render_a2ui` output so recovery is
 * reproducible without a real LLM:
 *   - "compare hotels" demo  → invalid HotelCard (missing required `rating`) on
 *      the FIRST attempt, then a VALID surface once the validation errors are fed
 *      back (recovery succeeds → no wipe, brief "Retrying…", final surface).
 *   - "broken hotels" demo   → ALWAYS invalid → recovery exhausts → hard-failure.
 *
 * Wire by calling `registerA2UIRecoveryFixtures(mockServer)` from aimock-setup.ts
 * BEFORE the generic fixture loader (predicate fixtures must come first).
 */
import type { LLMock, ChatMessage } from "@copilotkit/aimock";

const CATALOG_ID = "https://a2ui.org/demos/dojo/dynamic_catalog.json";

const textOf = (content: ChatMessage["content"] | undefined): string => {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content.filter((p) => p.type === "text" && typeof p.text === "string").map((p) => p.text!).join("");
  }
  return "";
};

const allText = (messages: ChatMessage[] = []): string => messages.map((m) => textOf(m.content)).join("\n");
const userText = (messages: ChatMessage[] = []): string =>
  textOf(messages.filter((m) => m.role === "user").pop()?.content);

// Marker the toolkit appends to the sub-agent prompt on retry
// (augmentPromptWithValidationErrors). Presence ⇒ this is a retry.
const RETRY_MARKER = "Previous attempt was invalid";

const ROOT = { id: "root", component: "Row", children: { componentId: "card", path: "/items" }, gap: 16 };
const hotelCard = (withRating: boolean) => ({
  id: "card",
  component: "HotelCard",
  name: { path: "name" },
  location: { path: "location" },
  ...(withRating ? { rating: { path: "rating" } } : {}), // omit → invalid (missing required `rating`)
  pricePerNight: { path: "price" },
  action: { event: { name: "book_hotel", context: { hotelName: { path: "name" } } } },
});
const HOTELS = [
  { name: "The Ritz", location: "Paris", rating: 4.8, price: "$450/night" },
  { name: "Holiday Inn", location: "Austin", rating: 4.1, price: "$180/night" },
  { name: "Boutique Loft", location: "Lisbon", rating: 4.6, price: "$320/night" },
];
const renderArgs = (withRating: boolean) =>
  JSON.stringify({ surfaceId: "hotel-comparison", components: [ROOT, hotelCard(withRating)], data: { items: HOTELS } });

export function registerA2UIRecoveryFixtures(mockServer: LLMock): void {
  const hasTool = (req: any, name: string) => req.tools?.some((t: any) => t.function.name === name);

  // 1) Main agent: any hotel/recovery prompt → call the generate_a2ui sub-agent tool.
  mockServer.addFixture({
    match: {
      predicate: (req: any) =>
        hasTool(req, "generate_a2ui") && /hotel/i.test(userText(req.messages)),
    },
    response: { toolCalls: [{ name: "generate_a2ui", arguments: JSON.stringify({ intent: "create" }) }] },
  });

  // 2) Sub-agent — EXHAUSTION demo ("broken hotels"): always invalid.
  mockServer.addFixture({
    match: {
      predicate: (req: any) =>
        hasTool(req, "render_a2ui") && /broken/i.test(allText(req.messages)),
    },
    response: { toolCalls: [{ name: "render_a2ui", arguments: renderArgs(false) }] },
  });

  // 3) Sub-agent — RECOVERY demo, RETRY (errors fed back) → valid. Must be
  //    registered before the first-attempt fixture so it matches first.
  mockServer.addFixture({
    match: {
      predicate: (req: any) =>
        hasTool(req, "render_a2ui") && allText(req.messages).includes(RETRY_MARKER),
    },
    response: { toolCalls: [{ name: "render_a2ui", arguments: renderArgs(true) }] },
  });

  // 4) Sub-agent — RECOVERY demo, FIRST attempt (no marker yet) → invalid.
  mockServer.addFixture({
    match: {
      predicate: (req: any) =>
        hasTool(req, "render_a2ui") && !allText(req.messages).includes(RETRY_MARKER),
    },
    response: { toolCalls: [{ name: "render_a2ui", arguments: renderArgs(false) }] },
  });
}
