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

# tar exits with 1 when a file changed while it was read. Without maintenance mode
# that happens very frequently. The archive is still complete, so only a higher exit
# code is a real error.
set +e
tar -cf - /var/www
tar_rc=$?
set -e

if [ "$tar_rc" -gt 1 ]; then
  >&2 echo "tar failed with exit code $tar_rc"
  exit "$tar_rc"
fi

if [ "$tar_rc" -eq 1 ]; then
  >&2 echo "tar reported files that changed during the backup, ignoring"
fi

exit 0
