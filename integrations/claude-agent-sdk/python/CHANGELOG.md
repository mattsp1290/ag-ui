# Changelog

## 0.2.0 — 2026-10-12

- Declares `protocol_version`, skips audio/video parts, and requires `ag-ui-protocol` 1.0.
- Adapter aligned to 1.0 models: validators from `@ag-ui/core/schemas`, part renames, flattened tool-result content, and dropped file sources emit a warning.
- Preserves sessions on input errors and preserves multimodal input.
- Errored turns now emit `RUN_ERROR` instead of `RUN_FINISHED`, with terminal ownership moved into `run()`.
- Deduplicates assistant messages on errored turns and surfaces `RunErrorEvent`.
- Fixes missing `parent_message_id` on tool calls in the non-streaming fallback path (#2118).
- FastAPI endpoint helpers forward route kwargs (name, tags, summary, operation_id, dependencies, include_in_schema).
- CORS no longer combines `allow_credentials` with wildcard origins (#1939, #1940).
- Adds Changelog URL to published package metadata and completes LICENSE/license-field coverage.

### Breaking changes

- Now requires `ag-ui-protocol` 1.0; adapter follows 1.0 part renames and model changes.
- Errored runs emit `RUN_ERROR` rather than `RUN_FINISHED`; consumers relying on the prior terminal event must update.
- CORS defaults changed: wildcard origins no longer paired with credentials, so credentialed cross-origin requests may stop working.
- Audio/video parts are now skipped.
