#!/bin/sh
# Flux start script — sources .env and starts the server.
# Usage: ./start.sh

set -a
[ -f .env ] && . ./.env
set +a
exec ./flux
