# AG-UI Swift SDK

This community Swift package contains `AGUICore` for AG-UI event and message
types and `AGUIClient` for HTTP streaming agents. It also includes the
`agui-dojo-chat` example command. The package declares iOS 16+ and macOS 13+
in [Package.swift](Package.swift), and requires Swift 5.9 or newer.

## SwiftPM dependency

The standalone distribution is at
`https://github.com/mattsp1290/ag-ui-swift.git`; its repository root contains
`Package.swift`. Pin a full immutable commit SHA from the distribution repository:

```swift
dependencies: [
    .package(url: "https://github.com/mattsp1290/ag-ui-swift.git", revision: "<published full SHA>"),
],
targets: [
    .target(
        name: "YourApp",
        dependencies: [
            .product(name: "AGUICore", package: "AGUISwift"),
            .product(name: "AGUIClient", package: "AGUISwift"),
        ]
    ),
]
```

Import `AGUICore` for protocol models and `AGUIClient` for `HttpAgent`.
See [DISTRIBUTION.md](DISTRIBUTION.md) for the source mapping and update procedure.

## Bounded raw SSE watch

`BoundedSSEClient.open(_:maximumFrameBytes:maximumQueuedBytes:)` accepts any
`URLRequest`, including GET watches without `RunAgentInput` and POST requests
with a body. It returns status, content type and raw `SseEvent` values with
`event`, `id`, `retry` and `data`. Handle application control frames before
passing AG-UI JSON data to `AGUIEventDecoder`. This surface does not admit runs,
retry, interpret host state, or apply run lifecycle validation.

The parser retains at most `maximumFrameBytes` of source frame bytes; the
transport's own queued network data stays at or below `maximumQueuedBytes`.
One incoming Foundation callback `Data` may temporarily exist outside that
queue. Decoding a completed line and producing an event create transient string
copies of at most one frame. Exceeding either configured limit terminates only
that request with a content-free error; no bytes or events are silently dropped.
Call `SSEWatchResponse.cancel()` when leaving a watch early. Each watch owns its
URLSession, so cancellation leaves other watches and host networking untouched.

## Build and test

From this directory:

```bash
swift build
swift test
```

The example command connects to a local dojo at `http://127.0.0.1:18000` by
default. Set `AGUI_DOJO_BASE_URL` to use another base URL:

```bash
swift run agui-dojo-chat "Hello"
```

See [CHANGELOG.md](CHANGELOG.md), [DERIVATION.md](DERIVATION.md), and
[LICENSE](LICENSE).
