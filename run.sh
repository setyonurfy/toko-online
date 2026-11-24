#!/bin/sh
. ./load-env.sh

SECRET_FILE=".jwt_secret"

if [ -z "${JWT_SECRET:-}" ]; then
  if [ ! -f "$SECRET_FILE" ]; then
    openssl rand -hex 32 > "$SECRET_FILE"
    chmod 600 "$SECRET_FILE"
  fi
  JWT_SECRET="$(cat "$SECRET_FILE")"
fi

WEBHOOK_FILE=".webhook_secret"

if [ -z "${PAYMENT_WEBHOOK_SECRET:-}" ]; then
  if [ ! -f "$WEBHOOK_FILE" ]; then
    openssl rand -hex 32 > "$WEBHOOK_FILE"
    chmod 600 "$WEBHOOK_FILE"
  fi
  PAYMENT_WEBHOOK_SECRET="$(cat "$WEBHOOK_FILE")"
fi

export JWT_SECRET PAYMENT_WEBHOOK_SECRET
exec go run ./cmd/api
