#!/usr/bin/env bash
# Creates the private CA that signs device and broker certificates.
# Run once; keep pki/ca.key offline afterwards (a copy in the password
# manager is enough) and commit nothing from pki/out.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p out
if [ -f out/ca.key ]; then echo "CA already exists in pki/out"; exit 0; fi
MQTT_DOMAIN="${1:?usage: init-ca.sh mqtt.example.sa device.example.sa}"
DEVICE_API_DOMAIN="${2:?usage: init-ca.sh mqtt.example.sa device.example.sa}"
openssl ecparam -genkey -name prime256v1 -noout -out out/ca.key
openssl req -x509 -new -key out/ca.key -sha256 -days 3650 -subj "/CN=Camera Device CA" -out out/ca.crt
# One server certificate for the broker and the device API, from the same CA:
# devices trust ca.crt alone, so no public CA is involved on the device path.
openssl ecparam -genkey -name prime256v1 -noout -out out/broker.key
openssl req -new -key out/broker.key -subj "/CN=${MQTT_DOMAIN}" -out out/broker.csr
printf "subjectAltName=DNS:%s,DNS:%s\nextendedKeyUsage=serverAuth\n" "$MQTT_DOMAIN" "$DEVICE_API_DOMAIN" > out/broker.ext
openssl x509 -req -in out/broker.csr -CA out/ca.crt -CAkey out/ca.key -CAcreateserial -days 825 -sha256 -extfile out/broker.ext -out out/broker.crt
rm out/broker.csr out/broker.ext
chmod 600 out/*.key
echo "CA and gateway certificate written to pki/out (renew the gateway certificate before $(date -d "+825 days" +%Y-%m-%d 2>/dev/null || echo "825 days"))"
