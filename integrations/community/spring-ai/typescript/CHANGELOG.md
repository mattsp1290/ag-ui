# Changelog

## 0.0.4 — 2026-10-13

- Replaced the deprecated `maxVersion` setting with `maxProtocolVersion`, pinned to `0.0.39`.
- The `0.0.39` ceiling enables the client's content-flattening shim, so string message content is sent instead of `ContentPart[]`, which the community Java SDK rejects with HTTP 400.
- Rewrote the README to document Spring AI instead of the copied LlamaIndex content.

### Breaking changes

- Requires `@ag-ui/client` 1.0, where `maxProtocolVersion` exists; verify the peer dependency.
- Protocol version ceiling is now `0.0.39` under the `maxProtocolVersion` name; re-verify if you relied on `maxVersion`.

## 0.0.3

- Publish the declared Apache-2.0 license and license file in the npm package.
