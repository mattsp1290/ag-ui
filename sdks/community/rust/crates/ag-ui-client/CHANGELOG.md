# Changelog

## Unreleased: server-readiness qualification

Adds public explicit-input lifecycle execution, complete reconciled result history, latest producer terminal status, and requested-run completion checks. Corrects streamed roles, snapshot/activity ownership and metadata, incremental bounded SSE, HTTP negotiation, request normalization, error-body limits, and structured diagnostic hooks. The HTTP current-1.0 path no longer translates retired THINKING/0.x events. Rust 1.88 is the compiler floor.

Git-based delivery requires the independent immutable-revision consumer proof. Registry publication remains a separate gate.

## 0.2.0

Breaking AG-UI 1.0 update from 0.1.0: depends on `ag-ui-core` 0.2.0; adds typed subscriber callbacks for activity, reasoning, and subagent events; exposes `RunAgentResult.outcome` and `.usage` and `RunAgentParams::with_resume`; expands chunk events; translates deprecated `THINKING_*` events to reasoning events with warnings. Update applications that exhaustively match events, messages, or run results.
