#!/bin/sh
set -eu

. ./load-env.sh

TEST_DB="toko_online_test"
export TEST_DATABASE_URL="postgres://localhost:5432/$TEST_DB?sslmode=disable"

if ! psql postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$TEST_DB'" | grep -q 1; then
  psql postgres -c "CREATE DATABASE $TEST_DB;"
fi

DATABASE_URL="$TEST_DATABASE_URL" ./migrate.sh up
go test -p 1 -count=1 -race -v ./internal/adapter/repository/postgres/...
