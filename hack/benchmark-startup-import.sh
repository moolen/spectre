#!/usr/bin/env bash

set -euo pipefail

OUT_DIR="${OUT_DIR:-/tmp/spectre-startup-import-$(date +%Y%m%d-%H%M%S)}"
SEED="${SEED:-42}"
KINDS="${KINDS:-55}"
RESOURCES="${RESOURCES:-5000}"
NAMESPACES="${NAMESPACES:-20}"
BENCHMARK_TIMEOUT_SECONDS="${BENCHMARK_TIMEOUT_SECONDS:-300}"

DATA_DIR="$OUT_DIR/data"
REPORT_PATH="$OUT_DIR/import-report.json"
SERVER_LOG_PATH="$OUT_DIR/server.log"
GENERATOR_SUMMARY_PATH="$OUT_DIR/generator-summary.json"
INTEGRATIONS_CONFIG_PATH="$OUT_DIR/integrations.yaml"

mkdir -p "$OUT_DIR"

echo "Output directory: $OUT_DIR"
echo "Building spectre binary..."
make build

echo "Ensuring FalkorDB is running..."
make graph-up

echo "Generating synthetic import dataset..."
go run ./cmd/spectre debug generate-import-data \
  --output-dir "$DATA_DIR" \
  --seed "$SEED" \
  --kinds "$KINDS" \
  --resources "$RESOURCES" \
  --namespaces "$NAMESPACES" >"$GENERATOR_SUMMARY_PATH"

echo "Starting spectre server with startup import..."
./bin/spectre server \
  --graph-enabled=true \
  --graph-host=localhost \
  --graph-port=6379 \
  --watcher-enabled=false \
  --reconciler-enabled=false \
  --integrations-config "$INTEGRATIONS_CONFIG_PATH" \
  --import-path "$DATA_DIR" \
  --import-benchmark-log "$REPORT_PATH" >"$SERVER_LOG_PATH" 2>&1 &
SERVER_PID=$!

cleanup() {
  if kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill -INT "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "Waiting for benchmark report: $REPORT_PATH"
elapsed=0
until [ -f "$REPORT_PATH" ]; do
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "spectre server exited before writing benchmark report"
    echo "server log:"
    cat "$SERVER_LOG_PATH"
    exit 1
  fi

  if [ "$elapsed" -ge "$BENCHMARK_TIMEOUT_SECONDS" ]; then
    echo "timed out after ${BENCHMARK_TIMEOUT_SECONDS}s waiting for benchmark report"
    echo "server log:"
    cat "$SERVER_LOG_PATH"
    exit 1
  fi

  sleep 1
  elapsed=$((elapsed + 1))
done

echo "Benchmark report written:"
cat "$REPORT_PATH"
echo
echo "Server log: $SERVER_LOG_PATH"
