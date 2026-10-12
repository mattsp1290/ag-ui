# Changelog

## 0.3.0 — 2026-10-12

- Raised peer floor for `@ag-ui/core` and `@ag-ui/client` to `>=1.0.0`; the wrapper uses `contentToText`/`contentHasMedia`, which only exist in core 1.0.
- Adopted the 1.0 content models: validators sourced from `@ag-ui/core/schemas`, `zod` declared as a dependency.
- Follows the 1.0 part renames and flattens tool result content to text where the provider needs it.
- Drops file sources with a warning instead of sending a provider handle as text.

### Breaking changes

- Requires `@ag-ui/core` and `@ag-ui/client` `>=1.0.0`; installs against older versions are no longer supported.
- Part renames and content-flattening behavior changed; re-verify tool results and file/media handling after upgrade.
