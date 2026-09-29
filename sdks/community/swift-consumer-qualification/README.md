# Agentcraft Swift SDK consumer qualification

This is an independent SwiftPM consumer of the published
`https://github.com/mattsp1290/ag-ui-swift.git` repository. `Package.swift`
pins a full distribution commit SHA. It must resolve without a local path
override or monorepo source dependency.

The two copied SSE fixtures are synthetic Agentcraft watch captures from
`agentcraft/apps/app/assets/fixtures/` and are described in
`agentcraft/apps/app/test/fixtures/README.md`. The tests serve them through a
real loopback HTTP listener, check GET watch decoding and preserved snapshot
revisions, exercise a generic POST, test parser and transport limits, and cancel
one stream while another runs.

Run from this directory on macOS:

```bash
swift package resolve
swift test
xcodebuild -scheme AGUIConsumerQualification-Package \
  -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO build
xcodebuild -scheme AGUIConsumerQualification-Package \
  -destination 'platform=iOS Simulator,name=iPhone 16 Pro' \
  CODE_SIGNING_ALLOWED=NO test
```

Record the resolved revision in `Package.resolved`, the host Swift/Xcode
versions, test counts and the simulator runtime. The physical iPhone validation
requires a connected provisioned device and is a separate check.
