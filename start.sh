#!/bin/sh
set -e

if [ -f /app/server ]; then
  exec /app/server
fi

cd backend
exec go run ./cmd/server
