#!/bin/bash
# Runs once when the data directory is created: the login role the API uses.
set -e
psql -v ON_ERROR_STOP=1 -U postgres -d camera <<SQL
CREATE ROLE camera_api LOGIN PASSWORD '${CAMERA_API_PASSWORD}';
SQL
