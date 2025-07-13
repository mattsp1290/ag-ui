# AG-UI Go SDK - Comprehensive Linting System

This document describes the comprehensive linting and static analysis system designed to detect deprecated `interface{}` usage and guide developers toward type-safe alternatives.

## 🎯 Overview

The linting system provides multiple layers of protection against type safety violations:

1. **Real-time IDE warnings** - Immediate feedback during development
2. **Pre-commit hooks** - Block problematic commits 
3. **CI/CD integration** - Enforce standards in automation
4. **Migration tools** - Help transition to type-safe alternatives
5. **Documentation generation** - Provide clear guidance and examples

## 📁 Components

### 1. golangci-lint Configuration (`.golangci.yml`)

Enhanced configuration with custom rules for interface{} detection:

- **forbidigo linter**: Detects and forbids specific patterns
- **Custom patterns**: Matches various interface{} usage patterns
- **Exclusion rules**: Allows legitimate uses (JSON, reflection)
- **Severity levels**: Error for new code, warnings for legacy

**Key Rules:**
- `interface{}` → Use specific types or generics
- `[]interface{}` → Use typed slices
- `map[string]interface{}` → Use structs or typed maps
- `.Any()` logger calls → Use typed methods

### 2. Custom Static Analyzer (`lint/typesafety_analyzer.go`)

Go analysis framework-based analyzer for deep code inspection:

**Features:**
- Context-aware pattern detection
- Specific fix suggestions
- Auto-fix capabilities where safe
- Integration with IDE and CI tools

**Detection Categories:**
- Empty interface usage
- Unsafe type assertions
- Function parameters/returns
- Struct field types
- Collection types

### 3. Migration Rules Analyzer (`lint/migration_rules.go`)

Tracks migration progress and detects inconsistent patterns:

**Capabilities:**
- Project-wide migration status
- Mixed usage detection (legacy + modern in same file)
- Priority-based migration suggestions
- Incomplete migration patterns
- Migration planning and reporting

### 4. Pre-commit Hooks (`scripts/pre-commit-hooks/`)

Three specialized scripts for different phases:

#### `check-typesafety.sh`
- Blocks commits with new interface{} usage
- Provides immediate feedback with context
- Suggests specific alternatives
- Respects exclusion patterns

#### `suggest-alternatives.sh`
- Analyzes existing code for improvement opportunities
- Generates detailed fix suggestions with examples
- Creates fix templates for common patterns
- Provides context-specific recommendations

#### `format-migration.sh`
- Auto-formats migrated code
- Applies safe transformations (interface{} → any)
- Validates syntax after changes
- Runs tests to ensure correctness

### 5. VS Code Integration (`.vscode/`)

Complete IDE integration for seamless development:

#### `settings.json`
- Real-time linting with golangci-lint
- Custom problem matchers for interface{} detection
- Enhanced Go language server configuration
- Type safety highlighting and warnings

#### `go.code-snippets`
- Type-safe code snippets
- Generic function templates
- Safe type assertion patterns
- Structured data alternatives

**Available Snippets:**
- `genfunc` - Generic function template
- `genslice` - Generic slice processing
- `typedstruct` - Typed struct for JSON
- `safeassert` - Safe type assertion
- `typeswitch` - Type switch pattern
- `typedlog` - Typed logger calls

### 6. Documentation Generator (`lint/generate_rules_doc.go`)

Automated documentation generation:

**Generated Documentation:**
- Comprehensive rule descriptions
- Before/after code examples
- Migration guides
- IDE integration instructions
- Troubleshooting guides

## 🚀 Quick Start

### 1. Install Tools
```bash
make tools-install
```

### 2. Run Type Safety Check
```bash
make lint-typesafety
```

### 3. Get Migration Suggestions
```bash
make lint-suggest
```

### 4. Format Migrated Code
```bash
make lint-format-migration
```

### 5. Generate Documentation
```bash
make lint-docs
```

## 🔧 Makefile Targets

| Target | Description |
|--------|-------------|
| `lint` | Run standard golangci-lint |
| `lint-typesafety` | Check for interface{} violations |
| `lint-migration` | Analyze migration progress |
| `lint-suggest` | Get type-safe alternatives |
| `lint-format-migration` | Auto-format migrated code |
| `lint-docs` | Generate documentation |
| `lint-all` | Run all linting checks |
| `pre-commit` | Pre-commit checks with type safety |
| `pre-commit-full` | Comprehensive pre-commit checks |

## 📋 Configuration

### Priority Paths
High-priority packages for migration:
- `pkg/messages/`
- `pkg/state/`
- `pkg/transport/`
- `pkg/events/`
- `pkg/client/`
- `pkg/server/`

### Allowed Patterns
Legitimate interface{} usage:
- JSON marshal/unmarshal operations
- context.WithValue calls
- Reflection operations
- fmt.Printf family functions
- Test files (more flexible)

### Excluded Files
- `*.pb.go` (protobuf generated)
- `*_test.go` (test files)
- `vendor/` (dependencies)
- `scripts/` (build scripts)

## 🎨 IDE Support

### VS Code
- Automatic configuration via `.vscode/settings.json`
- Real-time warnings and suggestions
- Integrated tasks for type safety checks
- Code snippets for type-safe patterns

### GoLand/IntelliJ
- Configure golangci-lint with project config
- File watchers for automatic checking
- Custom inspection profiles

### Vim/Neovim
- vim-go integration with golangci-lint
- LSP configuration for real-time checks

## 🔄 CI/CD Integration

### GitHub Actions
```yaml
- name: Type Safety Check
  run: make lint-all
```

### GitLab CI
```yaml
lint:
  script:
    - make lint-all
```

### Pre-commit Framework
```yaml
repos:
  - repo: local
    hooks:
      - id: type-safety
        name: Type Safety Check
        entry: make lint-typesafety
        language: system
        files: \.go$
```

## 📊 Migration Strategy

### Phase 1: Assessment
1. Run migration analyzer: `make lint-migration`
2. Identify high-priority files
3. Understand current patterns

### Phase 2: Planning
1. Prioritize by impact and complexity
2. Plan incremental migration
3. Set up monitoring

### Phase 3: Implementation
1. Fix high-priority files first
2. Use suggestion tools
3. Apply auto-formatting
4. Validate with tests

### Phase 4: Monitoring
1. Track progress with migration analyzer
2. Prevent regressions with pre-commit hooks
3. Update documentation

## 🔍 Common Patterns and Fixes

### Interface{} Function Parameters
```go
// Before
func ProcessData(data interface{}) error

// After - Specific Type
func ProcessData(data UserConfig) error

// After - Generics
func ProcessData[T any](data T) error
```

### Map[string]interface{} for Configuration
```go
// Before
config := map[string]interface{}{
    "host": "localhost",
    "port": 8080,
}

// After
type Config struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}
config := Config{Host: "localhost", Port: 8080}
```

### []interface{} Collections
```go
// Before
items := []interface{}{"hello", 42, true}

// After - Separate by Type
strings := []string{"hello"}
numbers := []int{42}
flags := []bool{true}

// After - Generics
func Process[T any](items []T) []T
```

### Logger .Any() Calls
```go
// Before
logger.Info("message", zap.Any("key", value))

// After
logger.Info("message", zap.String("key", stringValue))
logger.Info("message", zap.Int("key", intValue))
```

## 🐛 Troubleshooting

### Common Issues

1. **False Positives**
   - Add exclusions to `.golangci.yml`
   - Use `//nolint:forbidigo` comments

2. **Performance Issues**
   - Increase timeout: `--timeout=10m`
   - Run on specific packages only

3. **Migration Failures**
   - Check type constraints
   - Verify generic usage
   - Update tests

### Debug Mode
```bash
golangci-lint run --config=.golangci.yml --verbose
```

### Getting Help
1. Check generated documentation
2. Use suggestion tools
3. Review examples
4. Contact development team

## 📚 Additional Resources

- [Go Generics Tutorial](https://go.dev/doc/tutorial/generics)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [golangci-lint Documentation](https://golangci-lint.run/)
- [Go Analysis Framework](https://pkg.go.dev/golang.org/x/tools/go/analysis)

## 🎉 Benefits

### Type Safety
- Compile-time error detection
- Reduced runtime panics
- Better API documentation

### Performance
- Eliminated boxing/unboxing
- More efficient memory usage
- Better inlining opportunities

### Maintainability
- Self-documenting code
- Better IDE support
- Easier refactoring

### Developer Experience
- Real-time feedback
- Automated suggestions
- Consistent patterns

## 🤝 Contributing

When contributing to the linting system:

1. Test new rules thoroughly
2. Provide clear documentation
3. Include examples and fixes
4. Update integration tests
5. Ensure backward compatibility

## 📄 License

This linting system is part of the AG-UI Go SDK and follows the same license terms.