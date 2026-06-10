#!/bin/bash
#
# git-auto-sync menubar plugin for SwiftBar (https://swiftbar.app).
#
# Shows whether the daemon is running and the result of the most recent sync
# for each watched repo. Install by symlinking this file into your SwiftBar
# plugin directory, e.g.:
#
#   ln -s "$PWD/gitautosync.30s.sh" "$HOME/Library/Application Support/SwiftBar/gitautosync.30s.sh"
#
# The "30s" in the filename tells SwiftBar to refresh every 30 seconds.
#
# <bitbar.title>git-auto-sync</bitbar.title>
# <bitbar.desc>Status of the git-auto-sync daemon and watched repos.</bitbar.desc>

export PATH="$HOME/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"

GAS="$HOME/go/bin/git-auto-sync"
LABEL="git-auto-sync-daemon"
SUPPORT="$HOME/Library/Application Support/git-auto-sync"
STATUS_FILE="$SUPPORT/status.json"
LOG="$HOME/Library/Logs/${LABEL}.err.log"
JQ="$(command -v jq)"

# --- daemon running? ---------------------------------------------------------
if launchctl list 2>/dev/null | grep -q "$LABEL"; then
  RUNNING=1
else
  RUNNING=0
fi

# --- determine overall health ------------------------------------------------
# 0 = all good, 1 = a repo failed its last sync, 2 = daemon not running.
HEALTH=0
[ "$RUNNING" -eq 0 ] && HEALTH=2

if [ "$RUNNING" -eq 1 ] && [ -f "$STATUS_FILE" ] && [ -n "$JQ" ]; then
  ANY_FAIL=$("$JQ" -r '[.repos[] | select(.ok == false)] | length' "$STATUS_FILE" 2>/dev/null)
  [ "${ANY_FAIL:-0}" -gt 0 ] && HEALTH=1
fi

case "$HEALTH" in
  0) echo " | sfimage=arrow.triangle.2.circlepath.circle.fill sfcolor=green" ;;
  1) echo " | sfimage=exclamationmark.triangle.fill sfcolor=orange" ;;
  2) echo " | sfimage=xmark.circle.fill sfcolor=red" ;;
esac

echo "---"

# --- daemon line -------------------------------------------------------------
if [ "$RUNNING" -eq 1 ]; then
  echo "Daemon: running | color=green"
else
  echo "Daemon: stopped | color=red"
fi

echo "---"

# --- per-repo status ---------------------------------------------------------
now=$(date +%s)

human_age() {
  local secs=$1
  if   [ "$secs" -lt 60 ];    then echo "${secs}s ago"
  elif [ "$secs" -lt 3600 ];  then echo "$((secs / 60))m ago"
  elif [ "$secs" -lt 86400 ]; then echo "$((secs / 3600))h ago"
  else echo "$((secs / 86400))d ago"; fi
}

if [ -f "$STATUS_FILE" ] && [ -n "$JQ" ]; then
  # repo<TAB>ok<TAB>synced_at_unix<TAB>error
  "$JQ" -r '.repos[] | [.repo, (.ok|tostring), (.synced_at_unix|tostring), (.error // "")] | @tsv' "$STATUS_FILE" 2>/dev/null |
  while IFS=$'\t' read -r repo ok ts err; do
    name=$(basename "$repo")
    age="never"
    [ -n "$ts" ] && [ "$ts" != "null" ] && age=$(human_age $((now - ts)))
    if [ "$ok" = "true" ]; then
      echo "✓ ${name} — synced ${age} | color=green href=file://${repo}"
    else
      echo "✗ ${name} — failed ${age} | color=red href=file://${repo}"
      [ -n "$err" ] && echo "--${err} | font=Menlo size=11 color=red"
    fi
    echo "Sync now | bash=\"$GAS\" param1=sync param2=\"$repo\" terminal=false refresh=true"
  done
else
  echo "No status yet | color=gray"
fi

echo "---"
echo "Open error log | bash=/usr/bin/open param1=\"$LOG\" terminal=false"
echo "Open repo list | bash=/usr/bin/open param1=\"$SUPPORT\" terminal=false"
echo "Refresh | refresh=true"
