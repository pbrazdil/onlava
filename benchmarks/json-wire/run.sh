#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

SCENERY_BIN="${SCENERY_BIN:-scenery}"
PORT="${PORT:-48191}"
LISTEN="${LISTEN:-127.0.0.1}"
BASE_URL="http://${LISTEN}:${PORT}"
LOG_FILE=".generated/scenery-run.log"

mkdir -p .generated

"${SCENERY_BIN}" check --app-root . --json >/dev/null
"${SCENERY_BIN}" gen client --app-root . --lang typescript --output .generated/client.ts >/dev/null

"${SCENERY_BIN}" run --app-root . --listen "${LISTEN}" --port "${PORT}" --log-format json >"${LOG_FILE}" 2>&1 &
server_pid=$!

cleanup() {
    kill "${server_pid}" >/dev/null 2>&1 || true
    wait "${server_pid}" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

for _ in $(seq 1 100); do
    if curl -fsS "${BASE_URL}/_wire/capabilities" >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

if ! curl -fsS "${BASE_URL}/_wire/capabilities" >/dev/null 2>&1; then
    echo "scenery benchmark server did not start; log follows:" >&2
    cat "${LOG_FILE}" >&2
    exit 1
fi

BASE_URL="${BASE_URL}" bun run bench.ts
