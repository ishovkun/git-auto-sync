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

# --- menubar icon ------------------------------------------------------------
# Render the Git logo as a monochrome template image (black in light mode,
# white in dark mode). The PNG lives next to this script; resolve through the
# symlink SwiftBar loads us as so we can find it.
SOURCE="${BASH_SOURCE[0]}"
while [ -L "$SOURCE" ]; do
  dir="$(cd -P "$(dirname "$SOURCE")" && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$dir/$SOURCE"
done
SCRIPT_DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
ICON_FILE="$SCRIPT_DIR/git-icon.png"

if [ -f "$ICON_FILE" ]; then
  ICON=$(base64 -i "$ICON_FILE" | tr -d '\n')
  echo " | templateImage=$ICON"
else
  echo " | sfimage=arrow.triangle.branch"
fi

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
