#!/bin/bash

# This script works both with enabled and disabled AOF. It ensures that there
# are no AOF rewritings happening at the time of backup. If there's a dump file
# it will also backup that one.

redis_cmd() {
  if [ -z "$REDIS_TLS_CERT_FILE" ]; then 
    redis-cli -a "${REDIS_PASSWORD}" "$@"
  else
    redis-cli -a "${REDIS_PASSWORD}" --cacert "${REDIS_TLS_CA_FILE}" --cert "${REDIS_TLS_CERT_FILE}" --key "${REDIS_TLS_KEY_FILE}" --tls "$@"
  fi
}

rewrite_percentage=$(redis_cmd --raw CONFIG GET auto-aof-rewrite-percentage | tail -n 1) 1>&2

redis_cmd CONFIG SET auto-aof-rewrite-percentage 0 1>&2

redis_cmd SAVE 1>&2

until redis_cmd INFO persistence | grep aof_rewrite_in_progress:0 > /dev/null
do
  sleep 1
done

tar -cf - /data

redis_cmd CONFIG SET auto-aof-rewrite-percentage "${rewrite_percentage}" 1>&2
