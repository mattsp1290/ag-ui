# Changelog

## 0.2.0 — 2026-10-12

- WayFlow flows now receive the last user message as input, instead of whatever message came last after dropping activity/reasoning messages.
- A run with no user message now fails with a clear ValueError, reported by the endpoint as RUN_ERROR.
- ContentPart arrays on user and tool messages are flattened to text in both LangGraph and WayFlow runners; media and FileSource parts are skipped with a warning.
- Developer messages are mapped to system instructions; activity and reasoning messages are dropped instead of raising.
- RUN_STARTED now carries protocol_version=PROTOCOL_VERSION.
- Endpoint helpers now forward FastAPI route kwargs (name, tags, summary, operation_id, dependencies, include_in_schema).
- Assistant message content is now read defensively when the field is absent from the dump.
- Examples locked on ag-ui-protocol 1.0; added Changelog URL to published package metadata.

### Breaking changes

- Minimum ag-ui-protocol raised to >=1.0.0.
- WayFlow flow input now resolves to the last user message rather than the last remaining message; verify flows relying on prior behavior.
- Runs without a user message now raise ValueError (RUN_ERROR) instead of IndexError.
- Media and FileSource content parts on user/tool messages are now dropped with a warning instead of being passed through.
