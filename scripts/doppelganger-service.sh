#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
RUNTIME_DIR="$ROOT_DIR/.runtime"
ENV_FILE="$RUNTIME_DIR/.env.lite"
BIN="$ROOT_DIR/build/weknora-doppelganger"
PID_FILE="$RUNTIME_DIR/weknora.pid"
LOG_FILE="$ROOT_DIR/logs/service.log"
BASE_URL="http://127.0.0.1:18081"

fail() { echo "error: $*" >&2; exit 1; }
require_runtime() {
  [[ -x "$BIN" ]] || fail "missing binary: $BIN (run: go build -tags sqlite_fts5 -ldflags '-X github.com/Tencent/WeKnora/internal/handler.Edition=lite' -o build/weknora-doppelganger ./cmd/server)"
  [[ -r "$ENV_FILE" ]] || fail "missing private env: $ENV_FILE"
  [[ "$(stat -f '%Lp' "$ENV_FILE")" == "600" ]] || fail "private env must have mode 0600"
}
read_pid() { [[ -s "$PID_FILE" ]] && cat "$PID_FILE" || true; }
owned_pid() {
  local pid
  pid="$(read_pid)"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  local command
  command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  [[ "$command" == "$BIN" ]] || { echo "error: refusing to operate on non-WeKnora PID $pid" >&2; return 2; }
  echo "$pid"
}
start() {
  require_runtime
  local pid ownership=0
  pid="$(owned_pid)" || ownership=$?
  [[ "$ownership" != 2 ]] || return 1
  if [[ "$ownership" == 0 ]]; then echo "already running pid=$pid url=$BASE_URL"; return; fi
  rm -f "$PID_FILE"
  mkdir -p "$ROOT_DIR/logs" "$ROOT_DIR/data/files"
  chmod 700 "$ROOT_DIR/logs" "$ROOT_DIR/data" "$ROOT_DIR/data/files"
  # env -i prevents old app credentials, DB, Redis or storage settings from leaking in.
  python3 - "$BIN" "$ENV_FILE" "$LOG_FILE" "$ROOT_DIR/web" <<'PYLAUNCH' >"$RUNTIME_DIR/launch.log" 2>&1 &
import os, sys, subprocess
binary, env_file, log_file, web_dir = sys.argv[1:]
env = {"PATH": os.environ.get("PATH", ""), "HOME": os.environ.get("HOME", ""),
       "ENV_FILE": env_file, "WEKNORA_WEB_DIR": web_dir}
with open(log_file, "ab", buffering=0) as out:
    child = subprocess.Popen([binary], env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=subprocess.STDOUT, start_new_session=True)
print(child.pid, flush=True)
PYLAUNCH
  local launcher_pid=$!
  wait "$launcher_pid"
  local child_pid
  child_pid="$(tail -1 "$RUNTIME_DIR/launch.log")"
  [[ "$child_pid" =~ ^[0-9]+$ ]] || fail "failed to launch service; see $RUNTIME_DIR/launch.log"
  echo "$child_pid" > "$PID_FILE"
  chmod 600 "$PID_FILE"
  echo "started pid=$(cat "$PID_FILE") url=$BASE_URL"
}
stop() {
  local pid ownership=0
  pid="$(owned_pid)" || ownership=$?
  [[ "$ownership" != 2 ]] || return 1
  if [[ "$ownership" != 0 ]]; then rm -f "$PID_FILE"; echo "not running"; return; fi
  kill -TERM "$pid"
  for _ in {1..30}; do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
  if kill -0 "$pid" 2>/dev/null; then kill -KILL "$pid"; fi
  rm -f "$PID_FILE"
  echo "stopped pid=$pid"
}
status() {
  local pid ownership=0
  pid="$(owned_pid)" || ownership=$?
  [[ "$ownership" != 2 ]] || return 1
  if [[ "$ownership" == 0 ]]; then
    echo "running pid=$pid url=$BASE_URL"
    curl --max-time 5 -fsS "$BASE_URL/health" || return 1
    echo
  else
    echo "stopped"
    return 1
  fi
}
case "${1:-}" in
  start) start ;;
  stop) stop ;;
  restart) stop; start ;;
  status) status ;;
  *) echo "usage: $0 {start|stop|restart|status}" >&2; exit 2 ;;
esac
