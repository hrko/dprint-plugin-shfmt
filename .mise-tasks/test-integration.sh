#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_ROOT="$ROOT_DIR/.tmp"
CACHE_ROOT="$TMP_ROOT/cache"
CACHE_DIR="$TMP_ROOT/dprint-cache"
TEMP_DIR="$TMP_ROOT/tmp"
TEST_DIR="$TMP_ROOT/integration-test"
WASM_PATH="$TMP_ROOT/plugin-integration.wasm"
CONFIG_PATH="$TEST_DIR/dprint.json"
TEST_FILE="$TEST_DIR/sample.sh"
EXPECTED_FILE="$TEST_DIR/expected.sh"
ACTUAL_FILE="$TEST_DIR/actual.sh"

mkdir -p "$CACHE_ROOT" "$CACHE_DIR" "$TEMP_DIR"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

export XDG_CACHE_HOME="$CACHE_ROOT"
export DPRINT_CACHE_DIR="$CACHE_DIR"
export TMPDIR="$TEMP_DIR"
export TMP="$TEMP_DIR"
export TEMP="$TEMP_DIR"

run_dprint() {
  dprint --log-level debug "$@"
}

cleanup() {
  rm -rf "$TMP_ROOT"
}

on_failure() {
  echo >&2
  echo "Integration test failed. Dumping debug information..." >&2
  run_dprint output-resolved-config --config "$CONFIG_PATH" "$TEST_FILE" >&2 || true
}
trap on_failure ERR
trap cleanup EXIT

tinygo build \
  -o "$WASM_PATH" \
  -target=wasm-unknown \
  -scheduler=none \
  -panic=trap \
  "$ROOT_DIR/main.go"

cat > "$CONFIG_PATH" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

cat > "$TEST_FILE" <<'SH'
if [ "$1" = "ok" ];then
 echo ok
fi
SH

cat > "$EXPECTED_FILE" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

run_dprint fmt --config "$CONFIG_PATH" --stdin sample.sh < "$TEST_FILE" > "$ACTUAL_FILE"

diff -u "$EXPECTED_FILE" "$ACTUAL_FILE"

echo "Integration test passed."
