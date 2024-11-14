#!/bin/bash -x

IS_OPENSHIFT=$1
NEXTCLOUD_INSTANCE=$2
COLLABORA_FQDN=$3
COLLABORA_URL="https://$COLLABORA_FQDN"

if [ "$IS_OPENSHIFT" == "true" ]; then

    while ! /var/www/html/occ status | grep "installed: true" ; do
        echo "Nextcloud is not ready yet"
        sleep 5
    done

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
    /var/www/html/occ richdocuments:activate-config
else
    while ! /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ status" | grep "installed: true" ; do
        echo "Nextcloud is not ready yet"
        sleep 5
    done

    #check if collabora is already installed
    /usr/bin/su -s /bin/bash www-data -c  "/var/www/html/occ status" | grep "installed: true" 

    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ app:get richdocuments"
    RC=$?
    if [ "$RC" == 0 ]; then
        echo "Collabora already installed"
        exit 0
    fi
    set -e
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ app:install richdocuments"
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ config:app:set --value \"$COLLABORA_URL\" richdocuments wopi_url"
    /usr/bin/su -s /bin/sh www-data -c  "/var/www/html/occ richdocuments:activate-config"
fi

echo "Collabora installed, wopi_url configured and app enabled!"