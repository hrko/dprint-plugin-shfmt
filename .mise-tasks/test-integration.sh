#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_ROOT="$ROOT_DIR/.tmp"
CACHE_ROOT="$TMP_ROOT/cache"
CACHE_DIR="$TMP_ROOT/dprint-cache"
TEMP_DIR="$TMP_ROOT/tmp"
TEST_ROOT="$TMP_ROOT/integration-test"
WASM_PATH="$TMP_ROOT/plugin-integration.wasm"

LAST_CASE_NAME=""
LAST_CONFIG_PATH=""
LAST_STDOUT_PATH=""
LAST_STDERR_PATH=""

mkdir -p "$CACHE_ROOT" "$CACHE_DIR" "$TEMP_DIR"
rm -rf "$TEST_ROOT"
mkdir -p "$TEST_ROOT"

export XDG_CACHE_HOME="$CACHE_ROOT"
export DPRINT_CACHE_DIR="$CACHE_DIR"
export TMPDIR="$TEMP_DIR"
export TMP="$TEMP_DIR"
export TEMP="$TEMP_DIR"

run_dprint() {
  dprint --log-level debug "$@"
}

clean_stderr() {
  sed \
    -e '/^Compiling /d' \
    -e '/^libunwind: __unw_add_dynamic_fde: bad fde: FDE is really a CIE$/d' \
    "$1"
}

cleanup() {
  rm -rf "$TMP_ROOT"
}

on_failure() {
  echo >&2
  echo "Integration test failed at case: ${LAST_CASE_NAME:-unknown}" >&2
  if [[ -n "$LAST_CONFIG_PATH" && -f "$LAST_CONFIG_PATH" ]]; then
    echo "--- resolved config ---" >&2
    run_dprint output-resolved-config --config "$LAST_CONFIG_PATH" >&2 || true
  fi
  if [[ -n "$LAST_STDOUT_PATH" && -f "$LAST_STDOUT_PATH" ]]; then
    echo "--- stdout ---" >&2
    cat "$LAST_STDOUT_PATH" >&2 || true
  fi
  if [[ -n "$LAST_STDERR_PATH" && -f "$LAST_STDERR_PATH" ]]; then
    echo "--- stderr (raw) ---" >&2
    cat "$LAST_STDERR_PATH" >&2 || true
    echo "--- stderr (clean) ---" >&2
    clean_stderr "$LAST_STDERR_PATH" >&2 || true
  fi
}
trap on_failure ERR
trap cleanup EXIT

tinygo build \
  -o "$WASM_PATH" \
  -target=wasm-unknown \
  -scheduler=none \
  -panic=trap \
  -no-debug \
  "$ROOT_DIR"

create_case_dir() {
  local case_name="$1"
  local case_dir="$TEST_ROOT/$case_name"
  rm -rf "$case_dir"
  mkdir -p "$case_dir"
  printf '%s\n' "$case_dir"
}

run_fmt() {
  local config_path="$1"
  local virtual_path="$2"
  local input_path="$3"
  local stdout_path="$4"
  local stderr_path="$5"

  LAST_CONFIG_PATH="$config_path"
  LAST_STDOUT_PATH="$stdout_path"
  LAST_STDERR_PATH="$stderr_path"

  set +e
  dprint fmt --log-level info --config "$config_path" --stdin "$virtual_path" \
    < "$input_path" > "$stdout_path" 2> "$stderr_path"
  local exit_code=$?
  set -e
  printf '%s\n' "$exit_code"
}

assert_exit_code() {
  local expected="$1"
  local actual="$2"
  if [[ "$actual" != "$expected" ]]; then
    echo "Expected exit code $expected, got $actual" >&2
    return 1
  fi
}

assert_stderr_clean_empty() {
  local stderr_path="$1"
  local cleaned
  cleaned="$(clean_stderr "$stderr_path")"
  if [[ -n "$cleaned" ]]; then
    echo "Expected stderr to be empty after filtering noise." >&2
    printf '%s\n' "$cleaned" >&2
    return 1
  fi
}

assert_stderr_contains() {
  local stderr_path="$1"
  local expected="$2"
  if ! clean_stderr "$stderr_path" | grep -Fq "$expected"; then
    echo "Expected stderr to contain: $expected" >&2
    echo "Actual stderr (cleaned):" >&2
    clean_stderr "$stderr_path" >&2
    return 1
  fi
}

assert_stdout_empty() {
  local stdout_path="$1"
  if [[ -s "$stdout_path" ]]; then
    echo "Expected stdout to be empty, but got:" >&2
    cat "$stdout_path" >&2
    return 1
  fi
}

run_case_format_success() {
  LAST_CASE_NAME="format-success"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ];then
 echo ok
fi
SH

  cat > "$expected_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_no_change() {
  LAST_CASE_NAME="no-change"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$input_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_parse_error() {
  LAST_CASE_NAME="parse-error"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ]; then
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 1 "$exit_code"
  assert_stdout_empty "$stdout_path"
  assert_stderr_contains "$stderr_path" "must end with \"fi\""
}

run_case_variant_detection_by_extension() {
  LAST_CASE_NAME="variant-detection-by-extension"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.bash"
  local sh_stdout_path="$case_dir/stdout.sh"
  local sh_stderr_path="$case_dir/stderr.sh.txt"
  local bash_stdout_path="$case_dir/stdout.bash"
  local bash_stderr_path="$case_dir/stderr.bash.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh", "**/*.bash"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
a=(1 2)
printf '%s\n' "${a[0]}"
SH

  cat > "$expected_path" <<'SH'
a=(1 2)
printf '%s\n' "${a[0]}"
SH

  local sh_exit_code
  sh_exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$sh_stdout_path" "$sh_stderr_path")"
  assert_exit_code 1 "$sh_exit_code"
  assert_stdout_empty "$sh_stdout_path"
  assert_stderr_contains "$sh_stderr_path" "arrays are a bash/mksh feature"

  local bash_exit_code
  bash_exit_code="$(run_fmt "$config_path" "sample.bash" "$input_path" "$bash_stdout_path" "$bash_stderr_path")"
  assert_exit_code 0 "$bash_exit_code"
  diff -u "$expected_path" "$bash_stdout_path"
  assert_stderr_clean_empty "$bash_stderr_path"
}

run_case_shebang_precedence() {
  LAST_CASE_NAME="shebang-precedence"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
#!/usr/bin/env bash
a=(1 2)
printf '%s\n' "${a[0]}"
SH

  cat > "$expected_path" <<'SH'
#!/usr/bin/env bash
a=(1 2)
printf '%s\n' "${a[0]}"
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_global_override() {
  LAST_CASE_NAME="global-override"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "indentWidth": 8,
  "useTabs": true,
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
if true; then
echo hi
fi
SH

  cat > "$expected_path" <<'SH'
if true; then
	echo hi
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_comment_and_shebang_preservation() {
  LAST_CASE_NAME="comment-and-shebang-preservation"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
#!/usr/bin/env bash
# keep me
if [ "$1" = "ok" ];then
 echo ok # trailing
fi
SH

  cat > "$expected_path" <<'SH'
#!/usr/bin/env bash
# keep me
if [ "$1" = "ok" ]; then
  echo ok # trailing
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_use_tabs_option() {
  LAST_CASE_NAME="use-tabs-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 8,
    "useTabs": true
  }
}
JSON

  cat > "$input_path" <<'SH'
if true; then
echo hi
fi
SH

  cat > "$expected_path" <<'SH'
if true; then
	echo hi
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_binary_next_line_option() {
  LAST_CASE_NAME="binary-next-line-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false,
    "binaryNextLine": true
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$a" = "b" ] &&
  [ "$c" = "d" ]; then
  echo ok
fi
SH

  cat > "$expected_path" <<'SH'
if [ "$a" = "b" ] \
  && [ "$c" = "d" ]; then
  echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_switch_case_indent_option() {
  LAST_CASE_NAME="switch-case-indent-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false,
    "switchCaseIndent": true
  }
}
JSON

  cat > "$input_path" <<'SH'
case x in
a) echo a ;
esac
SH

  cat > "$expected_path" <<'SH'
case x in
  a) echo a ;;
esac
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_space_redirects_option() {
  LAST_CASE_NAME="space-redirects-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false,
    "spaceRedirects": true
  }
}
JSON

  cat > "$input_path" <<'SH'
echo hi >/tmp/x
SH

  cat > "$expected_path" <<'SH'
echo hi > /tmp/x
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_func_next_line_option() {
  LAST_CASE_NAME="func-next-line-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false,
    "funcNextLine": true
  }
}
JSON

  cat > "$input_path" <<'SH'
foo(){
echo hi
}
SH

  cat > "$expected_path" <<'SH'
foo()
{
  echo hi
}
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_minify_option() {
  LAST_CASE_NAME="minify-option"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false,
    "minify": true
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  cat > "$expected_path" <<'SH'
if [ "$1" = "ok" ];then
echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 0 "$exit_code"
  diff -u "$expected_path" "$stdout_path"
  assert_stderr_clean_empty "$stderr_path"
}

run_case_config_type_error_diagnostic() {
  LAST_CASE_NAME="config-type-error-diagnostic"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "funcNextLine": "invalid"
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 1 "$exit_code"
  assert_stdout_empty "$stdout_path"
  assert_stderr_contains "$stderr_path" "Expected 'funcNextLine' to be a boolean"
  assert_stderr_contains "$stderr_path" "Had 1 configuration errors."
}

run_case_unknown_property_diagnostic() {
  LAST_CASE_NAME="unknown-property-diagnostic"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local stdout_path="$case_dir/stdout.sh"
  local stderr_path="$case_dir/stderr.txt"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "unknownField": true
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  local exit_code
  exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
  assert_exit_code 1 "$exit_code"
  assert_stdout_empty "$stdout_path"
  assert_stderr_contains "$stderr_path" "Unknown property 'unknownField'."
  assert_stderr_contains "$stderr_path" "Had 1 configuration errors."
}

run_case_repeated_invocations_same_cache() {
  LAST_CASE_NAME="repeated-invocations-same-cache"
  local case_dir
  case_dir="$(create_case_dir "$LAST_CASE_NAME")"
  local config_path="$case_dir/dprint.json"
  local input_path="$case_dir/input.sh"
  local expected_path="$case_dir/expected.sh"

  cat > "$config_path" <<JSON
{
  "includes": ["**/*.sh"],
  "plugins": ["$WASM_PATH"],
  "shfmt": {
    "indentWidth": 2,
    "useTabs": false
  }
}
JSON

  cat > "$input_path" <<'SH'
if [ "$1" = "ok" ];then
 echo ok
fi
SH

  cat > "$expected_path" <<'SH'
if [ "$1" = "ok" ]; then
  echo ok
fi
SH

  local iteration
  for iteration in 1 2 3; do
    local stdout_path="$case_dir/stdout.$iteration.sh"
    local stderr_path="$case_dir/stderr.$iteration.txt"
    local exit_code
    exit_code="$(run_fmt "$config_path" "sample.sh" "$input_path" "$stdout_path" "$stderr_path")"
    assert_exit_code 0 "$exit_code"
    diff -u "$expected_path" "$stdout_path"
    assert_stderr_clean_empty "$stderr_path"
  done
}

run_case_format_success
run_case_no_change
run_case_parse_error
run_case_variant_detection_by_extension
run_case_shebang_precedence
run_case_global_override
run_case_comment_and_shebang_preservation
run_case_use_tabs_option
run_case_binary_next_line_option
run_case_switch_case_indent_option
run_case_space_redirects_option
run_case_func_next_line_option
run_case_minify_option
run_case_config_type_error_diagnostic
run_case_unknown_property_diagnostic
run_case_repeated_invocations_same_cache

echo "Integration tests passed."
