# @ag-ui/a2ui-middleware

Middleware for streaming A2UI surfaces and generation recovery activities.

## Upgrade notice: custom generation wrappers (PNI-679)

**Breaking behavior change from 0.0.12:** when an ordinary tool call and a
`render_a2ui` call have distinct, nonempty `parentMessageId` values, the middleware
now treats them as independent unless the ordinary tool is a declared generation
wrapper. This prevents a sibling render from taking over an unrelated tool's
activity and suppressing its result.

If your adapter uses a custom generation-tool name (for example,
`getA2UITools({ ..., toolName: "create_dashboard" })`), configure that same name
on the middleware when upgrading:

```ts
import { A2UIMiddleware } from "@ag-ui/a2ui-middleware";

const middleware = new A2UIMiddleware({
  a2uiGenerationToolNames: ["generate_a2ui", "create_dashboard"],
});
```

The default is `["generate_a2ui"]`. An explicit array replaces the default, so
include every generation wrapper used by your agent. These are the outer
wrappers, not the inner render tools listed in `a2uiToolNames`.

Without this migration, custom wrappers whose nested renders carry distinct
message IDs get separate activities for each attempt. A rejected attempt can
leave a failure activity beside a later successful render. Configuring the
wrapper restores one activity that is replaced by retries and final success or
failure. Wrappers whose nested renders omit message IDs retain the legacy
fallback; the default `generate_a2ui` wrapper needs no configuration.

This is an intentional compatibility tradeoff: message IDs identify assistant
messages, not parent tool calls. Neither tool arguments nor `TOOL_CALL_END`
(which ends argument streaming) reliably proves nesting.

## Ancestry limitations

- Calls sharing a nonempty parent message ID remain siblings, including declared
  generation wrappers.
- **Distinct-ID siblings involving `generate_a2ui` (or another declared wrapper)
  and a direct `render_a2ui` can still be mistaken for nested calls.** Their
  activities can be combined and valid output suppressed. Wrapper-name
  configuration does not resolve this pre-existing ambiguity.
- Missing or empty message IDs retain heuristic nesting. Concurrent generation
  wrappers can also be associated with the most recent candidate rather than the
  actual parent.

Unambiguous association requires adapters to provide parent-tool-call ancestry.
