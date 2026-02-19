#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$ROOT_DIR/.tmp/isolation"
TEST_DIR="$TMP_DIR/test"
LOG_DIR="$TMP_DIR/logs"

mkdir -p "$TEST_DIR" "$LOG_DIR" "$TMP_DIR/dprint-cache" "$TMP_DIR/cache"

export DPRINT_CACHE_DIR="$TMP_DIR/dprint-cache"
export XDG_CACHE_HOME="$TMP_DIR/cache"

run_case() {
  local case_name="$1"
  local package_path="$2"
  local wasm_path="$TMP_DIR/${case_name}.wasm"
  local config_path="$TEST_DIR/dprint-${case_name}.json"
  local log_path="$LOG_DIR/${case_name}.log"

  tinygo build \
    -o "$wasm_path" \
    -target=wasm-unknown \
    -scheduler=none \
    -panic=trap \
    -no-debug \
    "$package_path"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$wasm_path"],
  "isolation": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  local exit_code=0
  if ! dprint --log-level debug output-resolved-config --config "$config_path" >"$log_path" 2>&1; then
    exit_code=$?
  fi

  if grep -q "RuntimeError: unreachable" "$log_path"; then
    echo "$case_name: exit=$exit_code (RuntimeError: unreachable)"
  else
    echo "$case_name: exit=$exit_code"
  fi
  echo "log: $log_path"
}

run_case "case_no_maps" "./investigation/isolation/case_no_maps"
run_case "case_maps_copy" "./investigation/isolation/case_maps_copy"
run_case "case_import_syntax" "./investigation/isolation/case_import_syntax"
run_case "case_stash_835cd96" "./investigation/isolation/case_stash_835cd96"
run_case "case_syntax_json_plugininfo" "./investigation/isolation/case_syntax_json_plugininfo"
run_case "case_syntax_json_register_config" "./investigation/isolation/case_syntax_json_register_config"
run_case "case_syntax_json_register_typed" "./investigation/isolation/case_syntax_json_register_typed"
run_case "case_syntax_json_unmarshal_only" "./investigation/isolation/case_syntax_json_unmarshal_only"
run_case "case_syntax_json_resolved_marshal_const" "./investigation/isolation/case_syntax_json_resolved_marshal_const"
run_case "case_syntax_store_literal" "./investigation/isolation/case_syntax_store_literal"
run_case "case_syntax_store_literal_no_marshal" "./investigation/isolation/case_syntax_store_literal_no_marshal"
run_case "case_syntax_global_map_unused" "./investigation/isolation/case_syntax_global_map_unused"
run_case "case_syntax_store_literal_no_read" "./investigation/isolation/case_syntax_store_literal_no_read"
run_case "case_syntax_slice_store" "./investigation/isolation/case_syntax_slice_store"

echo
echo "Isolation run complete."
