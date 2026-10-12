# Changelog

## 0.0.5 — 2026-10-12

- Declares `protocolVersion` and now skips audio/video content.
- Fixed empty-prompt resume after display-only frontend tools; the adapter now sends placeholder text instead of an empty user turn, so `followUp: false` is no longer required as a workaround.
- Updated validators to source from `@ag-ui/core/schemas`, follows part renames, flattens tool result content to text, and drops file sources with a warning.

### Breaking changes

- Requires `@ag-ui` 1.0 peer dependencies.
- Message part renames and content flattening change wire behavior; re-verify tool results and file-source handling against the 1.0 models.

## 0.0.4

- Publish the declared MIT license and license file in the npm package.
