#!/bin/bash

set -e

# Determine if we need to switch users to run occ
OCC_OWNER=$(stat -c '%U' /var/www/html/occ 2>/dev/null || echo "www-data")
CURRENT_USER=$(whoami)

run_occ() {
  if [ "$CURRENT_USER" = "$OCC_OWNER" ]; then
    /var/www/html/occ "$@"
  else
    runuser -u "$OCC_OWNER" -- /var/www/html/occ "$@"
  fi
}

function disableMaintenance {
  >&2 echo "Disabling maintenance"
  run_occ maintenance:mode --off 1>&2
}

if [ "$SKIP_MAINTENANCE" = false ]; then

  trap disableMaintenance EXIT

  run_occ maintenance:mode --on 1>&2
fi

tar -cf - /var/www
