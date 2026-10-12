# Changelog

## 0.2.1 — 2026-10-12

- `build_options` now defaults `setting_sources` to `[]`, preventing runs from loading host `~/.claude` settings and the working directory's `CLAUDE.md`.
- Restores the isolation behavior of claude-agent-sdk 0.1 when running on 0.2.
- Callers can opt back into filesystem settings by setting `setting_sources` explicitly.

### Breaking changes

- Runs no longer inherit host filesystem settings (`~/.claude`, `CLAUDE.md`) by default; re-add `setting_sources` if you relied on that.

## 0.2.0 — 2026-10-12

- Requires AG-UI 1.0 (`ag-ui-protocol>=1.0.0`). `RUN_STARTED` declares `protocol_version`.
- Audio and video parts are skipped with a warning; other multimodal input is preserved.
- Errored turns end with `RUN_ERROR` instead of `RUN_FINISHED`, without duplicate assistant messages.
- Tool calls on the non-streaming fallback path carry `parent_message_id` (#2118).
- Sessions survive input errors.
- The endpoint helper forwards FastAPI route kwargs (`name`, `tags`, `dependencies`, …).

### Breaking changes

None.
