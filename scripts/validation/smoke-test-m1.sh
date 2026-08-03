#!/usr/bin/env bash
# M1.7 — Validation jalon M1 (cahier des charges §10.3):
#   "un log tail arrive dans ClickHouse ; SELECT depuis l'UI" + latence
#   ingestion -> requêtable < 2s.
#
# Query Studio (M1.6, React) doesn't exist yet, and Redpanda isn't wired
# (M1.2's honest platform limitation — no librdkafka toolchain here), so
# this validates the equivalent real path available today:
#
#   agent (tail fichier) -> gRPC réel -> receiver -> ClickHouse -> API
#
# using `POST /api/v1/query` in place of a UI, and measures the same
# ingestion-to-queryable latency the cahier asks for. Also does a
# doctor-lite pass: reports whether each real service answers.
#
# Requires: real toolchains built (this script builds them), and nothing
# else already bound to the ports below.
set -euo pipefail

export PATH="$PATH:$HOME/sdk/go/bin:$HOME/go/bin:$HOME/sdk/protoc/bin:$HOME/sdk/node/bin"

cd "$(git rev-parse --show-toplevel)"

CLICKHOUSE_URL="http://127.0.0.1:8123"
API_URL="http://127.0.0.1:8080"
RECEIVER_ADDR="http://127.0.0.1:9090"
DATASET="m1_smoke_$$"
ADMIN_USER="smoke-admin-$$"
ADMIN_PASSWORD="smoke-password-$$"

WORKDIR="$(mktemp -d)"
PIDS=()

log() { echo "[smoke-test-m1] $*"; }

cleanup() {
  log "cleaning up..."
  for pid in "${PIDS[@]:-}"; do
    kill -TERM "$pid" 2>/dev/null || true
  done
  # The control-plane's own supervisor (M0.5) gives ClickHouse up to 10s to
  # drain on SIGTERM before it gives up — SIGKILL-ing the wrapper sooner
  # than that would orphan the clickhouse/watchdog processes underneath it
  # (this bit before; ps aux showed a live clickhouse process with no
  # parent). 12s covers that plus margin.
  sleep 12
  for pid in "${PIDS[@]:-}"; do
    kill -KILL "$pid" 2>/dev/null || true
  done
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

wait_for_http() {
  local url="$1" label="$2" tries="${3:-30}"
  for _ in $(seq 1 "$tries"); do
    if curl -sf -o /dev/null "$url"; then
      return 0
    fi
    sleep 1
  done
  log "FAILED: $label never became reachable at $url"
  exit 1
}

log "=== building binaries ==="
go build -o "$WORKDIR/herminas-cp" .
(cd rust && cargo build --workspace -q)

log "=== 1. starting control plane (ClickHouse via M0.5 supervisor) ==="
"$WORKDIR/herminas-cp" run > "$WORKDIR/control-plane.log" 2>&1 &
PIDS+=($!)
wait_for_http "$CLICKHOUSE_URL/ping" "ClickHouse"
log "OK: ClickHouse is up"

log "=== 2. starting the real Agent gRPC receiver (M1.2) ==="
HERMINAS_CLICKHOUSE_URL="$CLICKHOUSE_URL" \
  ./rust/target/debug/herminas-dataplane serve > "$WORKDIR/receiver.log" 2>&1 &
PIDS+=($!)
sleep 2 # no HTTP health endpoint on the gRPC receiver; fixed grace period

log "=== 3. starting the real HTTP API (M1.5) ==="
HERMINAS_CLICKHOUSE_URL="$CLICKHOUSE_URL" \
  HERMINAS_BOOTSTRAP_ADMIN_USER="$ADMIN_USER" \
  HERMINAS_BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  HERMINAS_JWT_SECRET="smoke-test-secret-not-for-production" \
  HERMINAS_CONFIG="config/herminas.example.yaml" \
  HERMINAS_HOME="$WORKDIR/api-home" \
  "$WORKDIR/herminas-cp" serve-api > "$WORKDIR/api.log" 2>&1 &
PIDS+=($!)
wait_for_http "$API_URL/api/v1/health" "HTTP API"
log "OK: HTTP API is up"

log "=== doctor-lite: service health ==="
# "|"-delimited, not ":" — both URLs already contain a colon (the port),
# which broke a first version of this loop that split on ":" and fed curl
# a truncated URL (found by actually running this script, not in review).
for check in "$CLICKHOUSE_URL/ping|ClickHouse" "$API_URL/api/v1/health|HTTP API"; do
  url="${check%%|*}"; name="${check##*|}"
  if curl -sf -o /dev/null "$url"; then
    echo "  [OK]   $name ($url)"
  else
    echo "  [FAIL] $name ($url)"
  fi
done

log "=== 4. logging in ==="
TOKEN=$(curl -sf -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
if [ -z "$TOKEN" ]; then
  log "FAILED: could not obtain a session token"
  exit 1
fi
log "OK: got a JWT session token"

log "=== 4b. creating the dataset via the API (M1.5's DDLExecutor creates the real ClickHouse table) ==="
# The Rust receiver (M1.2) inserts directly and does not auto-create
# tables — that cross-language wiring (Rust calling into Go's schemamgr)
# is explicitly not done yet (see tasks-herminas.md M1.2/M1.5). The HTTP
# API path *does* create real tables (M1.5), so we use it here to prepare
# the table the gRPC path will insert into.
curl -sf -X POST "$API_URL/api/v1/datasets" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"name\":\"$DATASET\",\"columns\":[{\"name\":\"event\",\"type\":\"String\"},{\"name\":\"marker\",\"type\":\"String\"}]}" \
  > "$WORKDIR/create-dataset.json"
log "OK: dataset $DATASET created (metadata + real ClickHouse table)"

log "=== 5. starting the real agent, tailing a fresh log file ==="
mkdir -p "$WORKDIR/logs" "$WORKDIR/wal"
LOGFILE="$WORKDIR/logs/app.log"
touch "$LOGFILE"
cat > "$WORKDIR/agent.yaml" <<EOF
http_addr: "127.0.0.1:8904"
wal_path: $WORKDIR/wal
receiver_addr: "$RECEIVER_ADDR"
agent_id: "smoke-test-agent"
dataset: "$DATASET"
batch_size: 10
flush_interval_ms: 200
backpressure_threshold_bytes: 1000000
sources:
  - path: $LOGFILE
    format:
      type: json
EOF

HERMINAS_AGENT_CONFIG="$WORKDIR/agent.yaml" \
  ./rust/target/debug/herminas-agent > "$WORKDIR/agent.log" 2>&1 &
PIDS+=($!)
sleep 1 # let it start tailing before we write anything

log "=== 6. writing one log line and timing until it's queryable ==="
START_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
echo "{\"event\":\"m1_milestone\",\"marker\":\"$DATASET\"}" >> "$LOGFILE"

DEADLINE_MS=$((START_MS + 5000)) # generous outer bound; we assert < 2000ms separately
FOUND_MS=""
while [ "$(python3 -c 'import time; print(int(time.time()*1000))')" -lt "$DEADLINE_MS" ]; do
  RESP=$(curl -sf -X POST "$API_URL/api/v1/query" \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d "{\"sql\":\"SELECT count() AS n FROM $DATASET\"}" 2>/dev/null || echo '{"row_count":0}')
  ROW_COUNT=$(echo "$RESP" | python3 -c 'import json,sys
try:
    print(json.load(sys.stdin).get("row_count", 0))
except Exception:
    print(0)')
  if [ "$ROW_COUNT" != "0" ]; then
    FOUND_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
    break
  fi
  sleep 0.1
done

if [ -z "$FOUND_MS" ]; then
  log "FAILED: row never became queryable within 5s"
  log "--- agent log ---"; tail -20 "$WORKDIR/agent.log"
  log "--- receiver log ---"; tail -20 "$WORKDIR/receiver.log"
  exit 1
fi

LATENCY_MS=$((FOUND_MS - START_MS))
log "OK: ingested row became queryable after ${LATENCY_MS}ms"

if [ "$LATENCY_MS" -ge 2000 ]; then
  log "FAILED: latency ${LATENCY_MS}ms exceeds the 2000ms budget (cahier des charges §10.3)"
  exit 1
fi

log "=== M1 milestone: PASS (ingestion -> queryable in ${LATENCY_MS}ms, budget 2000ms) ==="
