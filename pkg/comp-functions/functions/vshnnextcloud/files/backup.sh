#!/bin/bash

set -e

# We need to run the occ command as the www-data user, but there's no sudo by default
apt update 1>&2 && apt install sudo -y 1>&2

function disableMaintenance {
  >&2 echo "Disabling maintenance"
  sudo -u www-data /var/www/html/occ maintenance:mode --off 1>&2
}

trap disableMaintenance EXIT

sudo -u www-data /var/www/html/occ maintenance:mode --on 1>&2

tar -cf - /var/www
