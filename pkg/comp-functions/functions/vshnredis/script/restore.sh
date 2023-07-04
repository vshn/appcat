#!/bin/bash

set -euo pipefail

rm -rf /data/*
restic dump "${BACKUP_NAME}" "${BACKUP_PATH}" > /data/restore.tar
cd /data
tar xvf restore.tar
mv data/* .
rmdir data
rm restore.tar
