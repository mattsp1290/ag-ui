# Changelog

## 0.2.0 — 2026-10-12

- Now requires `ag-ui-protocol>=1.0.0`.
- Fixed parsing of protocol 1.0 input: `_extract_user_message` now reads TextPart models, dicts and strings; previously array-content messages became the literal prompt "Hello". Warns on dropped media.
- Declare `protocolVersion` on `RUN_STARTED`.
- Endpoint helpers now forward FastAPI route kwargs (name, tags, summary, operation_id, dependencies, include_in_schema).
- Health route registered at `/health` instead of `//health` when mounted at root path.
- CORS no longer combines `allow_credentials=True` with wildcard origin, closing a credentialed cross-origin exposure.
- Added Changelog URL to published package metadata.

### Breaking changes

- Requires `ag-ui-protocol>=1.0.0`; upgrade the protocol dependency.
- Users relying on the previous CORS behavior (wildcard origin with credentials) must reconfigure allowed origins explicitly.
- Health route path changed from `//health` to `/health` at root mount; update any clients hitting the old path.
