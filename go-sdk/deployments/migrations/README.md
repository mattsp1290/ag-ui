# Database Migration Scripts

This directory contains database migration scripts for the AG-UI Go SDK server framework security enhancements.

## Overview

These migrations implement the session security enhancements identified in the comprehensive code review, addressing P0 critical security vulnerabilities and implementing enhanced session management capabilities.

## Migration Files

### 001_add_session_security_columns.sql
- **Purpose**: Adds security enhancement columns to the sessions table
- **Breaking Change**: YES - Requires application restart with new security features enabled
- **Key Changes**:
  - Adds `security_hash` column for session integrity validation
  - Adds `encryption_key_id` for key rotation support
  - Adds `created_ip_hash` for session binding (privacy-preserving)
  - Adds `security_version` for implementation versioning
  - Adds `last_validated_at` for security monitoring
  - Adds `risk_score` for session risk assessment
  - Creates performance indexes for new columns
  - Adds triggers for automatic security hash generation
  - Creates cleanup procedures for expired/risky sessions
  - Creates audit log table for security monitoring

### 001_rollback_session_security_columns.sql
- **Purpose**: Rollback script for the security enhancement migration
- **Use Case**: Emergency rollback if security features cause issues
- **Warning**: ⚠️ This will remove all security enhancements and related data

## Migration Execution

### Prerequisites

1. **Database Backup**: Always create a full database backup before migration
2. **Application Downtime**: Plan for application maintenance window
3. **Environment Variables**: Ensure all security environment variables are configured
4. **Monitoring**: Have database and application monitoring ready

### Forward Migration

```bash
# 1. Backup database
mysqldump -u username -p database_name > backup_before_migration_$(date +%Y%m%d_%H%M%S).sql

# 2. Run migration
mysql -u username -p database_name < deployments/migrations/001_add_session_security_columns.sql

# 3. Verify migration
mysql -u username -p database_name -e "
SELECT COUNT(*) FROM sessions WHERE security_hash IS NOT NULL;
SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME='sessions' AND COLUMN_NAME IN ('security_hash','encryption_key_id');
"

# 4. Start application with new security features enabled
```

### Rollback Migration (Emergency Use)

```bash
# 1. Stop application immediately
systemctl stop ag-ui-server

# 2. Run rollback
mysql -u username -p database_name < deployments/migrations/001_rollback_session_security_columns.sql

# 3. Verify rollback
mysql -u username -p database_name -e "
SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME='sessions' AND COLUMN_NAME IN ('security_hash','encryption_key_id');
"

# 4. Start application with previous version (without security features)
```

## Database Compatibility

### Supported Database Systems

- **MySQL 8.0+**: Fully supported (primary target)
- **MySQL 5.7**: Supported with minor syntax adjustments
- **PostgreSQL**: Requires syntax conversion (see PostgreSQL section)
- **SQLite**: Not recommended for production (limited features)

### PostgreSQL Conversion

For PostgreSQL deployments, key syntax differences:

```sql
-- MySQL: SHA2(data, 256)
-- PostgreSQL: encode(digest(data, 'sha256'), 'hex')

-- MySQL: AUTO_INCREMENT
-- PostgreSQL: SERIAL or BIGSERIAL

-- MySQL: ENGINE=InnoDB
-- PostgreSQL: (not needed)

-- MySQL: DELIMITER $$
-- PostgreSQL: Not needed for functions
```

## Security Considerations

### Data Protection
- All hash values use SHA-256 for cryptographic security
- IP addresses are hashed (not stored in plaintext) for privacy
- Encryption key IDs reference external key management
- Session data integrity is protected by security hashes

### Performance Impact
- New indexes optimize security query performance
- Risk scoring enables proactive session management
- Cleanup procedures prevent data accumulation
- Triggers add minimal overhead to session operations

### Monitoring
- Audit log captures all security events
- Risk scores enable automated threat detection
- Validation timestamps track session health
- Migration metrics available for monitoring

## Validation Procedures

### Post-Migration Validation

```sql
-- 1. Verify all columns exist
SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE 
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE TABLE_NAME = 'sessions' 
AND COLUMN_NAME LIKE '%security%' OR COLUMN_NAME LIKE '%risk%';

-- 2. Check index creation
SHOW INDEX FROM sessions WHERE Key_name LIKE 'idx_sessions_%';

-- 3. Validate data migration
SELECT 
    COUNT(*) as total_sessions,
    COUNT(security_hash) as sessions_with_hash,
    AVG(risk_score) as avg_risk_score
FROM sessions;

-- 4. Test trigger functionality
INSERT INTO sessions (session_id, user_id) VALUES ('test-session', 'test-user');
SELECT security_hash, security_version FROM sessions WHERE session_id = 'test-session';
DELETE FROM sessions WHERE session_id = 'test-session';
```

## Troubleshooting

### Common Issues

1. **Migration Timeout**
   - Large tables may require increased timeout: `SET SESSION innodb_lock_wait_timeout=300;`
   - Consider running during low-traffic periods

2. **Index Creation Slow**
   - Indexes are created with `WHERE` clauses to optimize storage
   - Monitor `SHOW PROCESSLIST` during migration

3. **Trigger Conflicts**
   - Existing triggers may conflict - review before migration
   - Backup existing triggers if customizations exist

4. **Memory Usage**
   - Migration may temporarily increase memory usage
   - Monitor database server resources

### Recovery Procedures

1. **Partial Migration Failure**
   - Check which step failed in migration log
   - Manually complete remaining steps
   - Validate data integrity before proceeding

2. **Application Compatibility Issues**
   - Use rollback script to revert changes
   - Update application code to handle new columns
   - Re-run migration after fixes

3. **Performance Degradation**
   - Run `ANALYZE TABLE sessions` to update statistics
   - Monitor query execution plans
   - Adjust indexes based on actual usage patterns

## Integration with Application

### Environment Variables Required

The migration prepares the database for these security features:

```bash
JWT_SECRET="your-strong-jwt-secret-min-32-chars"
HMAC_KEY="your-strong-hmac-key-min-32-chars"
REDIS_PASSWORD="your-redis-password"
DB_PASSWORD="your-database-password"
```

### Application Code Changes

After migration, the application will use:
- Enhanced session security validation
- Risk-based session management  
- Automatic cleanup of compromised sessions
- Comprehensive security audit logging

### Monitoring Integration

Monitor these new metrics after migration:
- Average session risk scores
- Security validation frequency
- Cleanup operation effectiveness
- Audit log growth rate

## Maintenance

### Regular Maintenance Tasks

1. **Cleanup Old Audit Logs**
   ```sql
   DELETE FROM session_audit_log 
   WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
   ```

2. **Update Session Risk Scores**
   ```sql
   CALL UpdateSessionRiskScores(); -- Custom procedure to implement
   ```

3. **Archive Old Security Data**
   ```sql
   -- Create archive table for old sessions with security data
   INSERT INTO sessions_archive 
   SELECT * FROM sessions 
   WHERE expires_at < DATE_SUB(NOW(), INTERVAL 90 DAY);
   ```

### Performance Monitoring

Monitor these queries for performance:
- Session lookup by security_hash
- Risk score filtering operations  
- Audit log insertions
- Cleanup procedure execution time

## Support

For issues with these migrations:
1. Check the troubleshooting section above
2. Review database error logs
3. Validate environment configuration
4. Test in non-production environment first
5. Contact development team with specific error messages and context