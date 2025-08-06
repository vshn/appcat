#!/bin/bash

set -euo pipefail

DEST="/data"

echo "Starting restore process"

echo "Removing existing files in ${DEST}"
rm -rf "${DEST:?}/"*

echo "Dumping restic backup '${BACKUP_NAME}' path '${BACKUP_PATH}' to ${DEST}/restore.tar"
restic dump "${BACKUP_NAME}" "${BACKUP_PATH}" > "${DEST}/restore.tar"

echo "Extracting files from restore.tar into ${DEST}"
tar xvf "${DEST}/restore.tar" --strip-components=1 -C "${DEST}"

echo "Removing temporary archive ${DEST}/restore.tar"
rm "${DEST}/restore.tar"

echo "Restore completed successfully"
