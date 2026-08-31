#!/bin/bash

set -eu

if [ "$(uname -s)" != "Darwin" ]; then
  exit 0
fi

SCRIPT_DIR="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN="$SCRIPT_DIR/gitautosync.30s.sh"
TEST_ROOT=$(mktemp -d)
TEST_HOME="$TEST_ROOT/home"
STATUS_DIR="$TEST_HOME/Library/Application Support/git-auto-sync"

cleanup() {
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

mkdir -p "$TEST_HOME/go/bin" "$STATUS_DIR"

cat > "$TEST_HOME/go/bin/launchctl" <<'EOF'
#!/bin/bash
if [ "${MOCK_DAEMON_RUNNING:-0}" -eq 1 ]; then
  printf '123\t0\tgit-auto-sync-daemon\n'
fi
EOF
chmod +x "$TEST_HOME/go/bin/launchctl"

write_status() {
  local ok=$1
  printf '{"repos":{"/tmp/notes":{"repo":"/tmp/notes","ok":%s,"synced_at_unix":1}}}\n' "$ok" > "$STATUS_DIR/status.json"
}

header_for() {
  local running=$1
  MOCK_DAEMON_RUNNING="$running" HOME="$TEST_HOME" "$PLUGIN" | sed -n '1p'
}

assert_contains() {
  local actual=$1
  local expected=$2
  case "$actual" in
    *"$expected"*) ;;
    *)
      printf 'expected header to contain %s, got:\n%s\n' "$expected" "$actual" >&2
      exit 1
      ;;
  esac
}

write_status true
healthy_header=$(header_for 1)
assert_contains "$healthy_header" 'templateImage='

write_status false
conflict_header=$(header_for 1)
assert_contains "$conflict_header" 'sfimage=xmark.circle.fill'
expected_config=$(printf '%s' '{"renderingMode":"Palette","colors":["white","red"]}' | base64 | tr -d '\n')
assert_contains "$conflict_header" "sfconfig=$expected_config"

write_status true
stopped_header=$(header_for 0)
assert_contains "$stopped_header" 'sfimage=xmark.circle.fill'

printf 'SwiftBar icon tests passed\n'
