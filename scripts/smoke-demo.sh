#!/usr/bin/env bash
# Requires a free local port 8080; no provider credentials or network calls.
set -euo pipefail
cd "$(dirname "$0")/.."
smoke_dir=$(mktemp -d "${TMPDIR:-/tmp}/wolfy-smoke.XXXXXX")
smoke_pid=
cleanup() {
    if [[ -n "$smoke_pid" ]]; then
        kill "$smoke_pid" 2>/dev/null || true
        wait "$smoke_pid" 2>/dev/null || true
    fi
    rm -rf "$smoke_dir"
}
trap cleanup EXIT
export WOLFY_MODE=demo WOLFY_LISTEN=127.0.0.1:8080 WOLFY_DB="$smoke_dir/demo.db"
export WOLFY_BUDGET_NANO_USD=2000000000 WOLFY_DAILY_LIMIT=20
./bin/wolfy serve > "$smoke_dir/demo.log" 2>&1 &
smoke_pid=$!
ready=false
for ((attempt = 0; attempt < 50; attempt++)); do
    if ! kill -0 "$smoke_pid" 2>/dev/null; then
        cat "$smoke_dir/demo.log"
        exit 1
    fi
    if curl --noproxy '*' -fsS http://127.0.0.1:8080/healthz > /dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 0.2
done
[[ "$ready" == true ]]
python3 scripts/demo-webhook.py
accepted=false
for ((attempt = 0; attempt < 50; attempt++)); do
    if grep -q '"msg":"reply_accepted"' "$smoke_dir/demo.log"; then
        accepted=true
        break
    fi
    sleep 0.2
done
[[ "$accepted" == true ]]
kill "$smoke_pid"
wait "$smoke_pid"
smoke_pid=
./bin/wolfy check
./bin/wolfy backup "$smoke_dir/backup.db"
WOLFY_DB="$smoke_dir/backup.db" ./bin/wolfy check
./bin/wolfy restore-safe
./bin/wolfy resume-ai 1000000
./bin/wolfy status
echo 'Demo and maintenance checks passed.'
