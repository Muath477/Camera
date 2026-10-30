#!/usr/bin/env bash
# Issues a client certificate for one device. The CN is the device id the
# platform assigned at claim time and the OU is its tenant: the broker uses
# the CN as the username and the ACL confines the device to its own topics;
# the device API reads both to place uploads.
#   ./issue-device.sh <device-id> <tenant-id>
# Copy out/devices/<id>.crt, <id>.key and ca.crt to /etc/camera/ on the box
# and point agent.yaml at them.
set -euo pipefail
cd "$(dirname "$0")"
ID="${1:?usage: issue-device.sh <device-id> <tenant-id>}"
TENANT="${2:?usage: issue-device.sh <device-id> <tenant-id>}"
for v in "$ID" "$TENANT"; do
  [[ "$v" =~ ^[0-9a-f-]{36}$ ]] || { echo "ids must be uuids"; exit 1; }
done
mkdir -p out/devices
openssl ecparam -genkey -name prime256v1 -noout -out "out/devices/$ID.key"
openssl req -new -key "out/devices/$ID.key" -subj "/CN=$ID/OU=$TENANT" -out "out/devices/$ID.csr"
printf "extendedKeyUsage=clientAuth\n" > "out/devices/$ID.ext"
openssl x509 -req -in "out/devices/$ID.csr" -CA out/ca.crt -CAkey out/ca.key -CAcreateserial -days 1095 -sha256 -extfile "out/devices/$ID.ext" -out "out/devices/$ID.crt"
rm "out/devices/$ID.csr" "out/devices/$ID.ext"
chmod 600 "out/devices/$ID.key"
openssl x509 -in "out/devices/$ID.crt" -noout -fingerprint -sha256
