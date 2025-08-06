-- AG-UI Go SDK Session Security Enhancement Rollback
-- Version: 001_rollback
-- Description: Rollback session security enhancement migration
-- DANGER: This will remove security enhancements and related data

-- =======================
-- ROLLBACK MIGRATION
-- =======================

-- IMPORTANT: Before running this rollback:
-- 1. Ensure application is stopped or in maintenance mode
-- 2. Create a backup of the database
-- 3. Verify this rollback is necessary
-- 4. Document the reason for rollback

-- Start transaction for atomic rollback
BEGIN;

-- Log rollback initiation
INSERT INTO session_audit_log (action, details, created_at) 
VALUES ('SECURITY_ROLLBACK_START', 'Starting rollback of security enhancements', NOW())
ON DUPLICATE KEY UPDATE created_at = NOW();

-- =======================
-- STEP 1: DROP TRIGGERS
-- =======================

-- Drop triggers first to prevent interference during rollback
DROP TRIGGER IF EXISTS sessions_security_hash_trigger;
DROP TRIGGER IF EXISTS sessions_validation_timestamp_trigger;

-- =======================
-- STEP 2: DROP STORED PROCEDURES
-- =======================

DROP PROCEDURE IF EXISTS CleanupExpiredSecureSessions;

-- =======================
-- STEP 3: BACKUP DATA (OPTIONAL)
-- =======================

-- Create backup table for security data before deletion (optional)
-- Uncomment if you want to preserve security data for analysis

/*
CREATE TABLE sessions_security_backup AS
SELECT 
    session_id,
    security_hash,
    encryption_key_id,
    created_ip_hash,
    security_version,
    last_validated_at,
    risk_score,
    NOW() as backed_up_at
FROM sessions 
WHERE security_hash IS NOT NULL;
*/

-- =======================
-- STEP 4: DROP INDEXES
-- =======================

-- Drop all security-related indexes
-- Order matters due to dependencies

DROP INDEX IF EXISTS idx_sessions_security_composite ON sessions;
DROP INDEX IF EXISTS idx_sessions_risk_score ON sessions;
DROP INDEX IF EXISTS idx_sessions_last_validated ON sessions;
DROP INDEX IF EXISTS idx_sessions_security_version ON sessions;
DROP INDEX IF EXISTS idx_sessions_ip_hash ON sessions;
DROP INDEX IF EXISTS idx_sessions_encryption_key ON sessions;
DROP INDEX IF EXISTS idx_sessions_security_hash ON sessions;

-- =======================
-- STEP 5: DROP COLUMNS
-- =======================

-- Remove security enhancement columns
-- Order is important due to potential constraints

ALTER TABLE sessions DROP COLUMN IF EXISTS risk_score;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_validated_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS security_version;
ALTER TABLE sessions DROP COLUMN IF EXISTS created_ip_hash;
ALTER TABLE sessions DROP COLUMN IF EXISTS encryption_key_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS security_hash;

-- =======================
-- STEP 6: CLEANUP AUDIT TABLE
-- =======================

-- Optionally remove the audit log table
-- Comment out if you want to keep audit history

DROP TABLE IF EXISTS session_audit_log;

-- =======================
-- STEP 7: UPDATE MIGRATION RECORD
-- =======================

-- Mark this migration as rolled back
UPDATE schema_migrations 
SET 
    rolled_back_at = NOW(),
    rollback_reason = 'Security enhancement rollback - see deployment logs for details'
WHERE version = '001';

-- Or delete the migration record entirely (more aggressive approach)
-- DELETE FROM schema_migrations WHERE version = '001';

-- =======================
-- STEP 8: VALIDATE ROLLBACK
-- =======================

-- Verify all security columns are removed
SELECT 
    COLUMN_NAME, 
    DATA_TYPE
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE 
    TABLE_NAME = 'sessions' 
    AND COLUMN_NAME IN (
        'security_hash', 
        'encryption_key_id', 
        'created_ip_hash',
        'security_version',
        'last_validated_at',
        'risk_score'
    );

-- This query should return 0 rows if rollback was successful

-- Verify indexes are removed
SELECT 
    INDEX_NAME,
    COLUMN_NAME
FROM INFORMATION_SCHEMA.STATISTICS 
WHERE 
    TABLE_NAME = 'sessions' 
    AND INDEX_NAME LIKE 'idx_sessions_security%'
    OR INDEX_NAME LIKE 'idx_sessions_encryption%'
    OR INDEX_NAME LIKE 'idx_sessions_ip_hash%'
    OR INDEX_NAME LIKE 'idx_sessions_risk%'
    OR INDEX_NAME LIKE 'idx_sessions_last_validated%';

-- This query should return 0 rows if rollback was successful

-- Verify triggers are removed
SELECT 
    TRIGGER_NAME
FROM INFORMATION_SCHEMA.TRIGGERS 
WHERE 
    TRIGGER_SCHEMA = DATABASE()
    AND EVENT_OBJECT_TABLE = 'sessions'
    AND TRIGGER_NAME LIKE '%security%';

-- This query should return 0 rows if rollback was successful

-- Final session count for verification
SELECT 
    COUNT(*) as total_sessions_after_rollback,
    MIN(created_at) as oldest_session,
    MAX(created_at) as newest_session
FROM sessions;

-- Log rollback completion
INSERT INTO migration_log (version, action, details, created_at) 
VALUES ('001', 'ROLLBACK_COMPLETED', 'Session security enhancement rollback completed successfully', NOW())
ON DUPLICATE KEY UPDATE 
    details = 'Session security enhancement rollback completed successfully',
    created_at = NOW();

COMMIT;

-- =======================
-- POST-ROLLBACK STEPS
-- =======================

-- After running this rollback:

-- 1. RESTART APPLICATION with previous version that doesn't use security enhancements
-- 2. VERIFY application functionality
-- 3. MONITOR for any issues
-- 4. UPDATE deployment documentation
-- 5. ANALYZE why rollback was necessary
-- 6. PLAN fixes for re-deployment if needed

-- =======================
-- ROLLBACK VALIDATION QUERIES
-- =======================

-- Run these queries to verify rollback success:

-- 1. Check for any remaining security columns (should be empty)
SELECT 'FAILED: Security columns still exist' as status
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE 
    TABLE_NAME = 'sessions' 
    AND COLUMN_NAME IN (
        'security_hash', 
        'encryption_key_id', 
        'created_ip_hash',
        'security_version',
        'last_validated_at',
        'risk_score'
    )
HAVING COUNT(*) > 0

UNION ALL

SELECT 'SUCCESS: No security columns found' as status
FROM INFORMATION_SCHEMA.COLUMNS 
WHERE 
    TABLE_NAME = 'sessions' 
    AND COLUMN_NAME IN (
        'security_hash', 
        'encryption_key_id', 
        'created_ip_hash',
        'security_version',
        'last_validated_at',
        'risk_score'
    )
HAVING COUNT(*) = 0;

-- 2. Verify table structure is back to original state
DESCRIBE sessions;

-- 3. Test basic session operations
-- (These should be run by application tests, not in SQL)

-- =======================
-- EMERGENCY ROLLBACK NOTES
-- =======================

-- If this script fails partway through:
-- 1. Check which step failed by looking at the error
-- 2. Manually complete the remaining steps
-- 3. Focus on removing columns and indexes that were successfully created
-- 4. Don't worry about triggers/procedures if columns are gone

-- Critical columns to remove in emergency:
-- ALTER TABLE sessions DROP COLUMN security_hash;
-- ALTER TABLE sessions DROP COLUMN encryption_key_id;
-- ALTER TABLE sessions DROP COLUMN created_ip_hash;
-- ALTER TABLE sessions DROP COLUMN security_version;
-- ALTER TABLE sessions DROP COLUMN last_validated_at;
-- ALTER TABLE sessions DROP COLUMN risk_score;

-- =======================
-- ROLLBACK PERFORMANCE NOTES
-- =======================

-- After rollback:
-- 1. ANALYZE TABLE sessions; -- Update table statistics
-- 2. Check query performance has returned to baseline
-- 3. Monitor application performance metrics
-- 4. Verify no security-related queries are failing