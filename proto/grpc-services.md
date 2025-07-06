# AG-UI gRPC Service Definitions

This directory contains protobuf definitions for gRPC services that implement the AG-UI protocol, providing an alternative to the RESTful HTTP endpoints.

## Overview

The gRPC services mirror the functionality of the existing HTTP-based AG-UI protocol while taking advantage of gRPC's features like bidirectional streaming, strong typing, and built-in error handling.

## Service Definitions

### 1. AGUIService (`service.proto`)

The primary service that mirrors the core HTTP endpoint functionality:

#### Methods

- **`RunAgent(RunAgentRequest) returns (stream EventResponse)`**
  - Equivalent to `POST /` in the REST API
  - Executes an agent with the given input and streams AG-UI events back
  - Uses server-side streaming for event delivery

- **`HealthCheck(HealthCheckRequest) returns (HealthCheckResponse)`**
  - Health monitoring endpoint
  - Returns service status and optional diagnostic information

- **`GetServiceInfo(ServiceInfoRequest) returns (ServiceInfoResponse)`**
  - Returns service capabilities, version, and supported features
  - Useful for client capability negotiation

- **`ValidateInput(RunAgentRequest) returns (ValidationResponse)`**
  - Validates input without executing the agent
  - Useful for debugging and client-side validation

#### Key Messages

- **`RunAgentRequest`**: Maps directly to `RunAgentInput` from the REST API
- **`EventResponse`**: Wraps any AG-UI event type for streaming
- **`ServiceCapabilities`**: Describes what features the service supports

### 2. AGUIAdvancedService (`advanced_service.proto`)

Extended service for advanced use cases and specialized workflows:

#### Methods

- **`StreamingChat(stream ChatRequest) returns (stream ChatResponse)`**
  - Bidirectional streaming for real-time conversations
  - Supports session management and chat controls

- **`HumanInLoop(stream HumanInLoopRequest) returns (stream HumanInLoopResponse)`**
  - Implements human-in-the-loop approval workflows
  - Supports approval requests and input collection

- **`GetState/UpdateState/WatchState`**
  - State management operations with real-time updates
  - Supports JSON Patch operations for efficient state deltas

- **`RegisterTools/ExecuteTool`**
  - Dynamic tool registration and execution
  - Allows runtime tool management

- **`CreateSession/GetSession/EndSession`**
  - Session lifecycle management
  - Supports session persistence and expiration

- **`BatchRunAgent`**
  - Batch execution of multiple agent requests
  - Supports parallel and sequential processing

## Protocol Mapping

### HTTP to gRPC Mapping

| HTTP Endpoint | gRPC Method | Description |
|---------------|-------------|-------------|
| `POST /` | `RunAgent` | Execute agent with streaming events |
| N/A | `HealthCheck` | Service health monitoring |
| N/A | `GetServiceInfo` | Service capabilities discovery |

### Event Streaming

- **HTTP**: Server-Sent Events (SSE) with `text/event-stream`
- **gRPC**: Server-side streaming with `EventResponse` messages
- **Content**: Both use the same AG-UI event types defined in `events.proto`

### Error Handling

- **HTTP**: HTTP status codes + error events
- **gRPC**: gRPC status codes + structured error details

## Implementation Examples

### Basic Agent Execution

```protobuf
// Request
RunAgentRequest {
  thread_id: "thread_123"
  run_id: "run_456"
  messages: [
    {
      id: "msg_1"
      role: "user"
      content: "Hello, world!"
    }
  ]
  tools: []
  context: []
}

// Response Stream
EventResponse { event: { type: RUN_STARTED, thread_id: "thread_123", run_id: "run_456" } }
EventResponse { event: { type: TEXT_MESSAGE_START, message_id: "msg_2", role: "assistant" } }
EventResponse { event: { type: TEXT_MESSAGE_CONTENT, message_id: "msg_2", delta: "Hello! " } }
EventResponse { event: { type: TEXT_MESSAGE_CONTENT, message_id: "msg_2", delta: "How can I help?" } }
EventResponse { event: { type: TEXT_MESSAGE_END, message_id: "msg_2" } }
EventResponse { event: { type: RUN_FINISHED, thread_id: "thread_123", run_id: "run_456" } }
```

### Bidirectional Chat

```protobuf
// Client sends
ChatRequest {
  init: {
    session_id: "session_789"
    tools: [...]
  }
}

// Server responds
ChatResponse {
  init: {
    chat_id: "chat_abc"
    success: true
  }
}

// Client sends message
ChatRequest {
  message: {
    chat_id: "chat_abc"
    message: {
      id: "msg_3"
      role: "user"
      content: "What's the weather?"
    }
  }
}

// Server streams events
ChatResponse {
  event: {
    event: { type: TEXT_MESSAGE_START, message_id: "msg_4", role: "assistant" }
  }
}
```

## Code Generation

### Go

```bash
# Generate Go code
protoc --go_out=./go-sdk/pkg/proto/generated \
       --go-grpc_out=./go-sdk/pkg/proto/generated \
       --proto_path=./proto \
       ./proto/service.proto ./proto/advanced_service.proto
```

### TypeScript

```bash
# Generate TypeScript code
protoc --plugin=protoc-gen-ts_proto=./node_modules/.bin/protoc-gen-ts_proto \
       --ts_proto_out=./typescript-sdk/packages/grpc/src/generated \
       --proto_path=./proto \
       ./proto/service.proto ./proto/advanced_service.proto
```

### Python

```bash
# Generate Python code
python -m grpc_tools.protoc \
       --python_out=./python-sdk/ag_ui/grpc/generated \
       --grpc_python_out=./python-sdk/ag_ui/grpc/generated \
       --proto_path=./proto \
       ./proto/service.proto ./proto/advanced_service.proto
```

## Benefits of gRPC Implementation

### Performance
- Binary protocol is more efficient than JSON over HTTP
- HTTP/2 multiplexing reduces connection overhead
- Built-in compression support

### Type Safety
- Strong typing with protobuf schemas
- Compile-time validation of message structures
- Cross-language compatibility

### Streaming
- Bidirectional streaming for real-time interactions
- Backpressure handling built into the protocol
- Efficient resource usage

### Tooling
- Rich ecosystem of tools and libraries
- Built-in service discovery and load balancing
- Observability and monitoring support

## Migration Path

### Phase 1: Basic Service
1. Implement `AGUIService.RunAgent` method
2. Add health check endpoint
3. Support existing event types

### Phase 2: Advanced Features
1. Add bidirectional streaming chat
2. Implement human-in-the-loop workflows
3. Add state management operations

### Phase 3: Production Features
1. Add authentication and authorization
2. Implement rate limiting and quotas
3. Add comprehensive monitoring and logging

## Compatibility

The gRPC services are designed to be fully compatible with existing AG-UI clients by:

1. **Event Compatibility**: Using the same event types defined in `events.proto`
2. **Data Compatibility**: Using the same message structures from `types.proto`
3. **Behavior Compatibility**: Maintaining the same semantic behavior as HTTP endpoints

This allows for gradual migration from HTTP to gRPC while maintaining interoperability.