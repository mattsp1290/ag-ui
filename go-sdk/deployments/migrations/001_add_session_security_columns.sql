-- AG-UI Go SDK Session Security Enhancement Migration
-- Version: 001
-- Description: Add security columns to sessions table for enhanced security
-- Breaking Change: YES - Requires application restart with new security features

-- =======================
-- FORWARD MIGRATION
-- =======================

-- Start transaction for atomic migration
BEGIN;

-- Add security enhancement columns to sessions table
-- These columns support the new security features from the code review

-- 1. Security hash for integrity validation
ALTER TABLE sessions 
ADD COLUMN security_hash VARCHAR(64) DEFAULT NULL 
COMMENT 'SHA-256 hash for session integrity validation';

-- 2. Encryption key identifier for key rotation support
ALTER TABLE sessions 
ADD COLUMN encryption_key_id VARCHAR(36) DEFAULT NULL 
COMMENT 'UUID of the encryption key used for this session';

-- 3. IP address hash for session binding (privacy-preserving)
ALTER TABLE sessions 
ADD COLUMN created_ip_hash VARCHAR(64) DEFAULT NULL 
COMMENT 'SHA-256 hash of the client IP address at session creation';

-- 4. Enhanced session metadata for security monitoring
ALTER TABLE sessions 
ADD COLUMN security_version INTEGER DEFAULT 1 NOT NULL 
COMMENT 'Version of security implementation used for this session';

-- 5. Last security validation timestamp
ALTER TABLE sessions 
ADD COLUMN last_validated_at TIMESTAMP DEFAULT NULL 
COMMENT 'Timestamp of last security validation';

-- 6. Session risk score (0-100, higher = more risky)
ALTER TABLE sessions 
ADD COLUMN risk_score INTEGER DEFAULT 0 CHECK (risk_score >= 0 AND risk_score <= 100)
COMMENT 'Calculated risk score for this session';

-- Create performance indexes for the new security columns
-- These indexes are critical for performance with the new security features

CREATE INDEX idx_sessions_security_hash ON sessions(security_hash) 
WHERE security_hash IS NOT NULL;

CREATE INDEX idx_sessions_encryption_key ON sessions(encryption_key_id) 
WHERE encryption_key_id IS NOT NULL;

CREATE INDEX idx_sessions_ip_hash ON sessions(created_ip_hash) 
WHERE created_ip_hash IS NOT NULL;

CREATE INDEX idx_sessions_security_version ON sessions(security_version);

CREATE INDEX idx_sessions_last_validated ON sessions(last_validated_at) 
WHERE last_validated_at IS NOT NULL;

CREATE INDEX idx_sessions_risk_score ON sessions(risk_score) 
WHERE risk_score > 0;

-- Composite index for security queries
CREATE INDEX idx_sessions_security_composite ON sessions(security_version, risk_score, last_validated_at) 
WHERE security_hash IS NOT NULL;

-- Update existing sessions with default security metadata
-- This ensures backward compatibility during the migration

UPDATE sessions 
SET 
    security_version = 1,
    last_validated_at = NOW(),
    risk_score = 0,
    -- Generate security hash based on existing session data
    security_hash = SHA2(CONCAT(session_id, COALESCE(created_at, NOW()), COALESCE(user_id, 'anonymous')), 256),
    -- Use migration default encryption key ID
    encryption_key_id = 'migration-default-key'
WHERE 
    security_hash IS NULL 
    AND session_id IS NOT NULL;

-- Create trigger for automatic security hash generation on new sessions
DELIMITER $$
CREATE TRIGGER sessions_security_hash_trigger
    BEFORE INSERT ON sessions
    FOR EACH ROW
BEGIN
    -- Auto-generate security hash if not provided
    IF NEW.security_hash IS NULL THEN
        SET NEW.security_hash = SHA2(CONCAT(
            NEW.session_id, 
            COALESCE(NEW.created_at, NOW()), 
            COALESCE(NEW.user_id, 'anonymous'),
            RAND()
        ), 256);
    END IF;
    
    -- Set default security version if not provided
    IF NEW.security_version IS NULL THEN
        SET NEW.security_version = 1;
    END IF;
    
    -- Set last validated timestamp
    IF NEW.last_validated_at IS NULL THEN
        SET NEW.last_validated_at = NOW();
    END IF;
END$$
DELIMITER ;

-- Create trigger for automatic validation timestamp update
DELIMITER $$
CREATE TRIGGER sessions_validation_timestamp_trigger
    BEFORE UPDATE ON sessions
    FOR EACH ROW
BEGIN
    -- Update last validated timestamp when session is accessed
    IF NEW.last_accessed_at != OLD.last_accessed_at THEN
        SET NEW.last_validated_at = NOW();
    END IF;
END$$
DELIMITER ;

-- Add session cleanup procedure for enhanced security
DELIMITER $$
CREATE PROCEDURE CleanupExpiredSecureSessions()
BEGIN
    DECLARE done INT DEFAULT FALSE;
    DECLARE session_count INT DEFAULT 0;
    
    -- Delete sessions that are expired and have high risk scores
    DELETE FROM sessions 
    WHERE 
        expires_at < NOW() 
        AND (
            risk_score > 50 
            OR last_validated_at < DATE_SUB(NOW(), INTERVAL 1 HOUR)
            OR security_version < 1
        );
    
    SELECT ROW_COUNT() INTO session_count;
    
    -- Log cleanup operation
    INSERT INTO session_audit_log (action, details, created_at) 
    VALUES ('SECURITY_CLEANUP', CONCAT('Cleaned up ', session_count, ' expired/risky sessions'), NOW())
    ON DUPLICATE KEY UPDATE created_at = NOW();
    
END$$
DELIMITER ;

-- Create session audit log table for security monitoring
CREATE TABLE IF NOT EXISTS session_audit_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id VARCHAR(255) DEFAULT NULL,
    action VARCHAR(50) NOT NULL,
    details TEXT DEFAULT NULL,
    ip_hash VARCHAR(64) DEFAULT NULL,
    user_agent_hash VARCHAR(64) DEFAULT NULL,
    risk_score INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_session_audit_session_id (session_id),
    INDEX idx_session_audit_action (action),
    INDEX idx_session_audit_created_at (created_at),
    INDEX idx_session_audit_risk_score (risk_score)
) ENGINE=InnoDB COMMENT='Security audit log for session operations';

-- Insert migration record
INSERT INTO schema_migrations (version, description, applied_at, checksum) 
VALUES (
    '001', 
    'Add session security enhancement columns and indexes', 
    NOW(),
    SHA2(LOAD_FILE('/path/to/this/migration.sql'), 256)
) ON DUPLICATE KEY UPDATE applied_at = NOW();

-- Verify the migration completed successfully
SELECT 
    COUNT(*) as total_sessions,
    COUNT(security_hash) as sessions_with_security_hash,
    COUNT(encryption_key_id) as sessions_with_encryption_key,
    AVG(risk_score) as average_risk_score
FROM sessions;

COMMIT;

-- =======================
-- VALIDATION QUERIES
-- =======================

-- These queries can be run after migration to verify success

-- 1. Verify all new columns exist
SELECT 
    COLUMN_NAME, 
    DATA_TYPE, 
    IS_NULLABLE,
    COLUMN_DEFAULT
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
ORDER BY ORDINAL_POSITION;

-- 2. Verify indexes were created
SELECT 
    INDEX_NAME,
    COLUMN_NAME,
    NON_UNIQUE,
    INDEX_TYPE
FROM INFORMATION_SCHEMA.STATISTICS 
WHERE 
    TABLE_NAME = 'sessions' 
    AND INDEX_NAME LIKE 'idx_sessions_%'
ORDER BY INDEX_NAME, SEQ_IN_INDEX;

-- 3. Verify triggers were created
SELECT 
    TRIGGER_NAME,
    EVENT_MANIPULATION,
    ACTION_TIMING,
    ACTION_STATEMENT
FROM INFORMATION_SCHEMA.TRIGGERS 
WHERE 
    TRIGGER_SCHEMA = DATABASE()
    AND EVENT_OBJECT_TABLE = 'sessions';

-- 4. Verify stored procedures were created
SELECT 
    ROUTINE_NAME,
    ROUTINE_TYPE,
    CREATED,
    LAST_ALTERED
FROM INFORMATION_SCHEMA.ROUTINES 
WHERE 
    ROUTINE_SCHEMA = DATABASE()
    AND ROUTINE_NAME = 'CleanupExpiredSecureSessions';

-- =======================
-- PERFORMANCE NOTES
-- =======================

-- After migration, consider running:
-- 1. ANALYZE TABLE sessions; -- Update table statistics
-- 2. OPTIMIZE TABLE sessions; -- Defragment table if needed
-- 3. Monitor query performance with new indexes
-- 4. Adjust index usage based on actual query patterns

-- =======================
-- ROLLBACK PREPARATION
-- =======================

-- To prepare for rollback, record current state:
-- SELECT COUNT(*) FROM sessions WHERE security_hash IS NOT NULL;
-- This count should match after successful rollback