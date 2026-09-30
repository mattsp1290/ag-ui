# AG-UI Core Types

This repo contains the Rust types needed to work with the AG-UI protocol. Implemented using `serde` to support
(de)serialization. 

Contained are:

* [Message types](src/types/message.rs)
* [Event types](src/event.rs)
* [State trait bounds](src/state.rs)
* [Input types](src/types/input.rs)
* [Tool type](src/types/tool.rs)
* [Context type](src/types/context.rs)
* [ID (new)types](src/types/ids.rs)

Intended to be used with [`ag-ui-client`](../ag-ui-client).

### 1.0 fixture tolerances

The fixture test requires every valid 1.0 example to deserialize and every invalid
example to fail, except this explicit compatibility tolerance:

- `TextMessageEndEvent/unknown-property.json`: the flattened base event allows
  unrecognized event keys so older Rust clients can consume newer producers.

The test pins this list exactly. Removing a tolerance requires adding the missing
wire field or closed-object validation, then deleting its entry from the test.

## 0.2.0

This breaking release models AG-UI 1.0, including seven message roles, multimodal content parts, 31 current events, typed JSON Patch deltas, run outcomes, usage, and interrupts. See [CHANGELOG.md](CHANGELOG.md) and the [Rust SDK reference](/sdk/rust/overview).
