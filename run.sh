#!/bin/sh
SECRET_FILE=".jwt_secret"

if [ -z "${JWT_SECRET:-}" ]; then
  if [ ! -f "$SECRET_FILE" ]; then
    openssl rand -hex 32 > "$SECRET_FILE"
    chmod 600 "$SECRET_FILE"
  fi
  JWT_SECRET="$(cat "$SECRET_FILE")"
fi

export JWT_SECRET
exec go run ./cmd/api
