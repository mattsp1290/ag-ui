# Changelog

## 0.0.3 — 2026-10-14

- Streamed tool calls are now retained on the assistant message in `MESSAGES_SNAPSHOT`, so frontend tools can run instead of being erased.
- Added support for an explicit `baseUrl` to target private endpoints or OpenAI-compatible mocks.
- Prompts are now built from text parts only via `contentToText`; media parts are dropped with a warning instead of being JSON-serialized.
- `RUN_STARTED` now carries `protocolVersion: PROTOCOL_VERSION`.

### Breaking changes

- Peer dependency floors for `@ag-ui/core` and `@ag-ui/client` raised to `>=1.0.0`.
- `baseUrl` now overrides the URL previously derived from `region` and `instanceId`; re-verify endpoint configuration.
- Media content parts are no longer serialized into the prompt and are dropped.

## 0.0.2

- Publish the declared MIT license and license file in the npm package.
