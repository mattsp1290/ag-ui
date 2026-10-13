# Changelog

## 0.0.7 — 2026-10-13

- `RUN_STARTED` now carries `protocolVersion`.
- Image, audio, video and document parts are sent as A2A file parts: inline data as bytes, URLs as URIs.
- Provider file handles have no A2A form and are dropped with a warning instead of being sent as raw data parts.
- `clone()` keeps the base agent's state and reuses the client.
- Added multimodality support.
- Added MIT LICENSE file and `license` field for publishing.

### Breaking changes

- Peer dependencies `@ag-ui/core` and `@ag-ui/client` now require `>=1.0.0`.
- Media part serialization changed: parts become A2A file parts rather than raw data parts.
- Provider file handles are now dropped with a warning instead of being sent.
