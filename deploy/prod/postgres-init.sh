#!/bin/bash
# Runs once when the data directory is created: the login role the API uses,
# already a member of camera_app, the role the migrations give table rights.
set -e
psql -v ON_ERROR_STOP=1 -U postgres -d camera <<SQL
CREATE ROLE camera_app NOLOGIN;
CREATE ROLE camera_api LOGIN PASSWORD '${CAMERA_API_PASSWORD}' IN ROLE camera_app;
SQL
