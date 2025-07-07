# AG-UI State Management Migration Guide

This guide provides comprehensive instructions for migrating from other state management systems to AG-UI State Management, upgrading between versions, and migrating between different storage backends.

## Table of Contents

1. [Migration Overview](#migration-overview)
2. [Pre-Migration Assessment](#pre-migration-assessment)
3. [Migration from Other Systems](#migration-from-other-systems)
4. [Version Upgrades](#version-upgrades)
5. [Storage Backend Migration](#storage-backend-migration)
6. [Data Migration Tools](#data-migration-tools)
7. [Migration Strategies](#migration-strategies)
8. [Testing and Validation](#testing-and-validation)
9. [Rollback Procedures](#rollback-procedures)
10. [Best Practices](#best-practices)

## Migration Overview

The AG-UI State Management system supports various migration scenarios:

- **System Migration**: Moving from Redis, PostgreSQL, or custom state systems
- **Version Upgrade**: Upgrading between AG-UI State Management versions
- **Storage Backend Migration**: Changing storage backends (File ↔ Redis ↔ PostgreSQL)
- **Schema Migration**: Updating state data structures
- **Configuration Migration**: Updating configuration formats

### Migration Process Flow

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              Migration Process                                       │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│  1. Pre-Migration Assessment                                                       │
│     ├── Current System Analysis                                                    │
│     ├── Data Volume Assessment                                                     │
│     ├── Performance Requirements                                                   │
│     └── Compatibility Check                                                        │
│                                                                                     │
│  2. Migration Planning                                                             │
│     ├── Strategy Selection                                                         │
│     ├── Timeline Planning                                                          │
│     ├── Resource Allocation                                                        │
│     └── Risk Assessment                                                            │
│                                                                                     │
│  3. Pre-Migration Setup                                                            │
│     ├── Backup Creation                                                            │
│     ├── Target System Setup                                                        │
│     ├── Migration Tools Preparation                                                │
│     └── Testing Environment Setup                                                  │
│                                                                                     │
│  4. Data Migration                                                                 │
│     ├── Schema Transformation                                                      │
│     ├── Data Export/Import                                                         │
│     ├── Validation & Verification                                                  │
│     └── Performance Testing                                                        │
│                                                                                     │
│  5. System Cutover                                                                 │
│     ├── Maintenance Window                                                         │
│     ├── Final Data Sync                                                            │
│     ├── Application Update                                                         │
│     └── Go-Live Validation                                                         │
│                                                                                     │
│  6. Post-Migration                                                                 │
│     ├── Monitoring Setup                                                           │
│     ├── Performance Validation                                                     │
│     ├── User Acceptance Testing                                                    │
│     └── Legacy System Decommission                                                 │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

## Pre-Migration Assessment

### System Analysis

```go
// Assessment tool for current system
type SystemAssessment struct {
    CurrentSystem    string
    DataVolume       int64
    StateCount       int
    AvgStateSize     int64
    MaxStateSize     int64
    UpdateFrequency  float64
    ReadWriteRatio   float64
    RetentionPeriod  time.Duration
    AvailabilityReq  string
    PerformanceReq   PerformanceRequirements
}

type PerformanceRequirements struct {
    MaxLatency       time.Duration
    MinThroughput    float64
    MaxMemoryUsage   int64
    AvailabilityPct  float64
}

func AssessCurrentSystem() (*SystemAssessment, error) {
    assessment := &SystemAssessment{}
    
    // Analyze current system
    // This would be specific to your current system
    
    return assessment, nil
}
```

### Compatibility Check

```go
// Check compatibility with AG-UI State Management
func CheckCompatibility(assessment *SystemAssessment) (*CompatibilityReport, error) {
    report := &CompatibilityReport{
        Compatible: true,
        Issues:     []string{},
        Recommendations: []string{},
    }
    
    // Check data volume
    if assessment.DataVolume > 100*1024*1024*1024 { // 100GB
        report.Recommendations = append(report.Recommendations, 
            "Consider using PostgreSQL backend for large data volumes")
    }
    
    // Check performance requirements
    if assessment.PerformanceReq.MaxLatency < 10*time.Millisecond {
        report.Recommendations = append(report.Recommendations,
            "Consider using Redis backend for ultra-low latency")
    }
    
    // Check state size
    if assessment.MaxStateSize > 10*1024*1024 { // 10MB
        report.Recommendations = append(report.Recommendations,
            "Enable compression for large state objects")
    }
    
    return report, nil
}

type CompatibilityReport struct {
    Compatible      bool
    Issues          []string
    Recommendations []string
}
```

## Migration from Other Systems

### From Redis

```go
// Redis to AG-UI State Management migration
type RedisMigrator struct {
    sourceClient *redis.Client
    targetManager *state.StateManager
    batchSize    int
}

func NewRedisMigrator(sourceAddr string, targetManager *state.StateManager) *RedisMigrator {
    client := redis.NewClient(&redis.Options{
        Addr: sourceAddr,
    })
    
    return &RedisMigrator{
        sourceClient: client,
        targetManager: targetManager,
        batchSize:    1000,
    }
}

func (m *RedisMigrator) Migrate(ctx context.Context) error {
    // Get all keys
    keys, err := m.sourceClient.Keys(ctx, "*").Result()
    if err != nil {
        return fmt.Errorf("failed to get keys: %w", err)
    }
    
    log.Printf("Migrating %d keys from Redis", len(keys))
    
    // Process in batches
    for i := 0; i < len(keys); i += m.batchSize {
        end := i + m.batchSize
        if end > len(keys) {
            end = len(keys)
        }
        
        batch := keys[i:end]
        if err := m.migrateBatch(ctx, batch); err != nil {
            return fmt.Errorf("failed to migrate batch: %w", err)
        }
        
        log.Printf("Migrated %d/%d keys", end, len(keys))
    }
    
    return nil
}

func (m *RedisMigrator) migrateBatch(ctx context.Context, keys []string) error {
    // Create context for migration
    contextID, err := m.targetManager.CreateContext("migration", map[string]interface{}{
        "migration": true,
        "source":    "redis",
    })
    if err != nil {
        return err
    }
    
    for _, key := range keys {
        // Get value from Redis
        value, err := m.sourceClient.Get(ctx, key).Result()
        if err == redis.Nil {
            continue // Key doesn't exist
        } else if err != nil {
            return err
        }
        
        // Parse JSON value
        var data map[string]interface{}
        if err := json.Unmarshal([]byte(value), &data); err != nil {
            // Handle non-JSON values
            data = map[string]interface{}{
                "value": value,
                "type":  "string",
            }
        }
        
        // Migrate to AG-UI State Management
        _, err = m.targetManager.UpdateState(contextID, key, data, state.UpdateOptions{
            SkipValidation: true, // Skip validation during migration
        })
        if err != nil {
            return fmt.Errorf("failed to update state %s: %w", key, err)
        }
    }
    
    return nil
}
```

### From PostgreSQL

```go
// PostgreSQL to AG-UI State Management migration
type PostgreSQLMigrator struct {
    sourceDB      *sql.DB
    targetManager *state.StateManager
    batchSize     int
}

func NewPostgreSQLMigrator(sourceDB *sql.DB, targetManager *state.StateManager) *PostgreSQLMigrator {
    return &PostgreSQLMigrator{
        sourceDB:      sourceDB,
        targetManager: targetManager,
        batchSize:     1000,
    }
}

func (m *PostgreSQLMigrator) Migrate(ctx context.Context) error {
    // Query to get all state data
    // This would be specific to your current schema
    query := `
        SELECT id, data, created_at, updated_at 
        FROM your_state_table 
        ORDER BY id
    `
    
    rows, err := m.sourceDB.QueryContext(ctx, query)
    if err != nil {
        return fmt.Errorf("failed to query source data: %w", err)
    }
    defer rows.Close()
    
    // Create migration context
    contextID, err := m.targetManager.CreateContext("migration", map[string]interface{}{
        "migration": true,
        "source":    "postgresql",
    })
    if err != nil {
        return err
    }
    
    batch := make([]MigrationRecord, 0, m.batchSize)
    
    for rows.Next() {
        var record MigrationRecord
        var dataJSON string
        
        err := rows.Scan(&record.ID, &dataJSON, &record.CreatedAt, &record.UpdatedAt)
        if err != nil {
            return fmt.Errorf("failed to scan row: %w", err)
        }
        
        // Parse JSON data
        if err := json.Unmarshal([]byte(dataJSON), &record.Data); err != nil {
            return fmt.Errorf("failed to parse JSON for %s: %w", record.ID, err)
        }
        
        batch = append(batch, record)
        
        // Process batch
        if len(batch) >= m.batchSize {
            if err := m.migrateBatch(ctx, contextID, batch); err != nil {
                return err
            }
            batch = batch[:0]
        }
    }
    
    // Process remaining records
    if len(batch) > 0 {
        if err := m.migrateBatch(ctx, contextID, batch); err != nil {
            return err
        }
    }
    
    return nil
}

type MigrationRecord struct {
    ID        string
    Data      map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (m *PostgreSQLMigrator) migrateBatch(ctx context.Context, contextID string, batch []MigrationRecord) error {
    for _, record := range batch {
        // Add migration metadata
        record.Data["_migration"] = map[string]interface{}{
            "original_created_at": record.CreatedAt,
            "original_updated_at": record.UpdatedAt,
            "migrated_at":         time.Now(),
        }
        
        _, err := m.targetManager.UpdateState(contextID, record.ID, record.Data, state.UpdateOptions{
            SkipValidation: true,
        })
        if err != nil {
            return fmt.Errorf("failed to migrate record %s: %w", record.ID, err)
        }
    }
    
    log.Printf("Migrated batch of %d records", len(batch))
    return nil
}
```

### From Custom Systems

```go
// Generic migrator for custom systems
type CustomMigrator struct {
    sourceReader  DataReader
    targetManager *state.StateManager
    transformer   DataTransformer
}

type DataReader interface {
    ReadAll(ctx context.Context) (<-chan Record, error)
    Close() error
}

type DataTransformer interface {
    Transform(record Record) (string, map[string]interface{}, error)
}

type Record struct {
    ID   string
    Data interface{}
    Metadata map[string]interface{}
}

func NewCustomMigrator(reader DataReader, manager *state.StateManager, transformer DataTransformer) *CustomMigrator {
    return &CustomMigrator{
        sourceReader:  reader,
        targetManager: manager,
        transformer:   transformer,
    }
}

func (m *CustomMigrator) Migrate(ctx context.Context) error {
    // Create migration context
    contextID, err := m.targetManager.CreateContext("migration", map[string]interface{}{
        "migration": true,
        "source":    "custom",
    })
    if err != nil {
        return err
    }
    
    // Read records from source
    recordsChan, err := m.sourceReader.ReadAll(ctx)
    if err != nil {
        return fmt.Errorf("failed to read source data: %w", err)
    }
    
    count := 0
    for record := range recordsChan {
        // Transform record
        stateID, stateData, err := m.transformer.Transform(record)
        if err != nil {
            log.Printf("Failed to transform record %s: %v", record.ID, err)
            continue
        }
        
        // Migrate to AG-UI State Management
        _, err = m.targetManager.UpdateState(contextID, stateID, stateData, state.UpdateOptions{
            SkipValidation: true,
        })
        if err != nil {
            return fmt.Errorf("failed to migrate record %s: %w", stateID, err)
        }
        
        count++
        if count%1000 == 0 {
            log.Printf("Migrated %d records", count)
        }
    }
    
    log.Printf("Migration completed: %d records migrated", count)
    return nil
}
```

## Version Upgrades

### Version Compatibility Matrix

```
Source Version  | Target Version | Auto Upgrade | Manual Steps Required
----------------|----------------|--------------|----------------------
1.0.x          | 1.1.x          | Yes          | None
1.0.x          | 2.0.x          | No           | Data format migration
1.1.x          | 1.2.x          | Yes          | None
1.1.x          | 2.0.x          | No           | Data format migration
2.0.x          | 2.1.x          | Yes          | Configuration update
```

### Automatic Version Upgrade

```go
// Automatic upgrade for compatible versions
func AutoUpgrade(dataPath string, fromVersion, toVersion string) error {
    upgrader := &VersionUpgrader{
        DataPath:    dataPath,
        FromVersion: fromVersion,
        ToVersion:   toVersion,
    }
    
    return upgrader.Upgrade()
}

type VersionUpgrader struct {
    DataPath    string
    FromVersion string
    ToVersion   string
}

func (u *VersionUpgrader) Upgrade() error {
    // Check if upgrade is supported
    if !u.isUpgradeSupported() {
        return fmt.Errorf("upgrade from %s to %s not supported", u.FromVersion, u.ToVersion)
    }
    
    // Create backup
    if err := u.createBackup(); err != nil {
        return fmt.Errorf("failed to create backup: %w", err)
    }
    
    // Apply upgrade steps
    steps := u.getUpgradeSteps()
    for _, step := range steps {
        if err := step.Execute(); err != nil {
            return fmt.Errorf("upgrade step failed: %w", err)
        }
    }
    
    // Update version file
    if err := u.updateVersionFile(); err != nil {
        return fmt.Errorf("failed to update version: %w", err)
    }
    
    return nil
}

func (u *VersionUpgrader) isUpgradeSupported() bool {
    // Check compatibility matrix
    compatibleUpgrades := map[string][]string{
        "1.0.0": {"1.1.0", "1.1.1"},
        "1.1.0": {"1.1.1", "1.2.0"},
        "1.1.1": {"1.2.0"},
        "2.0.0": {"2.1.0"},
    }
    
    targets, exists := compatibleUpgrades[u.FromVersion]
    if !exists {
        return false
    }
    
    for _, target := range targets {
        if target == u.ToVersion {
            return true
        }
    }
    
    return false
}
```

### Manual Version Migration

```go
// Manual migration for major version changes
func ManualMigration(sourceConfig, targetConfig *state.ManagerOptions) error {
    // Create source manager
    sourceManager, err := state.NewStateManager(*sourceConfig)
    if err != nil {
        return fmt.Errorf("failed to create source manager: %w", err)
    }
    defer sourceManager.Close()
    
    // Create target manager
    targetManager, err := state.NewStateManager(*targetConfig)
    if err != nil {
        return fmt.Errorf("failed to create target manager: %w", err)
    }
    defer targetManager.Close()
    
    // Migrate data
    migrator := &VersionMigrator{
        source: sourceManager,
        target: targetManager,
    }
    
    return migrator.Migrate(context.Background())
}

type VersionMigrator struct {
    source *state.StateManager
    target *state.StateManager
}

func (m *VersionMigrator) Migrate(ctx context.Context) error {
    // Get all state IDs from source
    stateIDs, err := m.source.ListStates(ctx)
    if err != nil {
        return fmt.Errorf("failed to list states: %w", err)
    }
    
    // Create migration context
    contextID, err := m.target.CreateContext("migration", map[string]interface{}{
        "migration": true,
        "timestamp": time.Now(),
    })
    if err != nil {
        return err
    }
    
    // Migrate each state
    for _, stateID := range stateIDs {
        if err := m.migrateState(ctx, contextID, stateID); err != nil {
            return fmt.Errorf("failed to migrate state %s: %w", stateID, err)
        }
    }
    
    return nil
}

func (m *VersionMigrator) migrateState(ctx context.Context, contextID, stateID string) error {
    // Get state from source
    sourceState, err := m.source.GetState(contextID, stateID)
    if err != nil {
        return err
    }
    
    // Transform state for new version
    targetState := m.transformState(sourceState)
    
    // Set state in target
    _, err = m.target.UpdateState(contextID, stateID, targetState, state.UpdateOptions{
        SkipValidation: true,
    })
    
    return err
}

func (m *VersionMigrator) transformState(sourceState map[string]interface{}) map[string]interface{} {
    // Apply transformations for new version
    targetState := make(map[string]interface{})
    
    // Copy all fields
    for k, v := range sourceState {
        targetState[k] = v
    }
    
    // Apply version-specific transformations
    if version, ok := targetState["version"]; ok {
        if version == "1.0" {
            // Transform from 1.0 to 2.0 format
            targetState["version"] = "2.0"
            targetState["schema_version"] = "2.0.0"
            
            // Move old fields to new structure
            if oldData, ok := targetState["data"]; ok {
                targetState["payload"] = oldData
                delete(targetState, "data")
            }
        }
    }
    
    return targetState
}
```

## Storage Backend Migration

### File to Redis Migration

```go
// Migrate from file storage to Redis
func MigrateFileToRedis(filePath string, redisConfig *state.StorageConfig) error {
    // Create file backend
    fileBackend, err := state.NewFileBackend(&state.StorageConfig{
        Path: filePath,
    })
    if err != nil {
        return err
    }
    defer fileBackend.Close()
    
    // Create Redis backend
    redisBackend, err := state.NewRedisBackend(redisConfig)
    if err != nil {
        return err
    }
    defer redisBackend.Close()
    
    // Migrate data
    migrator := &StorageMigrator{
        source: fileBackend,
        target: redisBackend,
    }
    
    return migrator.Migrate(context.Background())
}

type StorageMigrator struct {
    source state.StorageBackend
    target state.StorageBackend
}

func (m *StorageMigrator) Migrate(ctx context.Context) error {
    // Get all states from source
    states, err := m.source.ListStates(ctx)
    if err != nil {
        return fmt.Errorf("failed to list states: %w", err)
    }
    
    log.Printf("Migrating %d states between storage backends", len(states))
    
    // Migrate each state
    for i, stateID := range states {
        if err := m.migrateState(ctx, stateID); err != nil {
            return fmt.Errorf("failed to migrate state %s: %w", stateID, err)
        }
        
        if (i+1)%100 == 0 {
            log.Printf("Migrated %d/%d states", i+1, len(states))
        }
    }
    
    return nil
}

func (m *StorageMigrator) migrateState(ctx context.Context, stateID string) error {
    // Get state from source
    state, err := m.source.GetState(ctx, stateID)
    if err != nil {
        return err
    }
    
    // Set state in target
    if err := m.target.SetState(ctx, stateID, state); err != nil {
        return err
    }
    
    // Migrate version history
    versions, err := m.source.GetVersionHistory(ctx, stateID, 100)
    if err != nil {
        return err
    }
    
    for _, version := range versions {
        if err := m.target.SaveVersion(ctx, stateID, version); err != nil {
            return err
        }
    }
    
    return nil
}
```

### PostgreSQL to Redis Migration

```go
// Migrate from PostgreSQL to Redis with data transformation
func MigratePostgresToRedis(pgConfig, redisConfig *state.StorageConfig) error {
    // Create PostgreSQL backend
    pgBackend, err := state.NewPostgreSQLBackend(pgConfig)
    if err != nil {
        return err
    }
    defer pgBackend.Close()
    
    // Create Redis backend
    redisBackend, err := state.NewRedisBackend(redisConfig)
    if err != nil {
        return err
    }
    defer redisBackend.Close()
    
    // Create migrator with transformation
    migrator := &TransformingMigrator{
        source:      pgBackend,
        target:      redisBackend,
        transformer: NewPostgresToRedisTransformer(),
    }
    
    return migrator.Migrate(context.Background())
}

type TransformingMigrator struct {
    source      state.StorageBackend
    target      state.StorageBackend
    transformer StateTransformer
}

type StateTransformer interface {
    Transform(state map[string]interface{}) (map[string]interface{}, error)
}

type PostgresToRedisTransformer struct{}

func NewPostgresToRedisTransformer() *PostgresToRedisTransformer {
    return &PostgresToRedisTransformer{}
}

func (t *PostgresToRedisTransformer) Transform(state map[string]interface{}) (map[string]interface{}, error) {
    // Transform PostgreSQL-specific fields for Redis
    transformed := make(map[string]interface{})
    
    for k, v := range state {
        switch k {
        case "created_at", "updated_at":
            // Convert timestamps to strings for Redis
            if t, ok := v.(time.Time); ok {
                transformed[k] = t.Format(time.RFC3339)
            } else {
                transformed[k] = v
            }
        case "metadata":
            // Flatten metadata for Redis
            if meta, ok := v.(map[string]interface{}); ok {
                for mk, mv := range meta {
                    transformed["meta_"+mk] = mv
                }
            }
        default:
            transformed[k] = v
        }
    }
    
    return transformed, nil
}
```

## Data Migration Tools

### Migration Command Line Tool

```go
// Command line migration tool
func main() {
    var (
        sourceType   = flag.String("source", "", "Source system type (redis, postgres, file)")
        targetType   = flag.String("target", "", "Target system type (redis, postgres, file)")
        sourceConfig = flag.String("source-config", "", "Source configuration file")
        targetConfig = flag.String("target-config", "", "Target configuration file")
        batchSize    = flag.Int("batch-size", 1000, "Batch size for migration")
        dryRun       = flag.Bool("dry-run", false, "Perform dry run without actual migration")
    )
    flag.Parse()
    
    if *sourceType == "" || *targetType == "" {
        log.Fatal("Source and target types must be specified")
    }
    
    // Load configurations
    sourceConf, err := loadConfig(*sourceConfig)
    if err != nil {
        log.Fatalf("Failed to load source config: %v", err)
    }
    
    targetConf, err := loadConfig(*targetConfig)
    if err != nil {
        log.Fatalf("Failed to load target config: %v", err)
    }
    
    // Create migrator
    migrator, err := NewMigrator(*sourceType, *targetType, sourceConf, targetConf)
    if err != nil {
        log.Fatalf("Failed to create migrator: %v", err)
    }
    
    // Set batch size
    migrator.SetBatchSize(*batchSize)
    
    // Perform migration
    if *dryRun {
        log.Printf("Performing dry run...")
        if err := migrator.DryRun(context.Background()); err != nil {
            log.Fatalf("Dry run failed: %v", err)
        }
        log.Printf("Dry run completed successfully")
    } else {
        log.Printf("Starting migration...")
        if err := migrator.Migrate(context.Background()); err != nil {
            log.Fatalf("Migration failed: %v", err)
        }
        log.Printf("Migration completed successfully")
    }
}
```

### Migration Validation Tool

```go
// Validation tool to verify migration completeness
type MigrationValidator struct {
    source state.StorageBackend
    target state.StorageBackend
}

func (v *MigrationValidator) Validate(ctx context.Context) (*ValidationReport, error) {
    report := &ValidationReport{
        TotalStates:    0,
        MigratedStates: 0,
        MissingStates:  []string{},
        CorruptedStates: []string{},
        Errors:         []string{},
    }
    
    // Get all states from source
    sourceStates, err := v.source.ListStates(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list source states: %w", err)
    }
    
    report.TotalStates = len(sourceStates)
    
    // Validate each state
    for _, stateID := range sourceStates {
        if err := v.validateState(ctx, stateID, report); err != nil {
            report.Errors = append(report.Errors, fmt.Sprintf("State %s: %v", stateID, err))
        }
    }
    
    return report, nil
}

func (v *MigrationValidator) validateState(ctx context.Context, stateID string, report *ValidationReport) error {
    // Get state from source
    sourceState, err := v.source.GetState(ctx, stateID)
    if err != nil {
        return err
    }
    
    // Get state from target
    targetState, err := v.target.GetState(ctx, stateID)
    if err != nil {
        report.MissingStates = append(report.MissingStates, stateID)
        return nil
    }
    
    // Compare states
    if !compareStates(sourceState, targetState) {
        report.CorruptedStates = append(report.CorruptedStates, stateID)
        return nil
    }
    
    report.MigratedStates++
    return nil
}

type ValidationReport struct {
    TotalStates     int
    MigratedStates  int
    MissingStates   []string
    CorruptedStates []string
    Errors          []string
}

func (r *ValidationReport) String() string {
    return fmt.Sprintf(
        "Validation Report:\n"+
        "  Total States: %d\n"+
        "  Migrated States: %d\n"+
        "  Missing States: %d\n"+
        "  Corrupted States: %d\n"+
        "  Errors: %d\n",
        r.TotalStates, r.MigratedStates, len(r.MissingStates), len(r.CorruptedStates), len(r.Errors))
}
```

## Migration Strategies

### Blue-Green Migration

```go
// Blue-Green migration strategy
type BlueGreenMigration struct {
    blueSystem  *state.StateManager
    greenSystem *state.StateManager
    router      *MigrationRouter
}

func (m *BlueGreenMigration) Execute(ctx context.Context) error {
    // Phase 1: Setup green system
    log.Printf("Phase 1: Setting up green system")
    if err := m.setupGreenSystem(); err != nil {
        return fmt.Errorf("failed to setup green system: %w", err)
    }
    
    // Phase 2: Migrate data
    log.Printf("Phase 2: Migrating data")
    if err := m.migrateData(ctx); err != nil {
        return fmt.Errorf("failed to migrate data: %w", err)
    }
    
    // Phase 3: Sync incremental changes
    log.Printf("Phase 3: Syncing incremental changes")
    if err := m.syncIncrementalChanges(ctx); err != nil {
        return fmt.Errorf("failed to sync changes: %w", err)
    }
    
    // Phase 4: Switch traffic
    log.Printf("Phase 4: Switching traffic")
    if err := m.switchTraffic(); err != nil {
        return fmt.Errorf("failed to switch traffic: %w", err)
    }
    
    // Phase 5: Validate
    log.Printf("Phase 5: Validating")
    if err := m.validateMigration(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    return nil
}

type MigrationRouter struct {
    readFromGreen  bool
    writeToBlue    bool
    writeToGreen   bool
}

func (r *MigrationRouter) RouteRead(stateID string) *state.StateManager {
    if r.readFromGreen {
        return r.greenSystem
    }
    return r.blueSystem
}

func (r *MigrationRouter) RouteWrite(stateID string) []*state.StateManager {
    var targets []*state.StateManager
    
    if r.writeToBlue {
        targets = append(targets, r.blueSystem)
    }
    if r.writeToGreen {
        targets = append(targets, r.greenSystem)
    }
    
    return targets
}
```

### Rolling Migration

```go
// Rolling migration strategy
type RollingMigration struct {
    sourceManager *state.StateManager
    targetManager *state.StateManager
    migrationRate int // states per second
}

func (m *RollingMigration) Execute(ctx context.Context) error {
    // Get all states to migrate
    states, err := m.sourceManager.ListStates(ctx)
    if err != nil {
        return fmt.Errorf("failed to list states: %w", err)
    }
    
    // Calculate migration batches
    batchSize := m.migrationRate
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for i := 0; i < len(states); i += batchSize {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // Migrate batch
            end := i + batchSize
            if end > len(states) {
                end = len(states)
            }
            
            batch := states[i:end]
            if err := m.migrateBatch(ctx, batch); err != nil {
                return fmt.Errorf("failed to migrate batch: %w", err)
            }
            
            log.Printf("Migrated %d/%d states", end, len(states))
        }
    }
    
    return nil
}
```

## Testing and Validation

### Migration Testing Framework

```go
// Migration testing framework
type MigrationTest struct {
    name        string
    sourceData  map[string]interface{}
    expectedData map[string]interface{}
    migrator    Migrator
}

func (t *MigrationTest) Run() error {
    // Setup test environment
    sourceManager := createTestSourceManager()
    targetManager := createTestTargetManager()
    
    // Populate source data
    contextID, _ := sourceManager.CreateContext("test", nil)
    for stateID, data := range t.sourceData {
        sourceManager.UpdateState(contextID, stateID, data, state.UpdateOptions{})
    }
    
    // Run migration
    migrator := NewMigrator(sourceManager, targetManager)
    if err := migrator.Migrate(context.Background()); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }
    
    // Validate results
    for stateID, expectedData := range t.expectedData {
        actualData, err := targetManager.GetState(contextID, stateID)
        if err != nil {
            return fmt.Errorf("failed to get migrated state: %w", err)
        }
        
        if !reflect.DeepEqual(expectedData, actualData) {
            return fmt.Errorf("data mismatch for state %s", stateID)
        }
    }
    
    return nil
}

// Test suite
func RunMigrationTests() error {
    tests := []MigrationTest{
        {
            name: "Basic state migration",
            sourceData: map[string]interface{}{
                "user-1": map[string]interface{}{
                    "name": "John Doe",
                    "age":  30,
                },
            },
            expectedData: map[string]interface{}{
                "user-1": map[string]interface{}{
                    "name": "John Doe",
                    "age":  30,
                },
            },
        },
        // Add more test cases...
    }
    
    for _, test := range tests {
        log.Printf("Running test: %s", test.name)
        if err := test.Run(); err != nil {
            return fmt.Errorf("test %s failed: %w", test.name, err)
        }
    }
    
    return nil
}
```

### Performance Testing

```go
// Performance testing for migration
func BenchmarkMigration(b *testing.B) {
    // Setup test data
    sourceManager := createTestSourceManager()
    targetManager := createTestTargetManager()
    
    // Create test data
    contextID, _ := sourceManager.CreateContext("bench", nil)
    for i := 0; i < 10000; i++ {
        stateID := fmt.Sprintf("state-%d", i)
        data := map[string]interface{}{
            "id":    stateID,
            "value": i,
            "data":  strings.Repeat("x", 1000), // 1KB of data
        }
        sourceManager.UpdateState(contextID, stateID, data, state.UpdateOptions{})
    }
    
    // Benchmark migration
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        migrator := NewMigrator(sourceManager, targetManager)
        if err := migrator.Migrate(context.Background()); err != nil {
            b.Fatal(err)
        }
    }
}
```

## Rollback Procedures

### Automatic Rollback

```go
// Automatic rollback on migration failure
type SafeMigration struct {
    migrator Migrator
    backup   BackupManager
}

func (s *SafeMigration) Execute(ctx context.Context) error {
    // Create backup
    backupID, err := s.backup.CreateBackup(ctx)
    if err != nil {
        return fmt.Errorf("failed to create backup: %w", err)
    }
    
    // Attempt migration
    if err := s.migrator.Migrate(ctx); err != nil {
        log.Printf("Migration failed: %v", err)
        log.Printf("Initiating rollback...")
        
        // Rollback to backup
        if rollbackErr := s.backup.RestoreBackup(ctx, backupID); rollbackErr != nil {
            return fmt.Errorf("migration failed and rollback failed: %w", rollbackErr)
        }
        
        return fmt.Errorf("migration failed, rolled back to backup: %w", err)
    }
    
    // Clean up backup if migration successful
    if err := s.backup.DeleteBackup(ctx, backupID); err != nil {
        log.Printf("Warning: failed to clean up backup: %v", err)
    }
    
    return nil
}
```

### Manual Rollback

```go
// Manual rollback procedures
func RollbackMigration(backupID string) error {
    backup := NewBackupManager()
    
    // Verify backup exists
    if !backup.BackupExists(backupID) {
        return fmt.Errorf("backup %s does not exist", backupID)
    }
    
    // Stop current system
    log.Printf("Stopping current system...")
    if err := stopCurrentSystem(); err != nil {
        return fmt.Errorf("failed to stop current system: %w", err)
    }
    
    // Restore from backup
    log.Printf("Restoring from backup %s...", backupID)
    if err := backup.RestoreBackup(context.Background(), backupID); err != nil {
        return fmt.Errorf("failed to restore backup: %w", err)
    }
    
    // Start system
    log.Printf("Starting system...")
    if err := startSystem(); err != nil {
        return fmt.Errorf("failed to start system: %w", err)
    }
    
    // Validate system
    log.Printf("Validating system...")
    if err := validateSystem(); err != nil {
        return fmt.Errorf("system validation failed: %w", err)
    }
    
    log.Printf("Rollback completed successfully")
    return nil
}
```

## Best Practices

### Migration Checklist

1. **Pre-Migration**
   - [ ] Create full backup of current system
   - [ ] Test migration in staging environment
   - [ ] Validate migration scripts
   - [ ] Prepare rollback procedures
   - [ ] Schedule maintenance window
   - [ ] Notify stakeholders

2. **During Migration**
   - [ ] Monitor migration progress
   - [ ] Verify data integrity
   - [ ] Check system performance
   - [ ] Validate functionality
   - [ ] Monitor error rates

3. **Post-Migration**
   - [ ] Validate all data migrated
   - [ ] Test system functionality
   - [ ] Monitor performance
   - [ ] Update documentation
   - [ ] Train operators
   - [ ] Clean up old system

### Common Pitfalls to Avoid

1. **Insufficient Testing**
   - Always test migration in staging environment
   - Test with production-like data volumes
   - Validate all edge cases

2. **No Rollback Plan**
   - Always have a rollback plan
   - Test rollback procedures
   - Keep backups until migration is fully validated

3. **Ignoring Data Validation**
   - Validate data integrity after migration
   - Check for data corruption
   - Verify all relationships are preserved

4. **Insufficient Resources**
   - Allocate sufficient time for migration
   - Ensure adequate hardware resources
   - Have experienced team members available

5. **Poor Communication**
   - Communicate migration schedule clearly
   - Provide regular updates during migration
   - Document all changes and issues

This migration guide provides comprehensive procedures for moving to AG-UI State Management from various source systems, ensuring data integrity and minimal downtime during the transition process.