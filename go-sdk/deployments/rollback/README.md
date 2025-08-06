# Rollback Procedures for AG-UI Go SDK Server Framework

This directory contains comprehensive rollback procedures for the AG-UI Go SDK server framework security enhancements. Use these procedures if critical issues are discovered after deployment.

## 🚨 Emergency Rollback Overview

The security enhancement deployment introduces breaking changes that require careful rollback planning. This document provides step-by-step procedures for different rollback scenarios.

## ⚠️ Before You Begin

**STOP AND ASSESS:**
1. **Document the Issue**: What specific problem requires rollback?
2. **Check Alternatives**: Can the issue be fixed with a hotfix instead?
3. **Impact Analysis**: What systems and users will be affected?
4. **Backup Status**: Confirm you have current backups
5. **Team Notification**: Alert all stakeholders of the rollback

## 🎯 Rollback Scenarios

### Scenario 1: Application Startup Failures
**Symptoms**: Service won't start due to missing environment variables or configuration errors
**Severity**: Critical
**Estimated Rollback Time**: 5-10 minutes

### Scenario 2: Authentication System Failures  
**Symptoms**: Users cannot authenticate, JWT validation failing
**Severity**: Critical
**Estimated Rollback Time**: 10-15 minutes

### Scenario 3: Database Migration Issues
**Symptoms**: Database errors, session management failures
**Severity**: High
**Estimated Rollback Time**: 15-30 minutes

### Scenario 4: Performance Degradation
**Symptoms**: Slow response times, high resource usage
**Severity**: Medium
**Estimated Rollback Time**: 20-45 minutes

## 🔄 Rollback Procedures

### Immediate Rollback (< 5 minutes)

For critical issues requiring immediate action:

```bash
# 1. Stop current application
sudo systemctl stop ag-ui-server

# 2. Switch to previous container/binary
docker tag ag-ui:previous ag-ui:current
# OR
cp /backup/ag-ui-server-previous /usr/local/bin/ag-ui-server

# 3. Revert environment variables (if needed)
cp /backup/environment.previous /etc/ag-ui/environment

# 4. Start previous version
sudo systemctl start ag-ui-server

# 5. Verify service is running
curl -f http://localhost:8080/health
```

### Application-Level Rollback (10-20 minutes)

For issues that require reverting application changes:

```bash
# 1. Execute application rollback script
./deployments/rollback/application-rollback.sh

# 2. Verify rollback success
./scripts/health-check-validation.sh

# 3. Run smoke tests
./deployments/rollback/smoke-tests.sh
```

### Full System Rollback (30-60 minutes)

For complex issues requiring database and infrastructure changes:

```bash
# 1. Execute comprehensive rollback
./deployments/rollback/full-system-rollback.sh

# 2. Validate all systems
./deployments/rollback/post-rollback-validation.sh
```

## 📋 Rollback Scripts

### 1. application-rollback.sh
- Reverts application to previous version
- Updates configuration files
- Restarts services
- Validates basic functionality

### 2. database-rollback.sh
- Reverts database schema changes
- Restores data if needed
- Updates connection strings
- Validates database integrity

### 3. full-system-rollback.sh
- Comprehensive rollback of all components
- Coordinates application and database rollback
- Updates load balancer configuration
- Comprehensive validation

### 4. post-rollback-validation.sh
- Validates all systems after rollback
- Runs health checks
- Verifies user functionality
- Generates rollback report

## 🏗️ Infrastructure Rollback

### Kubernetes Deployment

```bash
# Rollback to previous deployment
kubectl rollout undo deployment/ag-ui-server

# Check rollback status
kubectl rollout status deployment/ag-ui-server

# Verify pods are healthy
kubectl get pods -l app=ag-ui-server
```

### Docker Compose

```bash
# Stop current deployment
docker-compose down

# Switch to previous compose file
cp docker-compose.previous.yml docker-compose.yml

# Start previous version
docker-compose up -d

# Check service health
docker-compose ps
```

### Load Balancer Updates

```bash
# Update upstream servers to point to previous version
# (This is environment-specific - update accordingly)

# Nginx example:
sudo cp /etc/nginx/sites-available/ag-ui.previous /etc/nginx/sites-available/ag-ui
sudo systemctl reload nginx

# HAProxy example:  
sudo cp /etc/haproxy/haproxy.previous.cfg /etc/haproxy/haproxy.cfg
sudo systemctl reload haproxy
```

## 🗄️ Database Rollback Details

### Session Security Rollback

The database migration for session security can be rolled back using:

```sql
-- Use the provided rollback script
SOURCE deployments/migrations/001_rollback_session_security_columns.sql;
```

**Important Notes:**
- This rollback will lose all session security data
- Users may need to re-authenticate
- Monitor for session-related errors after rollback

### Data Preservation Options

If you need to preserve session security data during rollback:

```sql
-- Create backup table before rollback
CREATE TABLE session_security_backup AS 
SELECT session_id, security_hash, encryption_key_id, risk_score, created_at
FROM sessions 
WHERE security_hash IS NOT NULL;

-- After rollback, you can analyze the backup data
-- SELECT * FROM session_security_backup;
```

## 🔍 Validation Procedures

### Post-Rollback Health Checks

```bash
# 1. Basic service health
curl -f http://localhost:8080/health

# 2. Authentication validation
./scripts/auth-testing.sh --base-url=http://localhost:8080

# 3. Database connectivity  
mysql -u user -p database -e "SELECT 1"

# 4. Redis connectivity (if used)
redis-cli ping

# 5. End-to-end testing
./scripts/integration-testing.sh
```

### Performance Validation

```bash
# Load test to ensure performance is restored
ab -n 1000 -c 10 http://localhost:8080/health

# Check response times are acceptable
# Should be similar to pre-deployment metrics
```

### Security Validation

```bash
# Verify no credentials are exposed
./scripts/credential-scan.sh

# Check that basic security features still work
# (This validates the previous version's security)
```

## 📊 Rollback Decision Matrix

| Severity | Time Since Deployment | Recommended Action |
|----------|----------------------|-------------------|
| Critical | < 1 hour | Immediate rollback |
| Critical | 1-4 hours | Evaluate rollback vs hotfix |
| Critical | > 4 hours | Prefer hotfix if possible |
| High | < 1 day | Consider rollback |
| High | > 1 day | Prefer hotfix |
| Medium | Any time | Prefer hotfix |
| Low | Any time | Schedule fix for next release |

## 🎯 Success Criteria for Rollback

Rollback is considered successful when:

- [ ] Application starts and serves traffic normally
- [ ] All health checks pass
- [ ] Authentication works for existing users  
- [ ] Database is accessible and consistent
- [ ] Performance metrics return to baseline
- [ ] No critical errors in logs
- [ ] User-facing functionality works
- [ ] Monitoring systems show green status

## 📈 Monitoring During Rollback

Key metrics to monitor:

- **Error Rate**: Should decrease after rollback
- **Response Time**: Should return to baseline
- **Memory Usage**: Should stabilize
- **Database Connections**: Should be stable
- **Authentication Success Rate**: Should improve
- **User Sessions**: May see temporary disruption

## 🔄 Re-Deployment Planning

After a successful rollback:

1. **Root Cause Analysis**: Understand why rollback was needed
2. **Fix Development**: Develop fixes for the identified issues
3. **Testing Enhancement**: Add tests to prevent similar issues
4. **Staging Validation**: Thoroughly test fixes in staging
5. **Deployment Plan Update**: Update deployment procedures
6. **Team Communication**: Share lessons learned

## 📞 Emergency Contacts

**During Rollback:**
- Database Team: [Contact Info]
- Infrastructure Team: [Contact Info]  
- Security Team: [Contact Info]
- Management: [Contact Info]

**Communication Channels:**
- Incident Channel: #incident-response
- Team Channel: #ag-ui-deployment
- Status Page: [URL]

## 📚 Additional Resources

- [Deployment Guide](../migrations/README.md)
- [Security Review](../../proompts/reviews/matt.spurlin-server-framework-20250806/02-security-review.md)
- [Migration Guide](../../proompts/reviews/matt.spurlin-server-framework-20250806/07-deployment-migration.md)
- [Environment Variable Validation](../../scripts/validate-deployment.sh)

---

## ⚠️ Final Reminders

1. **Always test rollback procedures in non-production first**
2. **Document everything during the rollback process**
3. **Communicate status updates regularly**
4. **Validate thoroughly before declaring rollback complete**
5. **Learn from the incident to prevent future occurrences**

**Remember**: A successful rollback is better than a broken deployment. Don't hesitate to rollback if there's any doubt about system stability or security.