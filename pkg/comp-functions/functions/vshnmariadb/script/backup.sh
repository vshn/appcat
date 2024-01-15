#!/bin/bash

mariadb-backup --user=root --password="$MARIADB_ROOT_PASSWORD" --backup --stream=xbstream
