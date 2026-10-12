# Changelog

## 0.0.4 — 2026-10-12

- Removed an unused `RunFinishedEventSchema` import from `@ag-ui/core/schemas`; the package no longer imports anything from `@ag-ui/core`.
- Aligned with the 1.0 client, including the zod-free core entry and the enforcement pipeline that runs after middleware.

### Breaking changes

- The `@ag-ui/client` peer moved from `>=0.0.40` to `>=1.0.0`; only 1.x is tested.
- The `@ag-ui/core` peer dependency was dropped entirely.
- Verify against `@ag-ui/client` 1.x, which strips events against the schema after middleware runs.

## 0.0.3

- Publish the declared MIT license and license file in the npm package.
