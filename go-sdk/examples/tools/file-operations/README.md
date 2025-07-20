# File Operations Tool Example

A secure file operations tool demonstrating:

- Safe file system access with path validation
- Multiple file operations (read, write, list, copy, delete)
- Security constraints and sandboxing
- Error handling for file system operations

## Usage

```bash
go run main.go
```

## Features

- **File Operations**: Read, write, list, copy, delete files
- **Security**: Path validation and directory restrictions
- **Size Limits**: Configurable file size constraints
- **Backup Options**: Automatic backup creation
- **Error Handling**: Comprehensive error reporting

## Security Features

- Path traversal protection
- Directory access restrictions
- File size limits
- Safe path validation

## Running Tests

```bash
go test ./tests/...
```

This example demonstrates secure file handling patterns with the AG-UI SDK.