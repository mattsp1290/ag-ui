# AG-UI Client

A command-line interface for interacting with AG-UI servers, providing tool-based UI capabilities with Server-Sent Events (SSE) support.

## Features

- Tool-based UI client with SSE support
- Real-time communication with AG-UI servers
- State synchronization capabilities
- Built with Fang CLI framework (Charmbracelet)
- Structured logging with logrus (JSON/text formats)
- Configurable via flags or environment variables

## Requirements

- Go 1.21 or higher
- macOS (darwin/amd64 or darwin/arm64)

## Building

To build the CLI binary:

```bash
go build -o ag-ui-client ./cmd/fang
```

## Usage

### Basic Usage

Display help and available commands:

```bash
./ag-ui-client --help
```

### Global Flags

| Flag | Environment Variable | Description | Default |
|------|---------------------|-------------|---------|
| `--server` | `AG_UI_SERVER` | AG-UI server URL | `http://localhost:8080` |
| `--api-key` | `AG_UI_API_KEY` | API key for authentication | (empty) |
| `--log-level` | `AG_UI_LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `--log-format` | `AG_UI_LOG_FORMAT` | Logging format (json, text) | `text` |
| `--output` | `AG_UI_OUTPUT` | Output format (json, text) | `text` |

### Examples

Connect to a server with an API key:
```bash
./ag-ui-client --server https://api.example.com --api-key your-key
```

Set custom log level and output format:
```bash
./ag-ui-client --log-level debug --output json
```

Use environment variables:
```bash
export AG_UI_SERVER=https://api.example.com
export AG_UI_API_KEY=your-key
./ag-ui-client
```

Display current configuration:
```bash
./ag-ui-client config
```

## Development

This client is part of the AG-UI ecosystem and uses:
- [Fang](https://github.com/charmbracelet/fang) v0.1.0 - CLI starter kit from Charm
- [Cobra](https://github.com/spf13/cobra) - Command framework (via Fang)
- [Logrus](https://github.com/sirupsen/logrus) - Structured logging with JSON support

## Project Structure

```
go-sdk/examples/client/
├── cmd/
│   └── fang/
│       └── main.go       # CLI entrypoint with Fang/Cobra commands
├── internal/
│   └── logging/
│       └── logging.go    # Centralized logging setup with logrus
├── go.mod               # Module definition with pinned dependencies
├── go.sum               # Dependency checksums
└── README.md            # This file
```

## Next Steps

- Add SSE client implementation
- Implement tool-based UI components
- Add server connection commands
- Implement state synchronization