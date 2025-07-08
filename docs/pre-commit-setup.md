# Pre-commit Setup for ag-ui Go SDK

This document describes the pre-commit configuration for the ag-ui Go SDK project, including setup instructions and usage guidelines.

## Overview

The pre-commit framework helps maintain code quality by running automated checks before each commit. Our configuration includes:

- **Go formatting and linting**: `gofmt`, `goimports`, `golangci-lint`
- **Security scanning**: `gosec`, `detect-secrets`, `govulncheck`
- **Code quality**: Unit tests, vet checks, build verification
- **Documentation**: Markdown linting, Protocol Buffer validation
- **General hygiene**: Trailing whitespace, file formatting, merge conflicts

## Quick Start

### Automatic Setup

Run the setup script to install and configure everything automatically:

```bash
# From the repository root
./scripts/setup-pre-commit.sh
```

This script will:
1. Install pre-commit (if not already installed)
2. Install required Go tools
3. Install additional linting tools
4. Setup pre-commit hooks
5. Create secrets baseline
6. Run initial pre-commit checks

### Manual Setup

If you prefer to set up manually:

```bash
# Install pre-commit
pip install pre-commit
# or
brew install pre-commit

# Install Go tools
make tools-install

# Install pre-commit hooks
pre-commit install

# Run on all files (optional)
pre-commit run --all-files
```

## Configuration Files

### `.pre-commit-config.yaml`

The main configuration file that defines all hooks and their settings. Key sections:

- **Go tooling**: Formatting, imports, linting, and testing
- **Security**: Vulnerability scanning and secret detection
- **Documentation**: Markdown and Protocol Buffer validation
- **File hygiene**: Whitespace, line endings, and conflict detection

### `.golangci.yml`

Comprehensive Go linting configuration with:
- **80+ enabled linters**: Covering code quality, security, and maintainability
- **Reasonable defaults**: Configured for Go 1.24+ with appropriate complexity limits
- **Smart exclusions**: Different rules for tests, generated files, and examples

## Available Hooks

### Go-specific Hooks

| Hook | Description | When it runs |
|------|-------------|--------------|
| `go-fmt` | Formats Go code using `gofmt` | On `.go` files |
| `go-imports` | Organizes imports with `goimports` | On `.go` files |
| `go-vet` | Runs `go vet` for suspicious constructs | On `.go` files |
| `go-mod-tidy` | Ensures `go.mod` is tidy | On `go.mod`/`go.sum` |
| `go-unit-tests` | Runs unit tests with race detection | On `.go` files |
| `golangci-lint` | Comprehensive Go linting | On `.go` files |
| `gosec` | Security vulnerability scanning | On `.go` files |
| `govulncheck` | Checks for known vulnerabilities | On `go.mod`/`go.sum` |

### General Hooks

| Hook | Description | Files |
|------|-------------|-------|
| `trailing-whitespace` | Removes trailing whitespace | All files |
| `end-of-file-fixer` | Ensures files end with newline | All files |
| `check-yaml` | Validates YAML syntax | `.yaml`, `.yml` |
| `check-json` | Validates JSON syntax | `.json` |
| `check-merge-conflict` | Detects merge conflict markers | All files |
| `detect-secrets` | Scans for secrets in code | All files |
| `markdownlint` | Lints Markdown files | `.md` |
| `buf-lint` | Lints Protocol Buffer files | `.proto` |
| `shellcheck` | Lints shell scripts | `.sh`, `.bash` |
| `hadolint` | Lints Dockerfiles | `Dockerfile*` |

### Local Hooks

| Hook | Description | Purpose |
|------|-------------|---------|
| `go-generate` | Runs `go generate` | Ensures generated code is current |
| `go-build` | Builds the project | Verifies compilation |
| `proto-validate` | Validates protobuf compilation | Ensures proto files are valid |

## Usage

### Running Pre-commit

```bash
# Run on staged files only
pre-commit run

# Run on all files
pre-commit run --all-files

# Run specific hook
pre-commit run go-fmt

# Run specific hook on all files
pre-commit run --all-files golangci-lint
```

### Bypassing Hooks

Sometimes you need to bypass hooks (use sparingly):

```bash
# Skip all hooks
git commit --no-verify

# Skip specific hook
SKIP=go-unit-tests git commit -m "WIP: work in progress"

# Skip multiple hooks
SKIP=go-unit-tests,gosec git commit -m "WIP: work in progress"
```

### Updating Hooks

```bash
# Update all hooks to latest versions
pre-commit autoupdate

# Update specific hook
pre-commit autoupdate --repo https://github.com/golangci/golangci-lint
```

## Integration with Existing Workflow

### Make Targets

The configuration integrates with existing Makefile targets:

```bash
# Use these for development
make pre-commit      # Runs pre-commit checks
make fmt            # Formats code
make lint           # Runs linters
make test-short     # Runs unit tests

# Use these for CI
make ci             # Full CI pipeline
make full-check     # Comprehensive checks
```

### IDE Integration

#### VS Code

Install the pre-commit extension:

```json
{
  "pre-commit.enabled": true,
  "pre-commit.runOnSave": true
}
```

#### GoLand/IntelliJ

1. Install the pre-commit plugin
2. Configure to run on save
3. Set up golangci-lint integration

## Troubleshooting

### Common Issues

#### `pre-commit` command not found

```bash
# Install via pip
pip install pre-commit

# Or via Homebrew
brew install pre-commit
```

#### Go tools not found

```bash
# Install required Go tools
make tools-install

# Or run the setup script
./scripts/setup-pre-commit.sh
```

#### Hooks taking too long

```bash
# Run hooks in parallel (default)
pre-commit run --all-files

# Skip slow hooks for quick checks
SKIP=go-unit-tests,gosec pre-commit run
```

#### False positives in linting

1. Check if the issue is in generated files (excluded by default)
2. Update `.golangci.yml` to exclude specific rules
3. Use `//nolint:linter-name` comments sparingly

### Performance Tips

1. **Incremental runs**: Pre-commit only runs on changed files by default
2. **Caching**: Most tools cache results between runs
3. **Parallel execution**: Hooks run in parallel when possible
4. **Selective hooks**: Use `SKIP` environment variable for development

## Configuration Customization

### Adding New Hooks

Edit `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/example/repo
    rev: v1.0.0
    hooks:
      - id: example-hook
        args: [--option, value]
```

### Excluding Files

Add patterns to the global `exclude` section:

```yaml
exclude: |
  (?x)^(
    existing_patterns|
    new_pattern\.go$
  )$
```

### Adjusting Linter Settings

Edit `.golangci.yml`:

```yaml
linters-settings:
  lll:
    line-length: 100  # Adjust line length limit
  
  cyclop:
    max-complexity: 10  # Adjust complexity limit
```

## Best Practices

1. **Run hooks frequently**: Use `pre-commit run` during development
2. **Fix issues promptly**: Don't accumulate linting issues
3. **Review auto-fixes**: Always review automated changes before committing
4. **Use appropriate skips**: Only skip hooks when necessary
5. **Keep tools updated**: Regularly run `pre-commit autoupdate`
6. **Configure IDE**: Set up your IDE to run formatters on save

## CI/CD Integration

### GitHub Actions

```yaml
name: Pre-commit
on: [push, pull_request]
jobs:
  pre-commit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - uses: pre-commit/action@v3.0.0
```

### GitLab CI

```yaml
pre-commit:
  image: golang:1.24
  before_script:
    - pip install pre-commit
  script:
    - pre-commit run --all-files
```

## Tool Documentation

- [Pre-commit](https://pre-commit.com/)
- [golangci-lint](https://golangci-lint.run/)
- [gosec](https://github.com/securecodewarrior/gosec)
- [buf](https://docs.buf.build/)
- [detect-secrets](https://github.com/Yelp/detect-secrets)

## Contributing

When contributing to this project:

1. Ensure all pre-commit hooks pass
2. Add new hooks for new file types or quality checks
3. Update documentation when changing configuration
4. Test changes with `pre-commit run --all-files`

## Support

For questions or issues:

1. Check the [troubleshooting section](#troubleshooting)
2. Review tool documentation
3. Open an issue in the repository
4. Ask in the project's communication channels