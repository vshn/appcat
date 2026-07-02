#!/bin/sh
# Waits for PostgreSQL to accept connections using pg_isready.
# Runs on every Keycloak pod start; prevents startup before DB is truly ready.
# Exits with error after MAX_TRIES attempts (~10 minutes) to surface misconfiguration.

MAX_TRIES=300
tries=0

echo 'Waiting for Database to become ready...'

until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" --timeout=5 -q; do
    tries=$((tries + 1))
    if [ "$tries" -ge "$MAX_TRIES" ]; then
        echo
        echo "ERROR: Database did not become ready after ${MAX_TRIES} attempts" >&2
        exit 1
    fi
    printf '.'
    sleep 2
done

echo
echo 'Database OK ✓'
