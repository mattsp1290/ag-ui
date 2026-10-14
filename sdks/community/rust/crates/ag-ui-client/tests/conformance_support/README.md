# Rust conformance oracle

The pinned 1.0 stream corpus has 68 cases. `classification.json` names each
case, its specification page and the reason for its classification. All 62
applicable cases execute through the public HTTP client and explicit-input
lifecycle. Six legacy cases are out of scope: the old content flattening,
THINKING translation, binary upgrade, subagent downgrade, old-peer declaration
omission, and the current-peer fixture that still requires retired binary
content. This is deliberately not the old “64 passed, four skipped” claim.

The unknown optional text-role case follows the normative processing rule,
with an explicit `expectOverrides.rust` in the shared fixture. Its TypeScript
base expectation continues to pin that reference client's admitted bug.
Malformed-known-value overrides only replace validator-specific English with
Rust's semantic error category and path, and assert non-delivery. TypeScript
and .NET expectations are unchanged. The matrix distinguishes normative
passes from the one intentional normative divergence from the reference.

All 14 expectation keys are implemented. The oracle sees application-delivered
canonical events, structured boundary and reducer diagnostics, finalized
messages/state, and the actual complete HTTP body and relevant headers. A
producer RUN_ERROR is counted only from a delivered event, never from a
consumer exception. Empty object subsets require an actual object, present
null never matches absence, and arrays retain exact order and length.

The historical positive corpus predates request-ID assertions. Its request
uses the last announced run's IDs unless fixture input explicitly supplies
them; response events are never rewritten. Independent identity-negative
client tests author requests separately. Every input field is preserved and
unsupported client configuration is classified instead of ignored.

`http.rs` parses both Content-Length and chunked request bodies, including
headers/body boundaries and trailers. Both capture and execution are bounded.
Negative controls prove event loss, patch-extra loss, metadata loss,
missing/null confusion, required/unwanted warnings, changed request fields,
consumer/producer-error conflation and manifest/expectation-key mutations fail.

The separate core `spec_fixtures` suite validates all 135 valid and 84 invalid
documents plus their 84 exact diagnostic companions using jsonschema 0.33
against the pinned Draft 2020-12 schema. It deterministically resolves the
validator's `$ref` evaluation paths into the companion schema locations.
Typed representation and production normalization are separate lanes; the
encoder suite owns real producer egress of all 35 null-omission cases.

Baseline protocol pins (before Rust lane metadata):

- schema blob: `c2ce3c18036b6bb2afe583a9b916959e7dcd9b78`
- stream MANIFEST blob: `40a6774745bc620465164b0e59efed2d7461f682`
- stream tree: `36faadc4c40b0458e9bd0118ce007ca5087c028d`
- shared SDK fixtures tree: `e6dd6fd9d264a9af6c231d4865244a3ed85d99be`

Changing fixture metadata updates the stream tree, not the protocol schema or
inventory. Tests fail on duplicate/stale manifest entries, omitted files,
stale scope classifications, unknown expectation keys and vacuous cases.
