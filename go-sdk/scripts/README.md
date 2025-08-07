# AG-UI Go SDK Deployment Validation Scripts

This directory contains comprehensive deployment validation scripts for the AG-UI Go SDK server framework security enhancements. These scripts ensure deployment readiness and security compliance before production deployment.

## 🎯 Overview

The deployment validation suite addresses the critical path requirements identified in the comprehensive code review:

- **Environment Variable Security**: Validates all required credentials are properly configured
- **Database Migration Readiness**: Ensures session security enhancements are properly implemented
- **Authentication Security**: Tests JWT/HMAC authentication systems thoroughly
- **Integration Validation**: Verifies end-to-end system functionality
- **Credential Security**: Scans for potential credential exposures and security vulnerabilities

## 📋 Scripts Overview

### 🎛️ Master Validation Suite

#### `deploy-validation-suite.sh`
**Primary deployment readiness validation orchestrator**

```bash
# Run complete validation suite
./scripts/deploy-validation-suite.sh

# Run for specific environment
./scripts/deploy-validation-suite.sh --environment=staging

# Skip specific tests
./scripts/deploy-validation-suite.sh --skip-tests=Integration_Tests,Authentication_Tests

# Custom report location
./scripts/deploy-validation-suite.sh --report-dir=/var/reports/deployment-validation
```

**Features:**
- Orchestrates all validation scripts
- Generates comprehensive reports
- Provides deployment readiness assessment
- Creates detailed logs and artifacts
- Returns appropriate exit codes for CI/CD integration

---

### 🔒 Security Validation Scripts

#### `validate-deployment.sh`
**Environment variable and credential validation**

```bash
# Basic validation
./scripts/validate-deployment.sh

# Environment-specific validation
./scripts/validate-deployment.sh --environment=production

# Custom security thresholds
./scripts/validate-deployment.sh --min-key-length=32 --min-entropy=128
```

**Validates:**
- Required environment variables (JWT_SECRET, HMAC_KEY, etc.)
- Minimum key length requirements (32+ characters)
- Credential strength and entropy
- Configuration file security
- Network security settings

#### `credential-scan.sh`
**Comprehensive credential exposure scanning**

```bash
# Scan current directory
./scripts/credential-scan.sh

# Scan specific directory
./scripts/credential-scan.sh --scan-root=/path/to/code

# Generate whitelist template
./scripts/credential-scan.sh --generate-whitelist
```

**Scans for:**
- Hardcoded passwords and API keys
- JWT tokens and access credentials
- Database connection strings
- Weak cryptographic practices
- Configuration security issues
- Potential credential logging

---

### 🏥 Health & Functionality Scripts

#### `health-check-validation.sh`
**Service health and endpoint validation**

```bash
# Basic health checks
./scripts/health-check-validation.sh

# Custom service URL
./scripts/health-check-validation.sh --base-url=https://api.example.com

# Extended timeout and retries
./scripts/health-check-validation.sh --timeout=30 --retries=5
```

**Validates:**
- Health endpoints (/health, /ready, /metrics)
- Service connectivity and responsiveness
- Load balancer compatibility
- Dependency connectivity (Redis, Database)
- Prometheus metrics availability

#### `auth-testing.sh`
**Authentication and authorization security testing**

```bash
# Test authentication system
./scripts/auth-testing.sh

# Custom test credentials
./scripts/auth-testing.sh --test-user=testuser --test-password=testpass123

# Test specific service
./scripts/auth-testing.sh --base-url=https://staging.example.com
```

**Tests:**
- JWT token generation and validation
- Authentication endpoint security
- Protected resource access control
- Token expiration and refresh
- HMAC authentication (if available)
- Rate limiting on authentication attempts

#### `integration-testing.sh`
**End-to-end integration testing**

```bash
# Basic integration tests
./scripts/integration-testing.sh

# Load testing with concurrent users
./scripts/integration-testing.sh --concurrent-users=10 --test-duration=120

# Extended timeout for slow environments
./scripts/integration-testing.sh --timeout=60
```

**Tests:**
- API functionality end-to-end
- Message and event processing
- Session management lifecycle
- WebSocket connectivity
- Server-sent events
- Error handling scenarios
- Concurrent load handling

---

### 🔄 Rollback Scripts

Located in `deployments/rollback/`:

#### `application-rollback.sh`
**Application-level rollback procedures**

```bash
# Standard application rollback
./deployments/rollback/application-rollback.sh

# Force rollback without confirmation
./deployments/rollback/application-rollback.sh --force

# Custom service and backup location
./deployments/rollback/application-rollback.sh --service=my-service --backup-dir=/custom/backup
```

#### `full-system-rollback.sh`
**Complete system rollback including database**

```bash
# Full system rollback (requires confirmation)
./deployments/rollback/full-system-rollback.sh

# Force rollback for emergencies
./deployments/rollback/full-system-rollback.sh --force

# Custom database configuration
./deployments/rollback/full-system-rollback.sh --db-host=db.example.com --db-name=production_db
```

---

## 🚀 Usage Patterns

### Pre-Deployment Validation

```bash
# Complete deployment validation
./scripts/deploy-validation-suite.sh --environment=production

# Quick security-focused validation
./scripts/validate-deployment.sh && ./scripts/credential-scan.sh

# Health check before deployment
./scripts/health-check-validation.sh --base-url=https://staging.example.com
```

### Post-Deployment Validation

```bash
# Verify deployment success
./scripts/health-check-validation.sh --base-url=https://prod.example.com

# Test authentication after deployment
./scripts/auth-testing.sh --base-url=https://prod.example.com

# Full integration test
./scripts/integration-testing.sh --base-url=https://prod.example.com
```

### Emergency Rollback

```bash
# Application-only rollback
./deployments/rollback/application-rollback.sh --force

# Complete system rollback
./deployments/rollback/full-system-rollback.sh --force
```

## 📊 Exit Codes

All scripts use standardized exit codes:

- **0**: Success - All validations passed
- **1**: Warning - Minor issues found, deployment may proceed with caution
- **2**: Critical failure - Deployment should not proceed

## 📁 Report Generation

### Report Structure

```
/tmp/ag-ui-deployment-validation-20250806_143022/
├── deployment-validation-report.md          # Comprehensive markdown report
├── executive-summary.txt                     # Executive summary
├── validation-summary.md                     # Initial report template
├── logs/                                    # Individual test logs
│   ├── Environment_Variables-stdout.log
│   ├── Health_Checks-stderr.log
│   └── ...
├── artifacts/                              # Test artifacts
│   ├── credential-scan-results.txt
│   └── performance-metrics.json
└── ...
```

### Report Content

- **Executive Summary**: High-level pass/fail status
- **Detailed Results**: Individual test outcomes with timing
- **Security Analysis**: Credential scan results and security recommendations  
- **Performance Metrics**: Response times and load test results
- **Deployment Readiness**: Final go/no-go recommendation
- **Troubleshooting**: Links to logs and error details

## 🔧 Configuration

### Environment Variables

```bash
# Required for deployment validation
export JWT_SECRET="your-strong-jwt-secret-min-32-chars"
export HMAC_KEY="your-strong-hmac-key-min-32-chars"
export REDIS_PASSWORD="your-redis-password"
export DB_PASSWORD="your-database-password"

# Optional configuration
export CORS_ALLOWED_ORIGINS="https://yourdomain.com"
export RATE_LIMIT_REQUESTS_PER_SECOND="1000"
export SESSION_TIMEOUT="1800s"

# Database configuration for rollback scripts
export DB_HOST="localhost"
export DB_PORT="3306"
export DB_NAME="ag_ui"
export DB_USER="ag_ui_user"
```

### Script Configuration

Scripts can be configured via command-line arguments or environment variables:

```bash
# Via command line
./scripts/deploy-validation-suite.sh --base-url=https://api.example.com --environment=production

# Via environment variables
export BASE_URL="https://api.example.com"
export ENVIRONMENT="production"
./scripts/deploy-validation-suite.sh
```

## 🎛️ Integration with CI/CD

### GitHub Actions Example

```yaml
name: Deployment Validation
on:
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up environment
        run: |
          echo "JWT_SECRET=${{ secrets.JWT_SECRET }}" >> $GITHUB_ENV
          echo "HMAC_KEY=${{ secrets.HMAC_KEY }}" >> $GITHUB_ENV
          
      - name: Run deployment validation
        run: |
          chmod +x scripts/*.sh
          ./scripts/deploy-validation-suite.sh --environment=production
          
      - name: Upload validation report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: validation-report
          path: /tmp/ag-ui-deployment-validation-*
```

### GitLab CI Example

```yaml
validate-deployment:
  stage: validate
  script:
    - chmod +x scripts/*.sh
    - ./scripts/deploy-validation-suite.sh --environment=$CI_ENVIRONMENT_NAME
  artifacts:
    reports:
      junit: /tmp/ag-ui-deployment-validation-*/junit-report.xml
    paths:
      - /tmp/ag-ui-deployment-validation-*
  only:
    - main
    - develop
```

## 🔍 Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   chmod +x scripts/*.sh
   chmod +x deployments/rollback/*.sh
   ```

2. **Environment Variables Not Set**
   ```bash
   # Check required variables
   ./scripts/validate-deployment.sh --help
   
   # Set missing variables
   export JWT_SECRET="your-secret-here"
   ```

3. **Service Not Accessible**
   ```bash
   # Test connectivity
   curl -v http://localhost:8080/health
   
   # Use correct base URL
   ./scripts/health-check-validation.sh --base-url=https://your-service.com
   ```

4. **Database Connection Issues**
   ```bash
   # Test database connectivity
   mysql -h $DB_HOST -u $DB_USER -p$DB_PASSWORD -e "SELECT 1"
   
   # Update database configuration
   export DB_HOST="your-db-host"
   ```

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
# Enable bash debug mode
bash -x ./scripts/deploy-validation-suite.sh

# Check individual script logs
tail -f /tmp/ag-ui-deployment-validation-*/logs/*.log
```

## 📚 References

- [Security Review](../proompts/reviews/matt.spurlin-server-framework-20250806/02-security-review.md)
- [Deployment Migration Guide](../proompts/reviews/matt.spurlin-server-framework-20250806/07-deployment-migration.md)
- [Database Migrations](../deployments/migrations/README.md)
- [Rollback Procedures](../deployments/rollback/README.md)

## 🆘 Support

For issues with deployment validation:

1. **Check script logs** in the report directory
2. **Review error messages** for specific guidance
3. **Consult troubleshooting section** above
4. **Test in staging environment** before production
5. **Contact development team** with validation report

---

## 🎉 Quick Start

```bash
# 1. Ensure all scripts are executable
chmod +x scripts/*.sh deployments/rollback/*.sh

# 2. Set required environment variables
export JWT_SECRET="your-strong-jwt-secret-min-32-chars"
export HMAC_KEY="your-strong-hmac-key-min-32-chars"
export REDIS_PASSWORD="your-redis-password" 
export DB_PASSWORD="your-database-password"

# 3. Run complete validation suite
./scripts/deploy-validation-suite.sh --environment=production

# 4. Review the generated report
cat /tmp/ag-ui-deployment-validation-*/executive-summary.txt

# 5. Proceed with deployment if all tests pass! 🚀
```

**Remember**: These validation scripts are your safety net. Don't skip them – they prevent production incidents and ensure security compliance!