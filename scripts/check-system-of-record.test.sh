#!/usr/bin/env bash
# self-test for check-system-of-record.sh
#
# Builds a temporary fake repo layout, drops in synthetic migrations and
# the doc, then asserts the gate accepts/rejects the right cases. Run as
# part of CI alongside the gate itself so a refactor of the parser cannot
# silently weaken the contract.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GATE="$ROOT/scripts/check-system-of-record.sh"

if [ ! -x "$GATE" ]; then
  echo "error: gate not executable: $GATE" >&2
  exit 2
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/brain-sentry-go/internal/repository/postgres/migrations"
mkdir -p "$tmp/brain-sentry-go/docs/architecture"
mkdir -p "$tmp/scripts"
cp "$GATE" "$tmp/scripts/check-system-of-record.sh"

doc="$tmp/brain-sentry-go/docs/architecture/system-of-record.md"
cat > "$doc" <<'EOF'
# System of Record (test)

| Table        | What it owns        |
|--------------|---------------------|
| `memories`   | canonical memories  |
| `users`      | accounts            |
EOF

# Case 1: passing — only canonical tables.
mig="$tmp/brain-sentry-go/internal/repository/postgres/migrations/000001_pass.up.sql"
cat > "$mig" <<'EOF'
CREATE TABLE memories ( id varchar primary key );
CREATE TABLE users ( id varchar primary key );
EOF
if ! ( cd "$tmp" && ./scripts/check-system-of-record.sh > /dev/null ); then
  echo "FAIL: gate rejected a fully-canonical migration" >&2
  exit 1
fi

# Case 2: passing — a derived table with the comment marker.
cat > "$mig" <<'EOF'
CREATE TABLE memories ( id varchar primary key );
CREATE TABLE users ( id varchar primary key );
-- DERIVED: rebuild --communities
CREATE TABLE communities_cache ( id varchar primary key );
EOF
if ! ( cd "$tmp" && ./scripts/check-system-of-record.sh > /dev/null ); then
  echo "FAIL: gate rejected a properly-tagged derived table" >&2
  exit 1
fi

# Case 3: failing — an unclassified new table.
cat > "$mig" <<'EOF'
CREATE TABLE memories ( id varchar primary key );
CREATE TABLE secret_unclassified ( id varchar primary key );
EOF
if ( cd "$tmp" && ./scripts/check-system-of-record.sh > /dev/null 2>&1 ); then
  echo "FAIL: gate accepted an unclassified table" >&2
  exit 1
fi

# Case 4: failing — comment marker not immediately above (intervening
# blank-line + statement should reset the marker).
cat > "$mig" <<'EOF'
-- DERIVED: rebuild --x
CREATE TABLE other ( id varchar primary key );
CREATE TABLE not_marked ( id varchar primary key );
EOF
if ( cd "$tmp" && ./scripts/check-system-of-record.sh > /dev/null 2>&1 ); then
  echo "FAIL: gate accepted a table that did not have the marker on the line above" >&2
  exit 1
fi

echo "ok: gate behaves as expected on 4 synthetic cases"
