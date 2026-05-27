# MCP Middleware

AG-UI middleware that connects an agent run to one or more MCP servers.

`MCPMiddleware` takes a list of MCP server configurations in its constructor:

```ts
import { MCPMiddleware } from "@ag-ui/mcp-middleware";

const middleware = new MCPMiddleware([
  { type: "http", url: "https://example.com/mcp" },
]);
```

Placeholder scaffold — run-pipeline integration is not implemented yet.
