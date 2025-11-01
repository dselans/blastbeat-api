#!/bin/bash

echo "Starting Redis server..."
redis-server --daemonize no --port 6379 &
REDIS_PID=$!

sleep 2

echo "Waiting for Redis to be ready..."
for i in {1..10}; do
  if redis-cli ping &>/dev/null; then
    echo "Redis is ready!"
    break
  fi
  echo "Waiting for Redis... ($i/10)"
  sleep 1
done

export BLASTBEAT_API_ENV_NAME=dev
export BLASTBEAT_API_SERVICE_NAME=blastbeat-api
export BLASTBEAT_API_API_LISTEN_ADDRESS=0.0.0.0:8080
export BLASTBEAT_API_LOG_CONFIG=dev
export BLASTBEAT_API_DB_HOST="${PGHOST}"
export BLASTBEAT_API_DB_NAME="${PGDATABASE}"
export BLASTBEAT_API_DB_USER="${PGUSER}"
export BLASTBEAT_API_DB_PASSWORD="${PGPASSWORD}"
export BLASTBEAT_API_DB_PORT="${PGPORT}"
export BLASTBEAT_API_DB_SSL_MODE=require
export BLASTBEAT_API_REDIS_URL=localhost:6379
export BLASTBEAT_API_REDIS_PASSWORD=
export BLASTBEAT_API_REDIS_DATABASE=0
export BLASTBEAT_API_REDIS_POOL_SIZE=10
export BLASTBEAT_API_REDIS_DIAL_TIMEOUT=5s

echo "Starting blastbeat-api via make run..."
make run

kill $REDIS_PID 2>/dev/null
