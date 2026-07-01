#!/usr/bin/env bash
# Apply HTTP mode (useHttps in starcrystal.json) and restart API on Linux test host.
set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
cd "$ROOT"
./stopsvr.sh || true
sleep 1
./startsvr.sh
sleep 2
curl -fsS "http://127.0.0.1:8080/healthz"
echo ""
