# Interface{} Migration Tools

This directory contains comprehensive tools and scripts to assist with migrating from `interface{}` usage to type-safe alternatives across the entire codebase.

## Overview

The migration toolkit provides four main tools and three shell scripts to handle different aspects of the migration process:

### 🔧 Core Tools

1. **[AST-based Migration Script](#ast-based-migration-script)** (`migrate_interfaces.go`)
2. **[Interface Usage Analyzer](#interface-usage-analyzer)** (`analyze_interfaces.go`) 
3. **[Type-Safe Code Generator](#type-safe-code-generator)** (`generate_typesafe.go`)
4. **[Migration Validation Tool](#migration-validation-tool)** (`validate_migration.go`)

### 📜 Shell Scripts

1. **[Package Migration Script](#package-migration-script)** (`../scripts/migrate_package.sh`)
2. **[Quick Interface Check](#quick-interface-check)** (`../scripts/check_interfaces.sh`)
3. **[Comprehensive Test Runner](#comprehensive-test-runner)** (`../scripts/run_migration_tests.sh`)

## Quick Start

```bash
# 1. Quick check for interface{} usage
./scripts/check_interfaces.sh

# 2. Analyze detailed patterns
go run tools/analyze_interfaces.go -dir ./pkg/transport -output analysis.json

# 3. Generate type-safe alternatives
go run tools/generate_typesafe.go -config generation_config.json

# 4. Perform migration (dry run first)
./scripts/migrate_package.sh --dry-run ./pkg/transport

# 5. Execute migration with backup
./scripts/migrate_package.sh --execute --backup ./backup ./pkg/transport

# 6. Validate migration
go run tools/validate_migration.go -dir ./pkg/transport

# 7. Run comprehensive tests
./scripts/run_migration_tests.sh
```

## Tool Details

### AST-based Migration Script

**File:** `migrate_interfaces.go`

Automatically migrates `interface{}` usage patterns to type-safe alternatives using AST analysis.

#### Features

- **Automatic Pattern Detection**: Identifies common `interface{}` usage patterns
- **Type Inference**: Suggests appropriate type-safe replacements
- **Dry Run Mode**: Preview changes before applying them
- **Risk Assessment**: Categorizes migrations by risk level (high/medium/low)
- **Comprehensive Reporting**: Generates detailed before/after reports

#### Usage

```bash
# Dry run analysis
go run tools/migrate_interfaces.go -dir ./pkg/transport -dry-run

# Execute migration with logger calls
go run tools/migrate_interfaces.go -dir ./pkg/transport -dry-run=false -logger

# Custom output location
go run tools/migrate_interfaces.go -dir ./pkg/state -output ./reports/state_migration.json
```

#### Supported Patterns

- `Any()` logger calls → `SafeString()`, `SafeInt64()`, etc.
- `map[string]interface{}` → `map[string]any` or typed structs
- `[]interface{}` → typed slices
- Interface{} function parameters → generic constraints

#### Command Line Options

```
-dir string          Directory to process (default ".")
-dry-run             Only analyze without making changes (default true)
-output string       Output file for migration report (default "migration_report.json")
-logger              Enable logger migration (default true)
-maps                Enable map migration (default true)
-params              Enable parameter migration (default false, high risk)
-verbose             Enable verbose logging
-recursive           Process subdirectories recursively (default true)
```

### Interface Usage Analyzer

**File:** `analyze_interfaces.go`

Comprehensive analysis tool that scans the codebase for `interface{}` usage patterns and generates detailed reports.

#### Features

- **Pattern Recognition**: Identifies 9 different `interface{}` usage patterns
- **Risk Assessment**: Categorizes each usage by migration difficulty
- **Package-level Analysis**: Groups results by package for targeted migration
- **Multiple Output Formats**: JSON, text, and CSV formats
- **Code Examples**: Shows actual code snippets for each pattern
- **Migration Suggestions**: Provides specific recommendations for each pattern

#### Usage

```bash
# Basic analysis
go run tools/analyze_interfaces.go -dir ./pkg

# Detailed analysis with examples
go run tools/analyze_interfaces.go -dir ./pkg -examples -max-examples 5

# Generate JSON report for tooling
go run tools/analyze_interfaces.go -dir ./pkg -format json -output analysis.json

# CSV export for spreadsheet analysis
go run tools/analyze_interfaces.go -dir ./pkg -format csv > interface_usage.csv
```

#### Pattern Types Detected

1. **map_string_interface** - `map[string]interface{}` usage
2. **slice_interface** - `[]interface{}` usage  
3. **function_parameter** - `interface{}` as function parameters
4. **function_return** - `interface{}` as return types
5. **struct_field** - `interface{}` as struct fields
6. **json_unmarshal** - JSON unmarshaling to `interface{}`
7. **type_assertion** - Type assertions to `interface{}`
8. **logger_any** - Logger `Any()` calls
9. **empty_interface** - Generic `interface{}` usage

#### Command Line Options

```
-dir string          Root directory to analyze (default ".")
-output string       Output file (default "interface_analysis.json")
-format string       Output format: json, text, csv (default "json")
-examples            Include code examples (default true)
-max-examples int    Maximum examples per pattern (default 5)
-recursive           Analyze subdirectories (default true)
-verbose             Enable verbose logging
```

### Type-Safe Code Generator

**File:** `generate_typesafe.go`

Generates type-safe alternatives including typed structs, wrappers, conversion functions, and test builders.

#### Features

- **Typed Struct Generation**: Creates strongly-typed structs from specifications
- **Type-Safe Wrappers**: Generates wrappers around `interface{}` types
- **Conversion Functions**: Creates bidirectional conversion utilities
- **Test Data Builders**: Generates builder patterns for testing
- **Event Data Structures**: Creates type-safe event structures
- **Validation Methods**: Generates validation logic for created types

#### Usage

```bash
# Generate with default configuration
go run tools/generate_typesafe.go

# Use custom configuration
go run tools/generate_typesafe.go -config my_config.json -output ./generated

# Dry run to preview
go run tools/generate_typesafe.go -dry-run -verbose
```

#### Configuration File Format

The tool uses a JSON configuration file to specify what to generate:

```json
{
  "typed_structs": [
    {
      "name": "UserConfig",
      "description": "user configuration data",
      "fields": [
        {
          "name": "UserID",
          "type": "string",
          "tags": "`json:\"user_id\"`",
          "description": "unique user identifier",
          "validation": ["if s.UserID == \"\" { return fmt.Errorf(\"user_id is required\") }"]
        }
      ],
      "generate_validation": true,
      "generate_conversion": true
    }
  ],
  "typed_wrappers": [...],
  "conversion_functions": [...],
  "test_data_builders": [...],
  "event_data_structures": [...]
}
```

#### Command Line Options

```
-config string       Path to generation configuration (default "generation_config.json")
-output string       Output directory (default "generated")
-package string      Package name for generated code (default "generated")
-dry-run             Show what would be generated without creating files
-overwrite           Overwrite existing files
-verbose             Enable verbose logging
```

### Migration Validation Tool

**File:** `validate_migration.go`

Validates that migrations preserve semantic equivalence and don't introduce regressions.

#### Features

- **Semantic Equivalence**: Compares AST structures before and after migration
- **Test Validation**: Runs the full test suite and reports results
- **Performance Benchmarking**: Compares performance before and after migration
- **Backward Compatibility**: Checks for breaking API changes
- **Static Analysis**: Runs additional safety checks
- **Comprehensive Reporting**: Generates detailed validation reports

#### Usage

```bash
# Basic validation
go run tools/validate_migration.go -dir ./pkg/transport

# With before/after snapshots
go run tools/validate_migration.go \
  -before ./snapshots/before \
  -after ./snapshots/after \
  -dir ./pkg/transport

# Skip time-consuming checks
go run tools/validate_migration.go -dir ./pkg/transport -tests=false -benchmarks=false
```

#### Validation Categories

1. **Semantic Equivalence**: AST-level comparison
2. **Test Results**: Unit and integration test execution
3. **Performance**: Benchmark comparison with acceptable thresholds
4. **Compatibility**: API change detection
5. **Static Analysis**: `go vet` and additional safety checks

#### Command Line Options

```
-dir string                  Root directory to validate (default ".")
-before string               Path to pre-migration snapshot
-after string                Path to post-migration snapshot
-output string               Output file for report (default "validation_report.json")
-tests                       Run test suite (default true)
-benchmarks                  Run performance benchmarks (default true)
-compatibility               Check backward compatibility (default true)
-test-timeout duration       Timeout for tests (default 10m)
-max-slowdown float          Max acceptable performance slowdown % (default 10.0)
-verbose                     Enable verbose logging
```

## Shell Scripts

### Package Migration Script

**File:** `../scripts/migrate_package.sh`

End-to-end migration workflow for a complete Go package.

#### Features

- **Complete Workflow**: Analysis → Generation → Migration → Validation
- **Backup Creation**: Automatic backup before making changes
- **Progress Tracking**: Step-by-step progress with clear feedback
- **Error Handling**: Robust error handling with rollback capabilities
- **Comprehensive Reporting**: Detailed reports at each stage

#### Usage

```bash
# Dry run analysis
./scripts/migrate_package.sh ./pkg/transport

# Execute with backup
./scripts/migrate_package.sh -e -b ./backup ./pkg/transport

# Verbose execution with custom config
./scripts/migrate_package.sh -e -v -c custom.json ./pkg/state
```

### Quick Interface Check

**File:** `../scripts/check_interfaces.sh`

Fast interface{} usage overview for development workflow.

#### Features

- **Quick Feedback**: Fast scan of interface{} patterns
- **Risk Assessment**: Immediate risk level breakdown
- **Multiple Formats**: Text, JSON, CSV, and summary outputs
- **Integration Friendly**: Easy to integrate into CI/CD pipelines

#### Usage

```bash
# Quick check
./scripts/check_interfaces.sh

# Detailed with examples
./scripts/check_interfaces.sh -v -e ./pkg

# JSON output for tooling
./scripts/check_interfaces.sh -f json > usage.json
```

### Comprehensive Test Runner

**File:** `../scripts/run_migration_tests.sh`

Complete test suite for migration validation.

#### Features

- **Multiple Test Types**: Unit, integration, race detection, benchmarks
- **Performance Monitoring**: Benchmark comparison with baselines
- **Coverage Analysis**: Code coverage reporting with thresholds
- **Parallel Execution**: Configurable parallel test execution
- **HTML Reports**: Rich HTML reports with detailed results

#### Usage

```bash
# Run all tests
./scripts/run_migration_tests.sh

# Performance-focused run
./scripts/run_migration_tests.sh -b baseline.txt --benchmark-duration 60s

# Fast feedback (unit tests only)
./scripts/run_migration_tests.sh --no-integration --no-benchmarks --fail-fast
```

## Migration Workflow

### Recommended Migration Process

1. **📊 Assessment Phase**
   ```bash
   # Quick overview
   ./scripts/check_interfaces.sh
   
   # Detailed analysis
   go run tools/analyze_interfaces.go -dir ./pkg -examples
   ```

2. **🎯 Planning Phase**
   ```bash
   # Create generation config based on analysis
   go run tools/generate_typesafe.go -dry-run
   
   # Review and customize generation_config.json
   ```

3. **🔧 Implementation Phase**
   ```bash
   # Dry run migration
   ./scripts/migrate_package.sh --dry-run ./pkg/target
   
   # Execute with backup
   ./scripts/migrate_package.sh -e -b ./backup ./pkg/target
   ```

4. **✅ Validation Phase**
   ```bash
   # Comprehensive testing
   ./scripts/run_migration_tests.sh
   
   # Migration validation
   go run tools/validate_migration.go -dir ./pkg/target
   ```

5. **📋 Review Phase**
   - Review all generated reports
   - Check test coverage and performance
   - Verify backward compatibility
   - Update documentation

### Best Practices

#### 🎯 **Start Small**
- Begin with low-risk packages
- Focus on logger migrations first
- Use dry-run mode extensively

#### 🔒 **Safety First**
- Always create backups before executing
- Run comprehensive tests after migration
- Check performance impact
- Validate semantic equivalence

#### 📈 **Incremental Approach**
- Migrate one package at a time
- Use feature flags for gradual rollout
- Monitor production metrics

#### 🤝 **Team Coordination**
- Share analysis reports with team
- Review migration plans together
- Coordinate breaking changes

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Interface Migration Check
on: [push, pull_request]

jobs:
  migration-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      
      - name: Check interface usage
        run: ./scripts/check_interfaces.sh -f json > interface_usage.json
      
      - name: Run migration tests
        run: ./scripts/run_migration_tests.sh --no-benchmarks
      
      - name: Upload reports
        uses: actions/upload-artifact@v3
        with:
          name: migration-reports
          path: |
            interface_usage.json
            test_results/
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit
set -e

# Check for new interface{} usage
if ./scripts/check_interfaces.sh -f summary | grep -q "interface{} usages"; then
    echo "⚠️  New interface{} usage detected. Consider type-safe alternatives."
    echo "Run './scripts/check_interfaces.sh -v' for details."
fi
```

## Troubleshooting

### Common Issues

#### **"No Go files found"**
- Check directory path
- Ensure you're in a Go project
- Verify file permissions

#### **"Migration validation failed"**
- Review validation report for specific issues
- Check test failures
- Verify performance regressions

#### **"Type inference failed"**
- Manually specify types in generation config
- Use custom conversion functions
- Consider gradual migration

#### **"Tests failing after migration"**
- Check for semantic differences
- Verify type assertions
- Review breaking changes

### Debug Mode

Enable verbose logging for all tools:

```bash
# Detailed logging
go run tools/migrate_interfaces.go -verbose
./scripts/migrate_package.sh -v
./scripts/run_migration_tests.sh -v
```

### Getting Help

1. **Check tool help**: `go run tools/<tool>.go -help`
2. **Review generated reports**: Look for specific error messages
3. **Run validation**: Use validation tool to identify issues
4. **Test incrementally**: Start with smaller, isolated changes

## Contributing

### Adding New Patterns

To add support for new `interface{}` patterns:

1. **Update pattern definitions** in `analyze_interfaces.go`
2. **Add migration logic** in `migrate_interfaces.go`
3. **Create test cases** for the new pattern
4. **Update documentation**

### Improving Type Inference

Enhance type inference by:

1. **Adding more heuristics** in `inferSafeFunction()`
2. **Using static analysis** for better type detection
3. **Integrating with IDE** for context-aware suggestions

### Performance Optimization

Optimize tool performance by:

1. **Parallel processing** of files
2. **Caching AST parsing** results
3. **Incremental analysis** for large codebases

## License

These tools are part of the AG UI Go SDK and follow the same license terms.

---

**📚 For more detailed examples and advanced usage, see the individual tool documentation and the generated sample configurations.**