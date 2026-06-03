#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ONLAVA_BIN="${ONLAVA_BIN:-onlava}"
ONLV_ROOT="${ONLAVA_ONLV_SMOKE_ROOT:-${ONLAVA_RELEASE_GATE_EXTERNAL_APP_ROOT:-/Users/petrbrazdil/Repos/onlv}}"
LOG_DIR="${ONLAVA_ONLV_SMOKE_LOG_DIR:-$ROOT/.onlava/release-gate/onlv-smoke}"
EDGE_PUBLIC_ADDR="127.0.0.1:443"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'onlv smoke: missing required command: %s\n' "$1" >&2
    exit 1
  }
}

json_get() {
  python3 - "$1" "$2" <<'PY'
import json
import sys
path, expr = sys.argv[1:]
value = json.loads(open(path).read())
for part in expr.split("."):
    if part:
        value = value[part]
print(value)
PY
}

cleanup_items=()
session_ids=()
AGENT_HOME=""
AGENT_PID=""
EDGE_STARTED=0

cleanup() {
  local id item
  for id in "${session_ids[@]:-}"; do
    if [[ -n "$AGENT_HOME" ]]; then
      ONLAVA_AGENT_HOME="$AGENT_HOME" "$ONLAVA_BIN" down --session "$id" --all >/dev/null 2>&1 || true
    fi
  done
  if [[ -n "$AGENT_PID" ]]; then
    kill -INT "$AGENT_PID" >/dev/null 2>&1 || true
    wait "$AGENT_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$EDGE_STARTED" == "1" && -n "$AGENT_HOME" ]]; then
    ONLAVA_AGENT_HOME="$AGENT_HOME" "$ONLAVA_BIN" edge uninstall --json >/dev/null 2>&1 || true
  fi
  if [[ -n "$AGENT_HOME" ]]; then
    pkill -f "$AGENT_HOME" >/dev/null 2>&1 || true
  fi
  for item in "${cleanup_items[@]:-}"; do
    eval "$item" || true
  done
}
trap cleanup EXIT

need git
need python3

[[ -d "$ONLV_ROOT/.git" ]] || { printf 'onlv smoke: ONLV root not found: %s\n' "$ONLV_ROOT" >&2; exit 1; }
[[ -f "$ONLV_ROOT/.onlava.json" ]] || { printf 'onlv smoke: missing .onlava.json in %s\n' "$ONLV_ROOT" >&2; exit 1; }

mkdir -p "$LOG_DIR"
TMP="$(mktemp -d)"
cleanup_items+=("rm -rf '$TMP'")
AGENT_HOME="$TMP/agent"
export ONLAVA_AGENT_HOME="$AGENT_HOME"
export ONLAVA_LOCAL_PROXY=0

WT_A="$TMP/onlv-a"
WT_B="$TMP/onlv-b"
git -C "$ONLV_ROOT" worktree add --detach "$WT_A" HEAD >/dev/null
git -C "$ONLV_ROOT" worktree add --detach "$WT_B" HEAD >/dev/null
cleanup_items+=("git -C '$ONLV_ROOT' worktree remove --force '$WT_B' >/dev/null 2>&1")
cleanup_items+=("git -C '$ONLV_ROOT' worktree remove --force '$WT_A' >/dev/null 2>&1")

prepare_worktree() {
  local wt="$1"
  python3 - "$wt/go.mod" "$ROOT" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
root = sys.argv[2]
text = path.read_text()
lines = []
replaced = False
for line in text.splitlines():
    if line.strip().startswith("replace github.com/pbrazdil/onlava =>"):
        lines.append(f"replace github.com/pbrazdil/onlava => {root}")
        replaced = True
    else:
        lines.append(line)
if not replaced:
    lines.append(f"replace github.com/pbrazdil/onlava => {root}")
path.write_text("\n".join(lines) + "\n")
PY
  for env_file in .env .env.local ".secrets.local.cue"; do
    if [[ -f "$ONLV_ROOT/$env_file" && ! -e "$wt/$env_file" ]]; then
      cp "$ONLV_ROOT/$env_file" "$wt/$env_file"
    fi
  done
  for dep in node_modules apps/pulse/node_modules apps/blog/node_modules apps/ui/node_modules apps/console/node_modules apps/viewer/node_modules; do
    if [[ -d "$ONLV_ROOT/$dep" && ! -e "$wt/$dep" ]]; then
      mkdir -p "$(dirname "$wt/$dep")"
      ln -s "$ONLV_ROOT/$dep" "$wt/$dep"
    fi
  done
}

prepare_worktree "$WT_A"
prepare_worktree "$WT_B"

start_edge() {
  local out="$LOG_DIR/edge-install.json"
  local err="$LOG_DIR/edge-install.stderr"
  if "$ONLAVA_BIN" edge install --json >"$out" 2>"$err"; then
    EDGE_STARTED=1
    return 0
  fi
  cat "$err" >&2 || true
  if [[ -f "$AGENT_HOME/agent/edge/caddy.log" ]]; then
    printf '\nedge log:\n' >&2
    tail -200 "$AGENT_HOME/agent/edge/caddy.log" >&2 || true
  fi
  if [[ -f "$AGENT_HOME/agent/agent.log" ]]; then
    printf '\nagent log:\n' >&2
    tail -200 "$AGENT_HOME/agent/agent.log" >&2 || true
  fi
  return 1
}

start_edge

start_session() {
  local wt="$1"
  local name="$2"
  local out="$LOG_DIR/$name-detach.json"
  ONLAVA_AGENT_HOME="$AGENT_HOME" "$ONLAVA_BIN" dev --app-root "$wt" --new-session --detach --json >"$out"
  json_get "$out" "session.session_id"
}

SESSION_A="$(start_session "$WT_A" a)"
session_ids+=("$SESSION_A")
SESSION_B="$(start_session "$WT_B" b)"
session_ids+=("$SESSION_B")

STATUS="$LOG_DIR/status.json"
wait_for_sessions_ready() {
  local deadline=$((SECONDS + 180))
  while (( SECONDS < deadline )); do
    ONLAVA_AGENT_HOME="$AGENT_HOME" "$ONLAVA_BIN" status --json >"$STATUS"
    if python3 - "$STATUS" "$SESSION_A" "$SESSION_B" <<'PY'
import json
import sys
payload = json.loads(open(sys.argv[1]).read())
sessions = {s.get("session_id"): s for s in payload.get("sessions", [])}
required = ["api", "dashboard", "electric", "grafana", "temporal", "pulse", "blog"]
for sid in sys.argv[2:]:
    session = sessions.get(sid) or {}
    routes = session.get("routes") or {}
    backends = session.get("backends") or {}
    if not all(routes.get(route) for route in required):
        raise SystemExit(1)
    if (backends.get("api") or {}).get("network") != "unix":
        raise SystemExit(1)
print("ready")
PY
    then
      return 0
    fi
    sleep 1
  done
  printf 'timed out waiting for ONLV sessions to register all routes\n' >&2
  ONLAVA_AGENT_HOME="$AGENT_HOME" "$ONLAVA_BIN" status --json >&2 || true
  return 1
}

wait_for_sessions_ready

python3 - "$STATUS" "$SESSION_A" "$SESSION_B" "$EDGE_PUBLIC_ADDR" <<'PY'
import json
import re
import sys

status_path, session_a, session_b, edge_public_addr = sys.argv[1:]
edge_on_443 = edge_public_addr.endswith(":443")
payload = json.loads(open(status_path).read())
sessions = {s["session_id"]: s for s in payload.get("sessions", [])}
missing = [sid for sid in (session_a, session_b) if sid not in sessions]
if missing:
    raise SystemExit(f"missing sessions in status: {missing}")
a = sessions[session_a]
b = sessions[session_b]

def fail(msg):
    raise SystemExit(msg)

if a["session_id"] == b["session_id"]:
    fail("session IDs must differ")

api_a = a.get("backends", {}).get("api", {})
api_b = b.get("backends", {}).get("api", {})
if api_a.get("network") != "unix" or api_b.get("network") != "unix" or api_a.get("addr") == api_b.get("addr"):
    fail(f"API Unix socket backends are not isolated: {api_a} {api_b}")

required = {
    "api": "api",
    "electric": "electric",
    "grafana": "grafana",
    "temporal": "temporal",
    "dashboard": "console",
    "pulse": "pulse",
    "blog": "blog",
}
for session in (a, b):
    sid = session["session_id"]
    routes = session.get("routes", {})
    for route, label in required.items():
        url = routes.get(route, "")
        if not url:
            fail(f"{sid} missing route {route}: {routes}")
        if ".onlava.localhost" in url:
            fail(f"{sid} route {route} uses onlava.localhost: {url}")
        if ":9440" in url:
            fail(f"{sid} route {route} kept fallback router port under HTTPS 443: {url}")
        if edge_on_443 and re.search(r":\d+/", url):
            fail(f"{sid} route {route} kept explicit port while edge is on HTTPS 443: {url}")
        if ".onlv.localhost" not in url:
            fail(f"{sid} route {route} is not under onlv.localhost: {url}")
        if f"{label}.{sid}.onlv.localhost" not in url:
            fail(f"{sid} route {route} is not session-scoped with label {label}: {url}")

alias_owner = {}
for session in (a, b):
    for route, url in (session.get("aliases") or {}).items():
        host = re.sub(r"^https?://", "", url).split("/", 1)[0].split(":", 1)[0]
        previous = alias_owner.get(host)
        if previous and previous != session["session_id"]:
            fail(f"alias {host} is owned by both {previous} and {session['session_id']}")
        alias_owner[host] = session["session_id"]

if not alias_owner:
    fail("expected one live session to own friendly aliases")

for session in (a, b):
    for route, conflict in (session.get("alias_conflicts") or {}).items():
        host = conflict.get("host", "")
        owner = conflict.get("session_id", "")
        if host in alias_owner and alias_owner[host] == session["session_id"]:
            fail(f"{session['session_id']} reports conflict for its own alias {host}")
        if owner == session["session_id"]:
            fail(f"{session['session_id']} reports self-owned alias conflict {route}: {conflict}")
PY

python3 - "$STATUS" "$SESSION_A" "$SESSION_B" <<'PY'
import json
import os
import re
import subprocess
import sys

status_path, session_a, session_b = sys.argv[1:]
payload = json.loads(open(status_path).read())
sessions = {s["session_id"]: s for s in payload.get("sessions", [])}

def env_for_pid(pid):
    if not pid:
        return ""
    try:
        return subprocess.check_output(["ps", "eww", "-p", str(pid)], text=True, stderr=subprocess.DEVNULL)
    except Exception:
        return ""

def value(text, key):
    match = re.search(rf"(?:^|\\s){re.escape(key)}=([^\\s]+)", text)
    return match.group(1) if match else ""

db_names = []
electric_streams = []
temporal_queues = []
for sid in (session_a, session_b):
    session = sessions[sid]
    processes = session.get("processes") or {}
    api_env = env_for_pid((processes.get("api") or {}).get("pid"))
    worker_env = env_for_pid((processes.get("worker-typescript") or {}).get("pid"))
    electric_env = env_for_pid((processes.get("electric") or {}).get("pid"))
    db_name = value(api_env, "ONLAVA_MANAGED_DATABASE_NAME")
    queue = value(worker_env, "ONLAVA_TEMPORAL_TASK_QUEUE_PREFIX") or value(api_env, "ONLAVA_TEMPORAL_TASK_QUEUE_PREFIX")
    stream = value(electric_env, "ELECTRIC_REPLICATION_STREAM_ID")
    if not db_name:
        raise SystemExit(f"{sid} missing ONLAVA_MANAGED_DATABASE_NAME in API process environment")
    if not queue:
        raise SystemExit(f"{sid} missing ONLAVA_TEMPORAL_TASK_QUEUE_PREFIX in worker/API process environment")
    if not stream:
        expected = "onlava_" + re.sub(r"[^A-Za-z0-9_]", "_", sid)
        if expected not in electric_env:
            raise SystemExit(f"{sid} missing ELECTRIC_REPLICATION_STREAM_ID in Electric process command/environment")
        stream = expected
    db_names.append(db_name)
    temporal_queues.append(queue)
    electric_streams.append(stream)

if len(set(db_names)) != 2:
    raise SystemExit(f"managed DB names are not distinct: {db_names}")
if len(set(electric_streams)) != 2:
    raise SystemExit(f"Electric stream IDs are not distinct: {electric_streams}")
if len(set(temporal_queues)) != 2:
    raise SystemExit(f"Temporal task queue prefixes are not distinct: {temporal_queues}")
PY

printf 'onlv two-worktree smoke passed: %s %s\n' "$SESSION_A" "$SESSION_B"
