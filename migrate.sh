#!/bin/sh
# Pembungkus golang-migrate untuk proyek toko-online.
set -eu

. ./load-env.sh

DB_URL="${DATABASE_URL:?DATABASE_URL belum diisi, isi di .env}"
DIR="migrations"

export PATH="$PATH:$(go env GOPATH)/bin"

if ! command -v migrate >/dev/null 2>&1; then
  echo "perintah 'migrate' tidak ditemukan. Pasang dengan:"
  echo "  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2"
  exit 1
fi

usage() {
  cat <<USAGE
Pemakaian: ./migrate.sh <perintah> [argumen]

  up               terapkan semua migrasi yang belum dijalankan
  down [N]         mundurkan N migrasi (bawaan 1), meminta konfirmasi
  version          tampilkan versi migrasi saat ini
  create <nama>    buat sepasang file migrasi baru (.up.sql dan .down.sql)
  force <versi>    paksa penanda versi (hanya untuk memperbaiki status "dirty")
USAGE
}

confirm() {
  printf "%s [y/N] " "$1"
  read -r jawaban
  [ "$jawaban" = "y" ] || [ "$jawaban" = "Y" ]
}

cmd="${1:-}"

case "$cmd" in
  up)
    migrate -path "$DIR" -database "$DB_URL" up
    ;;
  down)
    n="${2:-1}"
    confirm "Mundurkan $n migrasi? Data pada tabel terkait bisa hilang." || { echo "dibatalkan"; exit 1; }
    migrate -path "$DIR" -database "$DB_URL" down "$n"
    ;;
  version)
    migrate -path "$DIR" -database "$DB_URL" version
    ;;
  create)
    [ -n "${2:-}" ] || { echo "nama migrasi wajib diisi, contoh: ./migrate.sh create add_orders_table"; exit 1; }
    migrate create -ext sql -dir "$DIR" -seq "$2"
    ;;
  force)
    [ -n "${2:-}" ] || { echo "versi wajib diisi, contoh: ./migrate.sh force 3"; exit 1; }
    confirm "Paksa penanda versi ke $2 tanpa menjalankan migrasi apa pun?" || { echo "dibatalkan"; exit 1; }
    migrate -path "$DIR" -database "$DB_URL" force "$2"
    ;;
  *)
    usage
    exit 1
    ;;
esac
