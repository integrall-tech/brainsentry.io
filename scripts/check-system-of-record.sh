#!/usr/bin/env bash
# check-system-of-record.sh — CI gate for system of record discipline.
#
# Walks every up-migration and asserts that any CREATE TABLE statement is
# either:
#   1. Listed in the canonical table set inside
#      brain-sentry-go/docs/architecture/system-of-record.md  (canonical), or
#   2. Preceded in the same file by a comment line starting with
#      "-- DERIVED:" naming the rebuild flag that owns it.
#
# Exit 1 if any new table fails the check. Designed to run in CI before
# merge — local devs can run it ahead of opening a PR.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIG_DIR="$ROOT/brain-sentry-go/internal/repository/postgres/migrations"
DOC="$ROOT/brain-sentry-go/docs/architecture/system-of-record.md"

if [ ! -d "$MIG_DIR" ]; then
  echo "error: migrations dir not found at $MIG_DIR" >&2
  exit 2
fi
if [ ! -f "$DOC" ]; then
  echo "error: system-of-record.md not found at $DOC" >&2
  exit 2
fi

# 1. Extract the set of canonical table names from the doc.
#    Pattern: lines like "| `tablename` | description |"
canonical=$(grep -oE '^\| `[a-z_][a-z0-9_]*` ' "$DOC" \
  | tr -d '|`' \
  | tr -d ' ' \
  | sort -u || true)

if [ -z "$canonical" ]; then
  echo "error: no canonical tables parsed from $DOC — has the format changed?" >&2
  exit 2
fi

# 2. For every up-migration, find CREATE TABLE statements and check.
#    Use process substitution (not a pipe) so `fail=1` survives outside
#    the loop — bash gotcha: pipes spawn a subshell.
fail=0
for mig in "$MIG_DIR"/*.up.sql; do
  [ -e "$mig" ] || continue
  base=$(basename "$mig")
  awk_out=$(awk '
    BEGIN { derived = 0 }
    /^-- DERIVED:/ { derived = 1; next }
    /^[[:space:]]*--/ { next }
    /^[[:space:]]*$/ { next }
    /CREATE TABLE/ {
      line = $0
      sub(/.*CREATE TABLE([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?[[:space:]]+/, "", line)
      sub(/[[:space:](].*$/, "", line)
      gsub(/"/, "", line)
      sub(/^[a-zA-Z_][a-zA-Z0-9_]*\./, "", line)
      print NR "\t" line "\t" derived
      derived = 0
      next
    }
    { derived = 0 }
  ' "$mig")
  while IFS=$'\t' read -r lineno table derived; do
    [ -n "$table" ] || continue
    if echo "$canonical" | grep -qx "$table"; then
      continue
    fi
    if [ "$derived" = "1" ]; then
      continue
    fi
    echo "FAIL: $base:$lineno introduces table '$table' that is neither in"
    echo "      $DOC's canonical list nor preceded by '-- DERIVED: <flag>'."
    echo "      Either add it to the canonical table in the doc, or tag the"
    echo "      migration with '-- DERIVED: rebuild --<flag>' immediately"
    echo "      above the CREATE TABLE statement."
    fail=1
  done <<< "$awk_out"
done

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "ok: every CREATE TABLE in migrations is classified."
