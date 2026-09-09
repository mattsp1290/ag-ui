<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

# General Guidelines for working with Nx

- For requests to add A2UI rendering to AG-UI applications or to scaffold an
  AG-UI + A2UI quickstart, use
  `skills/ag-ui-a2ui-integration/SKILL.md`.
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- You have access to the Nx MCP server and its tools, use them to help the user
- When answering questions about the repository, use the `nx_workspace` tool first to gain an understanding of the workspace architecture where applicable.
- When working in individual projects, use the `nx_project_details` mcp tool to analyze and understand the specific project structure and dependencies
- For questions around nx configuration, best practices or if you're unsure, use the `nx_docs` tool to get relevant, up-to-date docs. Always use this instead of assuming things about nx configuration
- If the user needs help with an Nx configuration or project graph error, use the `nx_workspace` tool to get any errors
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.

<!-- nx configuration end-->


# Go SDK validation

- The Go SDK and its consumers are separate modules:
  `sdks/community/go`, `sdks/community/go/example/client`,
  `sdks/community/go/example/server`, and
  `integrations/community/genkit/go/genkit`. SDK-root `go test ./...` does not
  test the nested modules. Use each module's `go.mod` toolchain and run its
  build, race tests, and vet checks separately with `GOWORK=off`.
- These Go modules currently have no Nx targets. Use direct Go commands for
  them; continue using Nx for the TypeScript projects. The
  [Go SDK README](sdks/community/go/README.md#verify-a-checkout) contains the
  module checks and cross-language parity commands.
- Before running `scripts/go-sdk-parity.sh`, install checkout dependencies
  with the repository's pinned Node/pnpm/Python toolchains,
  `pnpm install --frozen-lockfile`, and `uv sync --locked` in `sdks/python`.
  The coordinator owns fresh temporary artifacts and disables Nx caching for
  its oracle calls. A missing runtime or artifact is a failure, not a skip.
- Go server Docker builds use the SDK-root `.dockerignore` and both
  `example/server/Dockerfile` targets. Flutter also creates a separate context
  through `sdks/community/dart/example/test/helpers/go_build_context.dart`.
  When changing Go runtime dependencies, check both direct image builds and
  the Flutter contract suite described in the server README.
