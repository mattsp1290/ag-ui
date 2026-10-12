# Changelog

## 0.0.2 — 2026-10-12

- Initial functional release of the Cloudflare Agents integration.
- Adds WebSocket client with full AG-UI protocol compliance.
- Adds AI SDK v5 adapter supporting reasoning, streaming args, and error handling.
- Adds SSE and NDJSON response helpers with error propagation.
- Adds barrel exports and a Durable Object example Worker for the Dojo.
- Declares `protocolVersion` on `RUN_STARTED` events.
- Adds MIT LICENSE file and `license` field for publishing.
- Example Worker passes OpenAI settings via `CLOUDFLARE_INCLUDE_PROCESS_ENV` instead of `--var`.

### Breaking changes

- Requires AG-UI 1.0 peers; peer dependency on `@ag-ui/client` enforced.
