#!/bin/sh
set -eu

base_url="${SERVICE_URL:-http://localhost:8080}"

curl --fail-with-body --request POST "${base_url}/events" \
  --header 'Content-Type: application/json' \
  --data '{"id":"build-42","kind":"build","project":"compiler","outcome":"passed"}'
curl --fail-with-body --request POST "${base_url}/events" \
  --header 'Content-Type: application/json' \
  --data '{"id":"diag-42","kind":"diagnostic","project":"compiler","severity":"warning","value":3}'
curl --fail-with-body --request GET "${base_url}/chart"
