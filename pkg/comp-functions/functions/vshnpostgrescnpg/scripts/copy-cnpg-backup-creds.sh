#!/bin/sh

set -e

xrdtype="${CLAIM_TYPE}"

if [ "${xrdtype}" != "" ]
then
    xrdservicename=$(kubectl -n "${CLAIM_NAMESPACE}" get "${xrdtype#?}" "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')
    xrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get "${xrdtype}" "${xrdservicename}" -ojson | jq -r '.spec.resourceRefs | map(select(.kind == "XVSHNPostgreSQL")) | .[].name')
else
    xrdname=$(kubectl -n "${CLAIM_NAMESPACE}" get vshnpostgresqls "${CLAIM_NAME}" -ojson | jq -r '.spec.resourceRef.name')
fi

echo "resolved source composite: ${xrdname}"

# Get source instance's major version
source_major_version=$(kubectl get xvshnpostgresqls "${xrdname}" -ojson | jq -r '.spec.parameters.service.majorVersion')
echo "source major version: ${source_major_version}"

# Read backup bucket credentials from the Crossplane namespace
# The XObjectBucket writes its connection secret here
cred_secret="backup-bucket-credentials-${xrdname}"
echo "reading bucket credentials from ${CROSSPLANE_NAMESPACE}/${cred_secret}"
creds=$(kubectl -n "${CROSSPLANE_NAMESPACE}" get secret "${cred_secret}" -ojson)

# Extract fields (already base64-encoded from .data)
access_key_id=$(echo "${creds}" | jq -r '.data.AWS_ACCESS_KEY_ID')
secret_access_key=$(echo "${creds}" | jq -r '.data.AWS_SECRET_ACCESS_KEY')
bucket_name=$(echo "${creds}" | jq -r '.data.BUCKET_NAME')
region=$(echo "${creds}" | jq -r '.data.AWS_REGION')
endpoint_url=$(echo "${creds}" | jq -r '.data.ENDPOINT_URL')

# Create recovery credentials secret in the target namespace
echo "creating recovery credentials secret in ${TARGET_NAMESPACE}"
cat <<EOSECRET | kubectl -n "${TARGET_NAMESPACE}" apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: cnpg-recovery-credentials
data:
  AWS_ACCESS_KEY_ID: ${access_key_id}
  AWS_SECRET_ACCESS_KEY: ${secret_access_key}
  BUCKET_NAME: ${bucket_name}
  AWS_REGION: ${region}
  ENDPOINT_URL: ${endpoint_url}
  SOURCE_MAJOR_VERSION: $(echo -n "${source_major_version}" | base64)
EOSECRET
