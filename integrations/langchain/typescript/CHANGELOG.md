# Changelog

## 0.0.4 — 2026-10-12

- Detect LangChain messages by `_getType()`/`type` tag instead of `constructor.name`, so minified/production bundles no longer drop streamed tool calls.
- `RUN_STARTED` now carries `protocolVersion: PROTOCOL_VERSION`.
- Tool result content parts are flattened to text via `contentToText()`, with a warning when media is dropped.
- User message URL/inline image parts are mapped to LangChain `image_url` blocks; unmappable parts (audio, video, document, file) emit a warning.
- Migrated to `@langchain/core` 1.x; devDependency widened to `>=1.1.0 <2.0.0` (peer range stays `>=0.3.0 <2.0.0`).

### Breaking changes

- Requires `@langchain/core` 1.x at runtime; versions 1.0.0-1.0.6 do not type-check.
- Raised `@ag-ui/core` and `@ag-ui/client` peer dependency requirements; re-verify installed versions.
- Tool result media and unmappable message content parts are now dropped (with warnings) rather than passed through.
