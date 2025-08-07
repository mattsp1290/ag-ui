#!/bin/bash

# AG-UI Go SDK - Credential Scanning Validation Script
# Scans codebase for potential credential leaks and security vulnerabilities

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCAN_ROOT="${SCAN_ROOT:-$(pwd)}"
EXCLUDE_DIRS=".git node_modules vendor build dist tmp .idea .vscode coverage"
EXCLUDE_FILES="*.log *.tmp *.cache *_test.go *test*.json *.md"
OUTPUT_FILE="${OUTPUT_FILE:-/tmp/credential-scan-$(date +%Y%m%d_%H%M%S).txt}"

# Scanning results
CREDENTIAL_ISSUES=0
SECURITY_WARNINGS=0
FILES_SCANNED=0
FINDINGS=()

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ((CREDENTIAL_ISSUES++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((SECURITY_WARNINGS++))
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Function to add finding to results
add_finding() {
    local severity="$1"
    local file="$2"
    local line="$3"
    local pattern="$4"
    local content="$5"
    
    FINDINGS+=("$severity:$file:$line:$pattern:$content")
    
    case $severity in
        CRITICAL) 
            error "CRITICAL: $file:$line - $pattern"
            error "  Content: ${content:0:80}..."
            ;;
        HIGH)
            error "HIGH: $file:$line - $pattern"
            error "  Content: ${content:0:80}..."
            ;;
        MEDIUM)
            warn "MEDIUM: $file:$line - $pattern"
            warn "  Content: ${content:0:80}..."
            ;;
        LOW)
            warn "LOW: $file:$line - $pattern"
            ;;
    esac
}

# Function to build find command with exclusions
build_find_command() {
    local find_cmd="find '$SCAN_ROOT' -type f"
    
    # Exclude directories
    for dir in $EXCLUDE_DIRS; do
        find_cmd="$find_cmd -not -path '*/$dir/*'"
    done
    
    # Include specific file types for source code
    find_cmd="$find_cmd \\( -name '*.go' -o -name '*.py' -o -name '*.js' -o -name '*.ts' -o -name '*.java' -o -name '*.cpp' -o -name '*.c' -o -name '*.h' -o -name '*.hpp' -o -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.toml' -o -name '*.ini' -o -name '*.conf' -o -name '*.config' -o -name '*.env*' -o -name 'Dockerfile*' -o -name '*.sh' -o -name '*.bash' -o -name '*.zsh' -o -name 'Makefile*' \\)"
    
    echo "$find_cmd"
}

# Function to scan for hardcoded secrets and credentials
scan_hardcoded_credentials() {
    log "Scanning for hardcoded credentials..."
    
    # Define credential patterns with their severity levels
    declare -A credential_patterns=(
        # High-severity patterns (likely credentials)
        ["password[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9!@#$%^&*()_+\\-=\\[\\]{};':\"\\|,.<>?/~`]{8,}"]="HIGH:Hardcoded password"
        ["secret[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9!@#$%^&*()_+\\-=\\[\\]{};':\"\\|,.<>?/~`]{8,}"]="HIGH:Hardcoded secret"
        ["api[_\\-]?key[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9\\-_]{20,}"]="HIGH:Hardcoded API key"
        ["access[_\\-]?token[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9\\-_.]{20,}"]="HIGH:Hardcoded access token"
        
        # Critical patterns (definitely credentials)
        ["-----BEGIN [A-Z ]+ PRIVATE KEY-----"]="CRITICAL:Private key in plaintext"
        ["sk-[a-zA-Z0-9]{20,}"]="CRITICAL:OpenAI API key"
        ["AIza[0-9A-Za-z\\-_]{35}"]="CRITICAL:Google API key"
        ["AKIA[0-9A-Z]{16}"]="CRITICAL:AWS access key"
        ["ya29\\.[0-9A-Za-z\\-_]+"]="CRITICAL:Google OAuth token"
        
        # Database connection strings
        ["mysql://[^\\s\"']*:[^\\s\"']*@[^\\s\"']*"]="HIGH:MySQL connection string with credentials"
        ["postgres://[^\\s\"']*:[^\\s\"']*@[^\\s\"']*"]="HIGH:PostgreSQL connection string with credentials"
        ["mongodb://[^\\s\"']*:[^\\s\"']*@[^\\s\"']*"]="HIGH:MongoDB connection string with credentials"
        ["redis://[^\\s\"']*:[^\\s\"']*@[^\\s\"']*"]="HIGH:Redis connection string with credentials"
        
        # JWT tokens
        ["eyJ[A-Za-z0-9-_=]+\\.[A-Za-z0-9-_=]+\\.?[A-Za-z0-9-_.+/=]*"]="HIGH:JWT token"
        
        # Medium-severity patterns (potentially credentials)
        ["token[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9\\-_]{16,}"]="MEDIUM:Potential token"
        ["auth[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9\\-_]{16,}"]="MEDIUM:Potential auth credential"
        ["key[\"'\\s]*[:=][\"'\\s]*[a-zA-Z0-9\\-_]{16,}"]="MEDIUM:Potential key"
        
        # Low-severity patterns (suspicious but may be false positives)
        ["(password|secret|key|token).*=.*['\"][a-zA-Z0-9]{6,}['\"]"]="LOW:Potential credential assignment"
    )
    
    local find_cmd=$(build_find_command)
    
    # Execute find command and scan each file
    eval "$find_cmd" | while read -r file; do
        ((FILES_SCANNED++))
        
        # Skip binary files
        if file "$file" | grep -q "binary\|executable\|archive"; then
            continue
        fi
        
        # Scan file with each pattern
        for pattern in "${!credential_patterns[@]}"; do
            local severity_info="${credential_patterns[$pattern]}"
            local severity="${severity_info%%:*}"
            local description="${severity_info#*:}"
            
            # Search for pattern in file
            if grep -n -i -E "$pattern" "$file" 2>/dev/null; then
                while IFS= read -r match; do
                    local line_num=$(echo "$match" | cut -d: -f1)
                    local content=$(echo "$match" | cut -d: -f2-)
                    
                    # Skip lines that look like comments or examples
                    if echo "$content" | grep -E "^\s*(//|#|\*|/\*)" >/dev/null; then
                        continue
                    fi
                    
                    # Skip lines with obvious placeholder values
                    if echo "$content" | grep -iE "(example|placeholder|dummy|test|sample|your.*here|replace.*with|todo|fixme)" >/dev/null; then
                        continue
                    fi
                    
                    add_finding "$severity" "$file" "$line_num" "$description" "$content"
                done < <(grep -n -i -E "$pattern" "$file" 2>/dev/null || true)
            fi
        done
    done
}

# Function to scan for environment variable references
scan_environment_variables() {
    log "Scanning environment variable usage..."
    
    local find_cmd=$(build_find_command)
    local env_var_patterns=(
        "JWT_SECRET"
        "HMAC_KEY" 
        "API_KEY"
        "SECRET_KEY"
        "PASSWORD"
        "TOKEN"
        "ACCESS_KEY"
        "PRIVATE_KEY"
    )
    
    eval "$find_cmd" | while read -r file; do
        # Skip binary files
        if file "$file" | grep -q "binary\|executable\|archive"; then
            continue
        fi
        
        for env_var in "${env_var_patterns[@]}"; do
            # Look for direct usage (bad) vs environment variable reference (good)
            while IFS= read -r match; do
                local line_num=$(echo "$match" | cut -d: -f1)
                local content=$(echo "$match" | cut -d: -f2-)
                
                # Check if it's properly using environment variables
                if echo "$content" | grep -E "os\\.Getenv|ENV\\[|process\\.env|\\$\\{?$env_var\\}?" >/dev/null; then
                    # Good: using environment variable
                    continue
                elif echo "$content" | grep -E "\"$env_var\"[\\s]*[:=]|'$env_var'[\\s]*[:=]" >/dev/null; then
                    # Check if it's in a configuration file pointing to env var
                    if echo "$content" | grep -E "_env|env_var|environment" >/dev/null; then
                        # Good: configuration pointing to environment variable
                        continue
                    else
                        # Bad: direct assignment
                        add_finding "HIGH" "$file" "$line_num" "Direct credential assignment" "$content"
                    fi
                fi
            done < <(grep -n -i "$env_var" "$file" 2>/dev/null || true)
        done
    done
}

# Function to scan for logging vulnerabilities
scan_logging_vulnerabilities() {
    log "Scanning for credential exposure in logging..."
    
    local find_cmd=$(build_find_command)
    local logging_patterns=(
        "log.*password"
        "log.*secret"
        "log.*token"
        "log.*key"
        "printf.*password"
        "printf.*secret"
        "print.*password"
        "print.*secret"
        "fmt\\.Print.*password"
        "fmt\\.Print.*secret"
        "console\\.log.*password"
        "console\\.log.*secret"
    )
    
    eval "$find_cmd" | while read -r file; do
        # Skip binary files
        if file "$file" | grep -q "binary\|executable\|archive"; then
            continue
        fi
        
        for pattern in "${logging_patterns[@]}"; do
            while IFS= read -r match; do
                local line_num=$(echo "$match" | cut -d: -f1)
                local content=$(echo "$match" | cut -d: -f2-)
                
                # Skip obvious safe cases
                if echo "$content" | grep -iE "(masked|hidden|redacted|\*\*\*|\\[REDACTED\\])" >/dev/null; then
                    continue
                fi
                
                add_finding "HIGH" "$file" "$line_num" "Potential credential logging" "$content"
            done < <(grep -n -i -E "$pattern" "$file" 2>/dev/null || true)
        done
    done
}

# Function to scan for weak cryptographic practices
scan_weak_cryptography() {
    log "Scanning for weak cryptographic practices..."
    
    local find_cmd=$(build_find_command)
    local weak_crypto_patterns=(
        "md5"
        "sha1(?!.*sha256)" # SHA1 but not SHA256
        "DES(?!_EDE)" # DES but not 3DES
        "RC4"
        "ECB"
        "crypto/rand.*Read.*\\(.*\\[.*\\].*\\)"
        "math/rand.*Intn"
        "rand\\(\\)"
        "srand\\("
    )
    
    eval "$find_cmd" | while read -r file; do
        # Skip binary files and focus on code files
        if file "$file" | grep -q "binary\|executable\|archive"; then
            continue
        fi
        
        # Only scan source code files for crypto issues
        if [[ ! "$file" =~ \.(go|py|js|ts|java|c|cpp|h|hpp)$ ]]; then
            continue
        fi
        
        for pattern in "${weak_crypto_patterns[@]}"; do
            while IFS= read -r match; do
                local line_num=$(echo "$match" | cut -d: -f1)
                local content=$(echo "$match" | cut -d: -f2-)
                
                # Skip comments
                if echo "$content" | grep -E "^\s*(//|#|\*)" >/dev/null; then
                    continue
                fi
                
                case $pattern in
                    "md5")
                        add_finding "HIGH" "$file" "$line_num" "Weak hash algorithm: MD5" "$content"
                        ;;
                    "sha1"*)
                        add_finding "MEDIUM" "$file" "$line_num" "Weak hash algorithm: SHA1" "$content"
                        ;;
                    "DES"*)
                        add_finding "HIGH" "$file" "$line_num" "Weak encryption: DES" "$content"
                        ;;
                    "RC4")
                        add_finding "HIGH" "$file" "$line_num" "Weak encryption: RC4" "$content"
                        ;;
                    "ECB")
                        add_finding "MEDIUM" "$file" "$line_num" "Weak cipher mode: ECB" "$content"
                        ;;
                    *"math/rand"*|*"rand("*|*"srand("*)
                        add_finding "MEDIUM" "$file" "$line_num" "Weak random number generation" "$content"
                        ;;
                esac
            done < <(grep -n -i -E "$pattern" "$file" 2>/dev/null || true)
        done
    done
}

# Function to scan configuration files
scan_configuration_files() {
    log "Scanning configuration files for security issues..."
    
    local config_patterns=(
        "debug.*=.*true"
        "debug.*:.*true"
        "development.*=.*true"
        "test.*=.*true"
        "ssl.*=.*false"
        "tls.*=.*false"
        "verify.*=.*false"
        "insecure.*=.*true"
        "allow.*=.*\\*"
        "cors.*=.*\\*"
        "origin.*=.*\\*"
    )
    
    find "$SCAN_ROOT" -type f \( -name "*.yaml" -o -name "*.yml" -o -name "*.json" -o -name "*.toml" -o -name "*.ini" -o -name "*.conf" -o -name "*.config" -o -name ".env*" \) | while read -r file; do
        # Skip excluded directories
        local skip=false
        for excluded in $EXCLUDE_DIRS; do
            if [[ "$file" == *"/$excluded/"* ]]; then
                skip=true
                break
            fi
        done
        
        if $skip; then
            continue
        fi
        
        for pattern in "${config_patterns[@]}"; do
            while IFS= read -r match; do
                local line_num=$(echo "$match" | cut -d: -f1)
                local content=$(echo "$match" | cut -d: -f2-)
                
                case $pattern in
                    *"debug"*|*"development"*|*"test"*)
                        add_finding "MEDIUM" "$file" "$line_num" "Development/debug mode enabled" "$content"
                        ;;
                    *"ssl"*|*"tls"*|*"verify"*)
                        add_finding "HIGH" "$file" "$line_num" "Security verification disabled" "$content"
                        ;;
                    *"insecure"*)
                        add_finding "HIGH" "$file" "$line_num" "Insecure mode enabled" "$content"
                        ;;
                    *"*"*)
                        add_finding "MEDIUM" "$file" "$line_num" "Wildcard configuration (potential security risk)" "$content"
                        ;;
                esac
            done < <(grep -n -i -E "$pattern" "$file" 2>/dev/null || true)
        done
    done
}

# Function to scan for TODO/FIXME security comments
scan_security_todos() {
    log "Scanning for security-related TODO/FIXME comments..."
    
    local find_cmd=$(build_find_command)
    local security_todo_patterns=(
        "TODO.*security"
        "FIXME.*security"
        "HACK.*security"
        "XXX.*security"
        "TODO.*auth"
        "FIXME.*auth"
        "TODO.*password"
        "FIXME.*password"
        "TODO.*encrypt"
        "FIXME.*encrypt"
        "TODO.*ssl"
        "TODO.*tls"
        "FIXME.*ssl"
        "FIXME.*tls"
    )
    
    eval "$find_cmd" | while read -r file; do
        # Skip binary files
        if file "$file" | grep -q "binary\|executable\|archive"; then
            continue
        fi
        
        for pattern in "${security_todo_patterns[@]}"; do
            while IFS= read -r match; do
                local line_num=$(echo "$match" | cut -d: -f1)
                local content=$(echo "$match" | cut -d: -f2-)
                
                add_finding "LOW" "$file" "$line_num" "Security-related TODO/FIXME" "$content"
            done < <(grep -n -i -E "$pattern" "$file" 2>/dev/null || true)
        done
    done
}

# Function to generate whitelist of known false positives
generate_whitelist() {
    local whitelist_file="$SCAN_ROOT/.credential-scan-whitelist"
    
    if [[ -f "$whitelist_file" ]]; then
        log "Using existing whitelist: $whitelist_file"
        return 0
    fi
    
    log "Creating whitelist template at $whitelist_file"
    
    cat > "$whitelist_file" << 'EOF'
# Credential Scan Whitelist
# Add patterns of known false positives here
# Format: file_path:line_number:pattern_description
# Example: config/example.yaml:15:Example configuration
# Use wildcards: */test/*:*:Test files
# Use regex: .*_test\.go:*:Test files

# Common false positives
*/test/*:*:Test files
*_test.go:*:Go test files
*/examples/*:*:Example code
*/docs/*:*:Documentation
*example*:*:Example files
*sample*:*:Sample files
*template*:*:Template files
*/migrations/*:*:Database migrations (review manually)

# Specific patterns
*.md:*:Documentation files
README*:*:README files
CHANGELOG*:*:Changelog files
EOF
    
    warn "Created whitelist template. Review and customize as needed."
}

# Function to check whitelist
is_whitelisted() {
    local file="$1"
    local line="$2"
    local pattern="$3"
    local whitelist_file="$SCAN_ROOT/.credential-scan-whitelist"
    
    if [[ ! -f "$whitelist_file" ]]; then
        return 1
    fi
    
    # Check whitelist entries
    while IFS= read -r whitelist_entry; do
        # Skip comments and empty lines
        if [[ "$whitelist_entry" =~ ^[[:space:]]*# ]] || [[ -z "$whitelist_entry" ]]; then
            continue
        fi
        
        IFS=':' read -r file_pattern line_pattern desc_pattern <<< "$whitelist_entry"
        
        # Check file pattern
        if [[ "$file" == $file_pattern ]] || [[ "$file_pattern" == "*" ]]; then
            # Check line pattern
            if [[ "$line_pattern" == "*" ]] || [[ "$line" == "$line_pattern" ]]; then
                return 0
            fi
        fi
    done < "$whitelist_file"
    
    return 1
}

# Function to output detailed scan results
output_detailed_results() {
    log "Writing detailed results to $OUTPUT_FILE"
    
    {
        echo "AG-UI Go SDK Credential Scan Report"
        echo "Generated: $(date)"
        echo "Scan Root: $SCAN_ROOT"
        echo "Files Scanned: $FILES_SCANNED"
        echo "Critical Issues: $(echo "${FINDINGS[@]}" | grep -c "^CRITICAL:" || echo 0)"
        echo "High Issues: $(echo "${FINDINGS[@]}" | grep -c "^HIGH:" || echo 0)"
        echo "Medium Issues: $(echo "${FINDINGS[@]}" | grep -c "^MEDIUM:" || echo 0)"
        echo "Low Issues: $(echo "${FINDINGS[@]}" | grep -c "^LOW:" || echo 0)"
        echo
        echo "Detailed Findings:"
        echo "=================="
        
        for finding in "${FINDINGS[@]}"; do
            IFS=':' read -r severity file line pattern content <<< "$finding"
            
            # Check whitelist
            if is_whitelisted "$file" "$line" "$pattern"; then
                echo "[$severity] $file:$line - $pattern (WHITELISTED)"
                echo "  Content: $content"
            else
                echo "[$severity] $file:$line - $pattern"
                echo "  Content: $content"
            fi
            echo
        done
        
        echo "Recommendations:"
        echo "==============="
        echo "1. Review all CRITICAL and HIGH severity findings immediately"
        echo "2. Ensure all credentials use environment variable injection"
        echo "3. Remove any hardcoded secrets or API keys"
        echo "4. Fix weak cryptographic practices"
        echo "5. Address insecure configuration settings"
        echo "6. Update whitelist for confirmed false positives"
        
    } > "$OUTPUT_FILE"
    
    success "Detailed results written to $OUTPUT_FILE"
}

# Function to generate scan summary
generate_scan_summary() {
    local critical_count=$(printf '%s\n' "${FINDINGS[@]}" | grep -c "^CRITICAL:" || echo 0)
    local high_count=$(printf '%s\n' "${FINDINGS[@]}" | grep -c "^HIGH:" || echo 0)
    local medium_count=$(printf '%s\n' "${FINDINGS[@]}" | grep -c "^MEDIUM:" || echo 0)
    local low_count=$(printf '%s\n' "${FINDINGS[@]}" | grep -c "^LOW:" || echo 0)
    
    echo
    echo "========================================="
    echo "Credential Scan Summary"
    echo "========================================="
    echo "Scan Root: $SCAN_ROOT"
    echo "Files Scanned: $FILES_SCANNED"
    echo "Output File: $OUTPUT_FILE"
    echo
    echo "Findings by Severity:"
    echo "🔴 Critical: $critical_count"
    echo "🟠 High: $high_count"  
    echo "🟡 Medium: $medium_count"
    echo "🔵 Low: $low_count"
    echo "📊 Total: $((critical_count + high_count + medium_count + low_count))"
    echo
    
    # Risk assessment
    if [[ $critical_count -gt 0 ]]; then
        error "❌ CRITICAL security vulnerabilities found"
        error "🚫 DO NOT DEPLOY until critical issues are resolved"
        echo
        error "Critical issues must be fixed before deployment!"
        return 2
    elif [[ $high_count -gt 0 ]]; then
        error "⚠️  HIGH severity security issues found"
        warn "🟡 Deployment not recommended until high issues are addressed"
        echo
        warn "Review and fix high-severity issues before deployment"
        return 1
    elif [[ $medium_count -gt 5 ]]; then
        warn "⚠️  Multiple MEDIUM severity issues found"
        warn "🟡 Review recommended before deployment"
        echo
        warn "Consider addressing medium-severity issues"
        return 0
    else
        success "✅ No critical security issues found"
        if [[ $medium_count -gt 0 || $low_count -gt 0 ]]; then
            success "🟢 Deployment approved with minor recommendations"
        else
            success "🟢 Deployment approved - clean scan!"
        fi
        echo
        success "Security scan passed! Safe to deploy."
        return 0
    fi
}

# Main execution
main() {
    echo "========================================="
    echo "AG-UI Go SDK Credential Scanner"
    echo "========================================="
    echo
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --scan-root=*)
                SCAN_ROOT="${1#*=}"
                shift
                ;;
            --output=*)
                OUTPUT_FILE="${1#*=}"
                shift
                ;;
            --generate-whitelist)
                generate_whitelist
                exit 0
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Credential scanning for AG-UI Go SDK"
                echo
                echo "Options:"
                echo "  --scan-root=PATH      Root directory to scan (default: current directory)"
                echo "  --output=FILE         Output file for detailed results"
                echo "  --generate-whitelist  Generate whitelist template and exit"
                echo "  --help                Show this help message"
                echo
                echo "Exit codes:"
                echo "  0 - No critical issues found"
                echo "  1 - High severity issues found"
                echo "  2 - Critical issues found"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Validate scan root
    if [[ ! -d "$SCAN_ROOT" ]]; then
        error "Scan root directory does not exist: $SCAN_ROOT"
        exit 1
    fi
    
    log "Starting credential scan of: $SCAN_ROOT"
    
    # Generate whitelist if it doesn't exist
    if [[ ! -f "$SCAN_ROOT/.credential-scan-whitelist" ]]; then
        generate_whitelist
    fi
    
    # Run all scans
    FILES_SCANNED=0
    scan_hardcoded_credentials
    scan_environment_variables
    scan_logging_vulnerabilities
    scan_weak_cryptography
    scan_configuration_files
    scan_security_todos
    
    # Output results
    output_detailed_results
    generate_scan_summary
}

# Execute main function
main "$@"