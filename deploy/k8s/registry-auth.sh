#!/usr/bin/env bash

# Extract the cluster pull-secret and create the registry-auth secret

oc get secret pull-secret -n openshift-config \
  -o jsonpath='{.data.\.dockerconfigjson}' \
  | base64 -d > /tmp/pull-secret.json

echo "======== Copied pull-secret from openshift-config to /tmp/pull-secret.json ========"
echo " "

kubectl create secret generic registry-auth \
  --namespace image-roundup \
  --from-file=config.json=/tmp/pull-secret.json \
  --dry-run=client -o yaml \
  | kubectl apply -f -

echo "======== Copied pull-secret from openshift-config to /tmp/pull-secret.json ========"
echo " "

rm /tmp/pull-secret.json
echo "======== Cleaned up /tmp/pull-secret.json ========"
echo " "