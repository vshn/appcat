#!/bin/bash

MARIADB_ROOT_PASSWORD=$(cat "${MARIADB_ROOT_PASSWORD_FILE}")

mariadb-backup --user=root --password="$MARIADB_ROOT_PASSWORD" --backup --stream=xbstream
