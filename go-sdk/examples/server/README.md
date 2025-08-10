# AG-UI Example Server

A configurable HTTP server built with Fiber v3 framework, featuring Server-Sent Events (SSE) support and comprehensive configuration management.

## Features

- **Fiber v3 Framework**: High-performance HTTP server with built-in middleware
- **Server-Sent Events (SSE)**: Real-time event streaming capabilities
- **Structured Logging**: JSON-formatted logs with configurable levels
- **CORS Support**: Configurable Cross-Origin Resource Sharing
- **Graceful Shutdown**: Clean server termination with signal handling
- **Comprehensive Configuration**: Environment variables and CLI flags support

## Quick Start

```bash
# Run with defaults
go run cmd/server/main.go

# Run with custom configuration
go run cmd/server/main.go --host 127.0.0.1 --port 9090 --log-level debug

# Run with environment variables
AGUI_HOST=127.0.0.1 AGUI_PORT=9090 go run cmd/server/main.go
```

## Configuration

The server supports configuration through multiple sources with the following precedence:
**Command Line Flags > Environment Variables > Default Values**

### Default Values

| Setting | Default Value | Description |
|---------|---------------|-------------|
| Host | `0.0.0.0` | Server bind address |
| Port | `8080` | Server port (1-65535) |
| Log Level | `info` | Logging level (debug, info, warn, error) |
| Enable SSE | `true` | Enable Server-Sent Events |
| Read Timeout | `30s` | HTTP read timeout |
| Write Timeout | `30s` | HTTP write timeout |
| SSE Keep-Alive | `15s` | SSE connection keep-alive interval |
| CORS Enabled | `true` | Enable Cross-Origin Resource Sharing |

### Environment Variables

All environment variables use the `AGUI_` prefix:

| Variable | Type | Example |
|----------|------|---------|
| `AGUI_HOST` | string | `127.0.0.1` |
| `AGUI_PORT` | integer | `9090` |
| `AGUI_LOG_LEVEL` | string | `debug` |
| `AGUI_ENABLE_SSE` | boolean | `false` |
| `AGUI_READ_TIMEOUT` | duration | `60s` |
| `AGUI_WRITE_TIMEOUT` | duration | `45s` |
| `AGUI_SSE_KEEPALIVE` | duration | `30s` |
| `AGUI_CORS_ENABLED` | boolean | `false` |

**Duration Format**: Use Go duration strings like `30s`, `5m`, `1h30m`

### Command Line Flags

| Flag | Type | Description |
|------|------|-------------|
| `--host` | string | Server host address |
| `--port` | int | Server port (1-65535) |
| `--log-level` | string | Log level (debug, info, warn, error) |
| `--enable-sse` | bool | Enable Server-Sent Events |
| `--read-timeout` | duration | Read timeout duration |
| `--write-timeout` | duration | Write timeout duration |
| `--sse-keepalive` | duration | SSE keep-alive duration |
| `--cors-enabled` | bool | Enable CORS |

### Configuration Examples

```bash
# Using environment variables
export AGUI_HOST=127.0.0.1
export AGUI_PORT=9090
export AGUI_LOG_LEVEL=debug
export AGUI_ENABLE_SSE=true
export AGUI_READ_TIMEOUT=60s
export AGUI_CORS_ENABLED=false
go run cmd/server/main.go

# Using command line flags
go run cmd/server/main.go \
  --host 127.0.0.1 \
  --port 9090 \
  --log-level debug \
  --enable-sse \
  --read-timeout 60s \
  --cors-enabled=false

# Mixed configuration (flags override env vars)
export AGUI_HOST=192.168.1.100
go run cmd/server/main.go --host 127.0.0.1  # Will use 127.0.0.1
```

## API Endpoints

### Health Check
```
GET /health
```
Returns server health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "ag-ui-server"
}
```

### Server Information
```
GET /info
```
Returns server configuration and capabilities.

**Response:**
```json
{
  "service": "ag-ui-server",
  "version": "1.0.0",
  "sse_enabled": true,
  "cors_enabled": true
}
```

### Server-Sent Events (if enabled)
```
GET /events
```
Establishes an SSE connection for real-time events.

**Response:**
```
Content-Type: text/event-stream

data: {"type": "connection", "message": "SSE connection established"}

```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run specific test package
go test ./internal/config
```

### Building

```bash
# Build the server
go build -o server cmd/server/main.go

# Run the built server
./server --help
```

### Project Structure

```
├── cmd/
│   └── server/
│       └── main.go          # Server entry point
├── internal/
│   └── config/
│       ├── config.go        # Configuration management
│       └── config_test.go   # Configuration tests
├── go.mod                   # Go module definition
└── README.md               # This file
```

## Logging

The server uses structured JSON logging with configurable levels:

```json
{"time":"2025-08-10T10:30:00Z","level":"INFO","msg":"Server configuration loaded","host":"0.0.0.0","port":8080,"log_level":"info","enable_sse":true,"read_timeout":"30s","write_timeout":"30s","sse_keepalive":"15s","cors_enabled":true}
```

### Log Levels

- **debug**: Detailed debugging information
- **info**: General informational messages  
- **warn**: Warning messages for potential issues
- **error**: Error messages for failures

## Error Handling

The server includes comprehensive error handling:

- **Configuration Errors**: Invalid values are rejected with clear error messages
- **Validation Errors**: Configuration validation runs at startup
- **Runtime Errors**: Structured error responses with appropriate HTTP status codes
- **Graceful Shutdown**: Clean termination on SIGINT/SIGTERM signals

Example configuration error:
```
Failed to load configuration: configuration validation failed: port must be between 1 and 65535, got 70000
```

## Security Considerations

- **No Secrets Logging**: Configuration logging excludes sensitive information
- **Input Validation**: All configuration values are validated before use
- **Safe Defaults**: Secure default values for all settings
- **CORS Configuration**: Configurable CORS settings for security