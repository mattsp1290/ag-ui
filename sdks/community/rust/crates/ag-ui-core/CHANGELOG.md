# Changelog

## Unreleased: server-readiness qualification

Adds explicit tolerant ingress and strict producer boundaries, structured path diagnostics, correct effective JSON state/null/version processing, lossless legal patch extensions, and a documented limited Replay reducer. Bare serde parsing is no longer described as full wire conformance. Rust 1.88 is the compiler floor.

Git-based delivery requires the independent immutable-revision consumer proof. Registry publication remains a separate gate.

## 0.2.0

Breaking AG-UI 1.0 update from 0.1.0: seven message roles, multimodal content parts and sources, 31 current events including activity, reasoning, and subagent events, typed JSON Patch operations, run outcomes, token usage, interrupts, resume entries, and capabilities. Message and event shapes changed to match the 1.0 wire model.
