#!/bin/sh
set -e

backend_pid=""
frontend_pid=""

shutdown() {
  if [ -n "$backend_pid" ]; then
    kill -TERM "$backend_pid" 2>/dev/null || true
  fi
  if [ -n "$frontend_pid" ]; then
    kill -TERM "$frontend_pid" 2>/dev/null || true
  fi
  if [ -n "$backend_pid" ]; then
    wait "$backend_pid" 2>/dev/null || true
  fi
  if [ -n "$frontend_pid" ]; then
    wait "$frontend_pid" 2>/dev/null || true
  fi
  exit 0
}

trap shutdown TERM INT

PORT=8080 /app/server &
backend_pid=$!

cd /app/frontend
HOSTNAME=0.0.0.0 PORT=3000 node server.js &
frontend_pid=$!

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$frontend_pid" 2>/dev/null; do
  wait -n "$backend_pid" "$frontend_pid" 2>/dev/null || break
done

shutdown
