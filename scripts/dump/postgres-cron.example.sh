#!/usr/bin/env bash
# Tier 1 example: run on the DB host via cron; CosmosBDR backs up REMOTE_PATH with sftp (no pre_backup).
set -euo pipefail

REMOTE_ROOT="${REMOTE_ROOT:?set REMOTE_ROOT e.g. /var/backups}"
REL_PATH="${REL_PATH:-cosmosbdr/nightly.dump}"
DATABASE="${PGDATABASE:?}"

OUT="${REMOTE_ROOT%/}/${REL_PATH}"
mkdir -p "$(dirname "$OUT")"
export PGPASSWORD="${PGPASSWORD:?}"

pg_dump -Fc -f "$OUT" "$DATABASE"