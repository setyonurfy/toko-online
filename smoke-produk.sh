#!/bin/sh
# Uji cepat ke server yang SEDANG berjalan (./run.sh di terminal lain).
# Prasyarat: siti@mail.com berperan admin, andi@mail.com pembeli.
BASE="${BASE_URL:-http://localhost:8080}"
PASSWORD_UJI="${PASSWORD_UJI:-rahasia123}"   # hanya kata sandi akun uji lokal
JSON='Content-Type: application/json'
gagal=0

login() {
  curl -s -X POST "$BASE/auth/login" -H "$JSON" \
    -d "{\"email\":\"$1\",\"password\":\"$PASSWORD_UJI\"}" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
}

status() { curl -s -o /dev/null -w "%{http_code}" "$@"; }

cek() {
  if [ "$2" = "$3" ]; then
    echo "PASS  $1 ($3)"
  else
    echo "FAIL  $1: ingin $2, dapat $3"
    gagal=1
  fi
}

TOKEN_ADMIN=$(login siti@mail.com)
TOKEN_ANDI=$(login andi@mail.com)
[ -n "$TOKEN_ADMIN" ] || { echo "login admin gagal (cek password, role admin, atau batas laju login)"; exit 1; }
[ -n "$TOKEN_ANDI" ]  || { echo "login pembeli gagal"; exit 1; }

RESP=$(curl -s -X POST "$BASE/products" -H "$JSON" -H "Authorization: Bearer $TOKEN_ADMIN" \
  -d '{"nama":"Kopi Uji","deskripsi":"Data uji","harga":55000,"stok":10}')
ID=$(echo "$RESP" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
[ -n "$ID" ] || { echo "gagal membuat produk uji. Respons: $RESP"; exit 1; }
echo "produk uji dibuat dengan ID $ID"

cek "daftar produk terbuka"           200 "$(status "$BASE/products")"
cek "detail produk terbuka"           200 "$(status "$BASE/products/$ID")"
cek "tambah tanpa token"              401 "$(status -X POST "$BASE/products" -H "$JSON" -d '{"nama":"X","harga":1000,"stok":1}')"
cek "tambah sebagai pembeli"          403 "$(status -X POST "$BASE/products" -H "$JSON" -H "Authorization: Bearer $TOKEN_ANDI" -d '{"nama":"X","harga":1000,"stok":1}')"

cek "PUT tanpa stok ditolak"          400 "$(status -X PUT "$BASE/products/$ID" -H "$JSON" -H "Authorization: Bearer $TOKEN_ADMIN" -d '{"nama":"Kopi Uji","harga":60000}')"
if curl -s "$BASE/products/$ID" | grep -q '"stok":10'; then
  echo "PASS  stok tetap 10 setelah PUT yang ditolak"
else
  echo "FAIL  stok berubah setelah PUT yang seharusnya ditolak"; gagal=1
fi

cek "PUT lengkap berhasil"            200 "$(status -X PUT "$BASE/products/$ID" -H "$JSON" -H "Authorization: Bearer $TOKEN_ADMIN" -d '{"nama":"Kopi Uji Premium","deskripsi":"Diubah","harga":65000,"stok":8}')"
if curl -s "$BASE/products/$ID" | grep -q '"stok":8'; then
  echo "PASS  stok menjadi 8 setelah PUT"
else
  echo "FAIL  stok tidak menjadi 8"; gagal=1
fi

cek "hapus oleh pembeli ditolak"      403 "$(status -X DELETE "$BASE/products/$ID" -H "Authorization: Bearer $TOKEN_ANDI")"
cek "hapus oleh admin"                204 "$(status -X DELETE "$BASE/products/$ID" -H "Authorization: Bearer $TOKEN_ADMIN")"
cek "detail setelah dihapus"          404 "$(status "$BASE/products/$ID")"
cek "hapus ulang"                     404 "$(status -X DELETE "$BASE/products/$ID" -H "Authorization: Bearer $TOKEN_ADMIN")"
cek "method tidak didukung"           405 "$(status -X PATCH "$BASE/products/1")"

[ "$gagal" -eq 0 ] && echo "SEMUA LULUS" || echo "ADA YANG GAGAL"
exit "$gagal"
