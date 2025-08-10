#!/bin/bash

# Development server script with live reload support
# Supports reflex, air, or falls back to basic go run

set -e

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "Error: go.mod not found. Run this script from the server directory."
    exit 1
fi

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to create minimal air config if it doesn't exist
create_air_config() {
    if [ ! -f ".air.toml" ]; then
        cat > .air.toml << 'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
EOF
        echo "Created .air.toml configuration file"
    fi
}

echo "Starting development server..."

if command_exists reflex; then
    echo "Using reflex for live reload"
    exec reflex -r '\.(go|mod|sum)$' -- sh -c 'go run ./cmd/server'
elif command_exists air; then
    echo "Using air for live reload"
    create_air_config
    exec air
else
    echo "Live reload tools not found. Running with basic go run."
    echo "For live reload, install one of:"
    echo "  go install github.com/cespare/reflex@latest"
    echo "  go install github.com/cosmtrek/air@latest"
    echo ""
    exec go run ./cmd/server
fi