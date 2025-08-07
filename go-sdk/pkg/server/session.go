package server

// This file serves as a compatibility layer and re-exports types from the refactored session files.
// The original session.go (2,853 lines) has been refactored into focused files:
// - session_manager.go: Core session management logic
// - session_config.go: Configuration types and validation
// - session_memory.go: Memory storage implementation
// - session_redis.go: Redis storage implementation  
// - session_database.go: Database storage implementation
// - session_middleware.go: HTTP middleware and cookie handling
// - session_services.go: Background services and utilities

// Re-export main session types for backward compatibility
// These types are now defined in their respective focused files

// Core session types are defined in session_manager.go
// - Session struct
// - SessionManager struct  
// - SessionMetrics struct
// - SessionStorage interface

// Configuration types are defined in session_config.go
// - SessionConfig struct
// - RedisSessionConfig struct
// - SecureRedisSessionConfig struct
// - DatabaseSessionConfig struct
// - SecureDatabaseSessionConfig struct
// - MemorySessionConfig struct

// Storage implementations are in their respective files:
// - MemorySessionStorage (session_memory.go)
// - RedisSessionStorage, SecureRedisSessionStorage (session_redis.go)
// - DatabaseSessionStorage, SecureDatabaseSessionStorage (session_database.go)

// Middleware and HTTP integration are in session_middleware.go
// - SessionMiddleware functions
// - Cookie management functions
// - Context helpers

// Background services and utilities are in session_services.go
// - Cleanup services
// - Metrics collection
// - Shutdown handling

// This refactoring improves code organization by:
// 1. Breaking down a 2,853-line file into focused, maintainable modules
// 2. Separating concerns (config, storage, middleware, services)
// 3. Maintaining backward compatibility through shared interfaces
// 4. Making the codebase easier to test and modify
// 5. Reducing cognitive load for developers working with sessions

// All public APIs remain the same - this is a pure refactoring for code organization