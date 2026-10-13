# Changelog

## 0.0.5 — 2026-10-13

- Fixed `clone()` so cloned agents carry over agent cards, clients, instructions, and the orchestrator; previously the first run of a clone threw while reading cards.
- Delegated arguments are now read from the run's own stream.
- Demo and route handling updated to interoperate with a2a-sdk 1.x while the middleware client still speaks protocol 0.3.

### Breaking changes

None.

## 0.0.4 — 2026-10-12

- Requires `@ag-ui/client` `>=1.0.0`; the unused `@ag-ui/core` peer is gone.
- Drops an unused `@ag-ui/core/schemas` import, so the package no longer imports anything from `@ag-ui/core`.

### Breaking changes

None.

## 0.0.3

- Publish the declared MIT license and license file in the npm package.
