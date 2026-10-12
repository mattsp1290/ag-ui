# Changelog

## 1.0.1 — 2026-10-12

- Null optional fields are now omitted even when the serializer context is inserted into host-owned `JsonSerializerOptions`, fixing stray `null` fields (e.g. `parentRunId`, `input`) under Microsoft Agent Framework on .NET 10.
- Clarified serializer context remarks documenting the per-property null rule.

### Breaking changes

- Serialized output no longer includes null optional fields in host-owned JSON options scenarios; re-verify any consumers that relied on those keys being present.

## 1.0.0 — 2026-09-17

- Models are now generated from the frozen AG-UI 1.0 JSON Schema, covering content parts, tool result parts, capabilities, run outcomes, token usage with cache writes, and file source.
- `PROTOCOL_VERSION` now reads "1.0"; generated headers name the 1.0 schema address.
- Generator emits the `AGUIProtocol` type.

### Breaking changes

- Models renamed to their 1.0 names; verify usages against the generated types.
- Optional nulls are omitted on serialization while payload nulls are preserved.
- Protocol version constant value changed to "1.0".
