#!/bin/bash -xef

# If the path below doesn't exist, forgejo dump will return exit code 1
[ ! -d /data/git/repositories ] && mkdir -p /data/git/repositories

/usr/local/bin/forgejo dump --type tar -t /tmp/backup -V -f -
