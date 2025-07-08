#!/bin/bash

# Pre-commit setup script for ag-ui Go SDK
# This script installs and configures pre-commit hooks for the project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install pre-commit
install_pre_commit() {
    print_status "Installing pre-commit..."
    
    if command_exists brew; then
        print_status "Using Homebrew to install pre-commit..."
        brew install pre-commit
    elif command_exists pip3; then
        print_status "Using pip3 to install pre-commit..."
        pip3 install pre-commit
    elif command_exists pip; then
        print_status "Using pip to install pre-commit..."
        pip install pre-commit
    else
        print_error "No suitable package manager found. Please install pre-commit manually:"
        print_error "  - On macOS: brew install pre-commit"
        print_error "  - On Linux: pip install pre-commit"
        print_error "  - Or visit: https://pre-commit.com/#installation"
        exit 1
    fi
}

# Function to install Go tools
install_go_tools() {
    print_status "Installing required Go tools..."
    
    # Check if Go is installed
    if ! command_exists go; then
        print_error "Go is not installed. Please install Go first."
        exit 1
    fi
    
    # Install Go tools
    local tools=(
        "golang.org/x/tools/cmd/goimports@latest"
        "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        "github.com/securego/gosec/v2/cmd/gosec@latest"
        "golang.org/x/vuln/cmd/govulncheck@latest"
        "google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    )
    
    for tool in "${tools[@]}"; do
        print_status "Installing $tool..."
        go install "$tool"
    done
}

# Function to install additional tools
install_additional_tools() {
    print_status "Installing additional tools..."
    
    # Install buf for Protocol Buffers (if not already installed)
    if ! command_exists buf; then
        print_status "Installing buf for Protocol Buffer linting..."
        if command_exists brew; then
            brew install bufbuild/buf/buf
        else
            print_warning "buf not installed via Homebrew. Please install manually:"
            print_warning "  https://docs.buf.build/installation"
        fi
    fi
    
    # Install markdownlint-cli (if not already installed)
    if ! command_exists markdownlint; then
        print_status "Installing markdownlint-cli..."
        if command_exists npm; then
            npm install -g markdownlint-cli
        else
            print_warning "markdownlint-cli not installed. Please install Node.js and npm first."
        fi
    fi
    
    # Install hadolint for Dockerfile linting (if not already installed)
    if ! command_exists hadolint; then
        print_status "Installing hadolint..."
        if command_exists brew; then
            brew install hadolint
        else
            print_warning "hadolint not installed via Homebrew. Please install manually:"
            print_warning "  https://github.com/hadolint/hadolint#install"
        fi
    fi
    
    # Install shellcheck (if not already installed)
    if ! command_exists shellcheck; then
        print_status "Installing shellcheck..."
        if command_exists brew; then
            brew install shellcheck
        else
            print_warning "shellcheck not installed via Homebrew. Please install manually:"
            print_warning "  https://github.com/koalaman/shellcheck#installing"
        fi
    fi
}

# Function to setup pre-commit hooks
setup_pre_commit_hooks() {
    print_status "Setting up pre-commit hooks..."
    
    # Change to the repository root
    cd "$(dirname "$0")/.." || exit 1
    
    # Install pre-commit hooks
    pre-commit install
    
    # Install commit-msg hook
    pre-commit install --hook-type commit-msg
    
    # Install pre-push hook
    pre-commit install --hook-type pre-push
    
    print_success "Pre-commit hooks installed successfully!"
}

# Function to run initial checks
run_initial_checks() {
    print_status "Running initial pre-commit checks..."
    
    # Run pre-commit on all files
    if pre-commit run --all-files; then
        print_success "All pre-commit checks passed!"
    else
        print_warning "Some pre-commit checks failed. This is normal for the first run."
        print_warning "The hooks will fix many issues automatically."
        print_warning "Please review the changes and commit them if they look correct."
    fi
}

# Function to create secrets baseline
create_secrets_baseline() {
    print_status "Creating secrets baseline..."
    
    # Change to the repository root
    cd "$(dirname "$0")/.." || exit 1
    
    # Create secrets baseline if it doesn't exist
    if [ ! -f ".secrets.baseline" ]; then
        if command_exists detect-secrets; then
            detect-secrets scan --baseline .secrets.baseline
            print_success "Secrets baseline created!"
        else
            print_warning "detect-secrets not found. Installing..."
            if command_exists pip3; then
                pip3 install detect-secrets
                detect-secrets scan --baseline .secrets.baseline
                print_success "Secrets baseline created!"
            else
                print_warning "Could not install detect-secrets. Please install manually:"
                print_warning "  pip install detect-secrets"
            fi
        fi
    else
        print_success "Secrets baseline already exists!"
    fi
}

# Function to display usage information
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help        Show this help message"
    echo "  --skip-tools      Skip installation of additional tools"
    echo "  --skip-check      Skip running initial pre-commit checks"
    echo ""
    echo "This script will:"
    echo "  1. Install pre-commit (if not already installed)"
    echo "  2. Install required Go tools"
    echo "  3. Install additional linting tools"
    echo "  4. Setup pre-commit hooks"
    echo "  5. Create secrets baseline"
    echo "  6. Run initial pre-commit checks"
}

# Main execution
main() {
    local skip_tools=false
    local skip_check=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            --skip-tools)
                skip_tools=true
                shift
                ;;
            --skip-check)
                skip_check=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    print_status "Starting pre-commit setup for ag-ui Go SDK..."
    
    # Check if we're in a Git repository
    if [ ! -d ".git" ]; then
        print_error "This script must be run from the root of a Git repository."
        exit 1
    fi
    
    # Install pre-commit if not already installed
    if ! command_exists pre-commit; then
        install_pre_commit
    else
        print_success "pre-commit is already installed!"
    fi
    
    # Install Go tools
    install_go_tools
    
    # Install additional tools (unless skipped)
    if [ "$skip_tools" = false ]; then
        install_additional_tools
    fi
    
    # Setup pre-commit hooks
    setup_pre_commit_hooks
    
    # Create secrets baseline
    create_secrets_baseline
    
    # Run initial checks (unless skipped)
    if [ "$skip_check" = false ]; then
        run_initial_checks
    fi
    
    print_success "Pre-commit setup completed successfully!"
    print_status "You can now run 'pre-commit run --all-files' to check all files"
    print_status "or 'pre-commit run' to check only staged files."
    print_status ""
    print_status "The hooks will run automatically on each commit."
    print_status "To bypass hooks temporarily, use 'git commit --no-verify'"
}

# Run main function
main "$@"