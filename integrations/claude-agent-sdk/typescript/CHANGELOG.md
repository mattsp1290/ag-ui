# Changelog

## 0.0.6 — 2026-10-12

- `buildOptions` now defaults `settingSources` to `[]`, so runs no longer load the host's `~/.claude` settings or the working directory's `CLAUDE.md`.
- A caller-provided `settingSources` still takes precedence.

### Breaking changes

- Default `settingSources` changed from loading all sources to `[]`; runs relying on implicit host/filesystem settings must set `settingSources` explicitly.

## 0.0.5 — 2026-10-12

- Requires AG-UI 1.0 (`@ag-ui/core` / `@ag-ui/client` `>=1.0.0`). `RUN_STARTED` declares `protocolVersion`.
- Audio and video parts are skipped with a warning; tool-result content parts are flattened to text, and file sources are dropped with a warning.
- After a display-only frontend tool, the adapter no longer resumes with an empty prompt, so the `followUp: false` workaround is no longer needed.

### Breaking changes

None.

## 0.0.4

- Publish the declared MIT license and license file in the npm package.
