# Changelog

## 0.2.0

Breaking AG-UI 1.0 update from 0.1.0: depends on `ag-ui-core` 0.2.0; adds typed subscriber callbacks for activity, reasoning, and subagent events; exposes `RunAgentResult.outcome` and `.usage` and `RunAgentParams::with_resume`; expands chunk events; translates deprecated `THINKING_*` events to reasoning events with warnings. Update applications that exhaustively match events, messages, or run results.
