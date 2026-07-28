#!/bin/sh
# Waits for PostgreSQL to accept real queries before Keycloak starts.
# Runs on every Keycloak pod start; prevents startup before DB is truly ready.
#
# Uses psql rather than pg_isready on purpose: pg_isready reports success as soon
# as the postmaster answers a startup packet, which happens while it is still
# starting up and before it accepts authenticated sessions. Keycloak's JDBC pool
# opens a fully authenticated TLS connection, so this check must do the same or
# it reports OK too early and Keycloak still crashes on connect.
#
# Connection parameters come from the standard PG* env vars, including
# PGSSLMODE/PGSSLROOTCERT, so this mirrors Keycloak's JDBC settings.
# Exits with error after MAX_TRIES attempts (~10 minutes) to surface misconfiguration.

MAX_TRIES=300
# A CNPG primary can briefly accept connections mid-promotion, so require
# several consecutive successes before declaring the database ready.
REQUIRED_STREAK=3

tries=0
streak=0

echo 'Waiting for Database to become ready...'

while true; do
    if psql --no-password --quiet --tuples-only --no-align --command='SELECT 1' >/dev/null 2>&1; then
        streak=$((streak + 1))
        if [ "$streak" -ge "$REQUIRED_STREAK" ]; then
            break
        fi
    else
        streak=0
    fi

    tries=$((tries + 1))
    if [ "$tries" -ge "$MAX_TRIES" ]; then
        echo
        echo "ERROR: Database did not become ready after ${MAX_TRIES} attempts" >&2
        # The loop silences psql; run once more to surface the actual error.
        psql --no-password --command='SELECT 1' >&2
        exit 1
    fi
    printf '.'
    sleep 2
done

echo
echo 'Database OK ✓'
