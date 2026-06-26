#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../../.." && pwd)"
flutter_dir="$repo_root/sdks/community/dart/example"
go_server_dir="$repo_root/sdks/community/go/example/server"

host="${HOST:-127.0.0.1}"
port="${PORT:-8080}"
server_url="http://$host:$port"
base_url="${AG_UI_BASE_URL:-$server_url}"
server_log="${AG_UI_GO_SERVER_LOG:-$(mktemp -t ag-ui-go-server.XXXXXX.log)}"

native_device_for_host() {
  case "$(uname -s)" in
    Darwin) printf '%s\n' "macos" ;;
    Linux) printf '%s\n' "linux" ;;
    MINGW*|MSYS*|CYGWIN*) printf '%s\n' "windows" ;;
    *) printf '%s\n' "" ;;
  esac
}

flutter_device() {
  if [[ -n "${FLUTTER_DEVICE:-}" ]]; then
    printf '%s\n' "$FLUTTER_DEVICE"
    return
  fi

  local native_device
  native_device="$(native_device_for_host)"
  if [[ -n "$native_device" && -d "$flutter_dir/$native_device" ]]; then
    printf '%s\n' "$native_device"
    return
  fi

  printf '%s\n' "chrome"
}

wait_for_server() {
  local tries="${AG_UI_SERVER_WAIT_TRIES:-60}"
  local delay="${AG_UI_SERVER_WAIT_DELAY:-0.5}"

  for ((i = 1; i <= tries; i++)); do
    if ! kill -0 "$server_pid" 2>/dev/null; then
      printf 'Go example server exited before becoming ready. Log:\n' >&2
      cat "$server_log" >&2 || true
      return 1
    fi

    if curl -fsS "$server_url/" >/dev/null 2>&1; then
      return 0
    fi

    sleep "$delay"
  done

  printf 'Timed out waiting for Go example server at %s. Log:\n' "$server_url" >&2
  cat "$server_log" >&2 || true
  return 1
}

cleanup() {
  if [[ -n "${server_pid:-}" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

if [[ ! -d "$go_server_dir" ]]; then
  printf 'Missing Go example server directory: %s\n' "$go_server_dir" >&2
  exit 1
fi

if [[ ! -d "$flutter_dir" ]]; then
  printf 'Missing Flutter example directory: %s\n' "$flutter_dir" >&2
  exit 1
fi

device="$(flutter_device)"

printf 'Starting Go example server at %s (log: %s)\n' "$server_url" "$server_log"
(
  cd "$go_server_dir"
  HOST="$host" PORT="$port" CORS_ENABLED="${CORS_ENABLED:-true}" go run ./cmd
) >"$server_log" 2>&1 &
server_pid="$!"

wait_for_server

printf 'Starting Flutter example on device "%s" with AG_UI_BASE_URL=%s\n' "$device" "$base_url"
(
  cd "$flutter_dir"
  flutter pub get
  flutter run -d "$device" --dart-define=AG_UI_BASE_URL="$base_url" "$@"
)
