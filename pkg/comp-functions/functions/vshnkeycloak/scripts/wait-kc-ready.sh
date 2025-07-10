#!/bin/sh

set -e

echo "Checking Keycloak readiness at $KEYCLOAK_API_URL:9000/health/ready"
for i in $(seq 1 72); do # 72 * 5 = 360 seconds
  STATUS=$(curl -s -f -k --max-time 5 "$KEYCLOAK_API_URL:9000/health/ready" | grep status | xargs | awk '{print $2}' | sed 's/,//' 2>/dev/null || echo "CURL_ERROR")
  [ "$STATUS" = "UP" ] && break

  echo "Keycloak not ready yet (attempt $i), status: '$STATUS'"
  sleep 5
done

if [ "$STATUS" != "UP" ]; then
  echo "Keycloak was not ready in time"
  exit 1
fi

echo "Keycloak is $STATUS, stalling for $STALL_SECONDS seconds..."
sleep $STALL_SECONDS
