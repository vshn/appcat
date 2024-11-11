#!/bin/bash -x

IS_OPENSHIFT=$1
NAMESPACE=$2
NEXTCLOUD_INSTANCE=$3
COLLABORA_FQDN=$4
COLLABORA_URL="https://$COLLABORA_FQDN"

if [ "$IS_OPENSHIFT" == "true" ]; then
    #check if collabora is already installed
    /var/www/html/occ app:get richdocuments
    RC=$?
    if [ "$RC" == 0 ]; then
        echo "Collabora already installed"
        exit 0
    fi
    set -e
    /var/www/html/occ app:install richdocuments
    /var/www/html/occ config:app:set --value "$COLLABORA_URL" richdocuments wopi_url
    /var/www/html/occ app:enable richdocuments
else
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ app:get richdocuments"
    RC=$?
    if [ "$RC" == 0 ]; then
        echo "Collabora already installed"
        exit 0
    fi
    set -e
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ app:install richdocuments"
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ config:app:set --value $COLLABORA_URL richdocuments wopi_url"
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ app:enable richdocuments"
fi

echo "Collabora installed, wopi_url configured and app enabled!"