# AG-UI Go SDK

A comprehensive Go SDK for building AI agents that seamlessly integrate with front-end applications using the AG-UI (Agent-User Interaction) protocol.

## Overview

AG-UI is a lightweight, event-based protocol that standardizes how AI agents connect to front-end applications, enabling:

- **Real-time streaming communication** between agents and UIs
- **Bidirectional state synchronization** with JSON Patch operations
- **Human-in-the-loop collaboration** for complex workflows
- **Tool-based interactions** for enhanced agent capabilities

## Features

- 🚀 **High-performance** - Built for production workloads with minimal latency
- 🔌 **Multiple transports** - HTTP/SSE, WebSocket, and traditional HTTP
- 🛡️ **Type-safe** - Full Go type safety with comprehensive interfaces
- 🔧 **Extensible** - Pluggable middleware and transport layers
- 📝 **Well-documented** - Comprehensive documentation and examples
- 🧪 **Test-friendly** - Built-in testing utilities and mocks

## Quick Start

### Installation

```bash
# Install the SDK
go get github.com/mattsp1290/ag-ui/go-sdk

# Install development tools (for contributors)
make tools-install
# or
./scripts/install-tools.sh
```

### Prerequisites

- **Go 1.21+** - Required for all features and compatibility
- **protoc** - Protocol buffer compiler (auto-installed by setup scripts)
- **golangci-lint** - For code quality checks (auto-installed)

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/mattsp1290/ag-ui/go-sdk.git
cd go-sdk

# Install all dependencies and development tools
./scripts/install-tools.sh

# Verify installation
make deps-verify

# Run tests
make test

# Check code quality
make lint
```

### Basic Server

```go
package main

import (
    "context"
    "log"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/server"
)

type EchoAgent struct{}

func (a *EchoAgent) HandleEvent(ctx context.Context, event core.Event) ([]core.Event, error) {
    // Echo back the received event
    return []core.Event{event}, nil
}

func (a *EchoAgent) Name() string { return "echo" }
func (a *EchoAgent) Description() string { return "Echoes back received messages" }

func main() {
    // Create server
    s := server.New(server.Config{
        Address: ":8080",
    })

    // Register agent
    s.RegisterAgent("echo", &EchoAgent{})

    // Start server
    log.Println("Starting AG-UI server on :8080")
    if err := s.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}
```

### Basic Client

```go
package main

import (
    "context"
    "log"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
)

func main() {
    // Create client
    c, err := client.New(client.Config{
        BaseURL: "http://localhost:8080/ag-ui",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Send event to agent
    // Implementation details coming in subsequent phases
    log.Println("Client created successfully")
}
```

## Project Structure

```
go-sdk/
├── pkg/                    # Public API packages
│   ├── core/              # Core types and interfaces
│   ├── client/            # Client SDK
│   ├── server/            # Server SDK  
│   ├── transport/         # Transport implementations
│   ├── encoding/          # Event encoding/decoding
│   ├── middleware/        # Middleware system
│   ├── tools/             # Tool execution framework
│   └── state/             # State management
├── internal/              # Internal implementation
│   ├── protocol/          # Protocol implementation
│   ├── validation/        # Event validation
│   ├── utils/             # Shared utilities
│   └── testutil/          # Testing helpers
├── examples/              # Example applications
│   ├── basic/             # Basic usage examples
│   ├── advanced/          # Advanced features
│   └── integrations/      # Framework integrations
├── cmd/                   # Command-line tools
│   └── ag-ui-cli/         # Development CLI
├── proto/                 # Protocol buffer definitions
├── docs/                  # Documentation
└── test/                  # Integration tests
```

## Development Status

This is the foundational structure for the AG-UI Go SDK. The project is organized into 8 development phases:

- ✅ **Phase 1**: Project Structure Setup (Current)
- 🔄 **Phase 2**: Dependencies & Tooling
- ⏳ **Phase 3**: Protocol Buffer Implementation
- ⏳ **Phase 4**: Core Protocol Implementation  
- ⏳ **Phase 5**: Transport Layer Implementation
- ⏳ **Phase 6**: Client & Server SDKs
- ⏳ **Phase 7**: Advanced Features
- ⏳ **Phase 8**: Documentation & Examples

## Documentation

- [Development Guide](DEVELOPMENT.md) - **Complete development setup and workflow guide**
- [Getting Started](docs/getting-started.md) - Detailed setup and usage guide
- [Architecture](ARCHITECTURE.md) - Technical architecture and design decisions
- [Contributing](CONTRIBUTING.md) - Development guidelines and contribution process
- [Dependencies](docs/dependencies.md) - Dependency management and security strategy
- [Examples](examples/) - Code examples and tutorials

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup
- Code style and standards  
- Testing requirements
- Pull request process

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Related Projects

- [TypeScript SDK](../typescript-sdk/) - TypeScript/JavaScript implementation
- [Python SDK](../python-sdk/) - Python implementation
- [Protocol Specification](../docs/) - Detailed protocol documentation

---

**Note**: This SDK is currently in active development. APIs may change as we progress through the development phases. See the [roadmap](docs/development/roadmap.mdx) for more details. 