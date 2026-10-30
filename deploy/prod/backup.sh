#!/bin/bash
# Nightly logical backup of the database into the backups volume, keeping
# BACKUP_KEEP_DAYS days. Copy the volume off the machine (object storage in
# another region of the Kingdom) with the provider's snapshot tool.
set -e
while true; do
  now=$(date +%Y%m%d-%H%M)
  pg_dump -h postgres -U postgres -d camera -Fc -f "/backups/camera-$now.dump" && echo "backup $now ok"
  find /backups -name 'camera-*.dump' -mtime +"${BACKUP_KEEP_DAYS:-30}" -delete
  sleep 86400
done
