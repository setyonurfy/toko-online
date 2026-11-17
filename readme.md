# Toko Online (Go, tanpa framework)

API toko online sederhana yang dibangun dengan **Go standard library (`net/http`)** dan **PostgreSQL**, memakai **clean architecture**.

Proyek ini dibuat sebagai latihan membangun backend yang aman dan teruji: autentikasi JWT, otorisasi berbasis peran, pembatasan laju, transaksi database, dan pengurangan stok yang atomik (tanpa *overselling*).

> Status: proyek belajar. Belum siap produksi (lihat bagian [Batasan dan rencana](#batasan-dan-rencana)).

## Fitur

- Registrasi dan login: password di-hash dengan bcrypt, token JWT (HS256)
- Peran `customer` dan `admin`
- Katalog produk: baca terbuka untuk semua, tulis hanya untuk admin
- Pesanan dengan beberapa barang
  - stok dikurangi secara atomik dalam satu transaksi
  - nama dan harga disalin ke pesanan (*snapshot*), sehingga perubahan harga tidak mengubah riwayat
  - pembeli hanya bisa melihat pesanannya sendiri
- Pembatasan laju (token bucket per IP) untuk `/auth/register` dan `/auth/login`
- Unit test, integration test ke database sungguhan, dan uji konkurensi

## Teknologi

| Bagian | Pilihan |
|---|---|
| Bahasa | Go 1.21 |
| HTTP | `net/http` (tanpa framework) |
| Database | PostgreSQL lewat `database/sql` dan `github.com/lib/pq` |
| Migrasi | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Password | bcrypt (`golang.org/x/crypto`) |
| Token | JWT HS256 (`github.com/golang-jwt/jwt/v5`) |

## Memulai

### Prasyarat

- Go 1.21 atau lebih baru
- PostgreSQL yang sedang berjalan (misalnya Postgres.app di macOS)
- `openssl` (untuk membuat secret JWT)
- golang-migrate:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2
```

### Langkah

```bash
# 1. Buat database
psql postgres -c "CREATE DATABASE toko_online;"

# 2. Terapkan migrasi
./migrate.sh up

# 3. Jalankan server (port 8080)
./run.sh
```

`run.sh` membuat secret JWT satu kali dan menyimpannya di berkas `.jwt_secret` (sudah masuk `.gitignore`). Kalau variabel `JWT_SECRET` sudah diisi, nilai itulah yang dipakai.

### Membuat admin pertama

Tidak ada endpoint untuk menjadikan seseorang admin. Itu disengaja: pemberian hak tertinggi tidak boleh terjangkau dari internet. Daftarkan akun seperti biasa, lalu ubah perannya lewat SQL:

```bash
psql toko_online -c "UPDATE users SET role='admin' WHERE email='admin@mail.com';"
```

## Konfigurasi

| Variabel | Wajib | Bawaan | Keterangan |
|---|---|---|---|
| `JWT_SECRET` | Ya | - | Minimal 32 karakter. Server menolak berjalan tanpa ini |
| `DATABASE_URL` | Tidak | `postgres://localhost:5432/toko_online?sslmode=disable` | Alamat koneksi PostgreSQL |
| `PORT` | Tidak | `8080` | Port server |
| `TEST_DATABASE_URL` | Hanya untuk integration test | - | Harus menunjuk database yang namanya berakhiran `_test` |

Masa berlaku token adalah 24 jam (tetap di kode).

## Endpoint

| Method | Path | Akses | Keterangan |
|---|---|---|---|
| GET | `/health` | Publik | Pemeriksaan server |
| POST | `/auth/register` | Publik, dibatasi laju | Mendaftar sebagai `customer` |
| POST | `/auth/login` | Publik, dibatasi laju | Mengembalikan data user dan token |
| GET | `/me` | Login | Profil pemilik token |
| GET | `/products` | Publik | Daftar produk, `?limit=&offset=` |
| GET | `/products/{id}` | Publik | Detail produk |
| POST | `/products` | Admin | Tambah produk |
| PUT | `/products/{id}` | Admin | Ganti seluruh data produk |
| DELETE | `/products/{id}` | Admin | Hapus produk (409 jika sudah pernah dipesan) |
| POST | `/orders` | Login | Buat pesanan |
| GET | `/orders` | Login | Daftar pesanan milik sendiri |
| GET | `/orders/{id}` | Login | Detail pesanan milik sendiri |

Autentikasi memakai header `Authorization: Bearer <token>`.

### Contoh pemakaian

```bash
# Daftar
curl -X POST localhost:8080/auth/register -H "Content-Type: application/json" \
  -d '{"nama":"Siti","email":"siti@mail.com","password":"rahasia123"}'

# Login: respons memuat "token"
curl -X POST localhost:8080/auth/login -H "Content-Type: application/json" \
  -d '{"email":"siti@mail.com","password":"rahasia123"}'

# Tambah produk (admin). Harga dalam rupiah, bilangan bulat
curl -X POST localhost:8080/products -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -d '{"nama":"Kopi Gayo","deskripsi":"Biji pilihan","harga":50000,"stok":10}'

# Buat pesanan. Klien hanya mengirim ID produk dan jumlah
curl -X POST localhost:8080/orders -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"items":[{"product_id":1,"jumlah":2}]}'
```

Contoh respons pesanan:

```json
{
  "id": 1,
  "user_id": 2,
  "status": "pending",
  "total": 100000,
  "items": [
    {"product_id": 1, "nama_produk": "Kopi Gayo", "harga": 50000, "jumlah": 2, "subtotal": 100000}
  ],
  "created_at": "2026-11-17T10:00:00+07:00"
}
```

### Kode status

| Status | Arti |
|---|---|
| 200, 201, 204 | Berhasil, dibuat (dengan header `Location`), berhasil tanpa isi |
| 400 | Permintaan salah: JSON rusak, field asing, atau nilai tidak valid |
| 401 | Token tidak ada atau tidak valid |
| 403 | Login valid tetapi tidak berhak (bukan admin) |
| 404 | Tidak ditemukan. Pesanan milik orang lain juga dijawab 404 |
| 405 | Method tidak didukung (header `Allow` menyebut yang didukung) |
| 409 | Bentrok: email sudah terdaftar, stok tidak mencukupi, produk sudah pernah dipesan |
| 413 | Body lebih dari 16 KB |
| 422 | Produk dalam pesanan tidak ditemukan |
| 429 | Terlalu banyak percobaan (header `Retry-After`) |
| 500 | Kesalahan server (detail hanya di log, tidak dikirim ke klien) |

## Arsitektur

Aturan utamanya: **ketergantungan hanya menunjuk ke dalam**. Lapisan dalam tidak mengenal lapisan luar.

```
Klien
  |
  v
delivery/http   handler, middleware, router      (HTTP, JSON, status code)
  |
  v
usecase         aturan bisnis                    (validasi, bcrypt, kepemilikan)
  |
  v
domain          entity, error, interface         (kontrak penyimpanan)
  ^
  |
repository      implementasi PostgreSQL          (SQL, transaksi)
```

`usecase` hanya memegang interface (misalnya `domain.UserRepository`), sehingga di test bisa diganti tiruan tanpa database. Semua komponen dirangkai secara manual di `cmd/api/main.go`.

### Struktur folder

```
cmd/api/              titik masuk dan perakitan komponen
internal/
  auth/               pembuat dan pemeriksa token JWT
  config/             pembacaan environment variable
  database/           koneksi dan pool PostgreSQL
  domain/             entity, error bernama, interface repository
  usecase/            aturan bisnis (auth, produk, pesanan)
  repository/         implementasi PostgreSQL dan integration test
  delivery/http/      handler, middleware, router, rate limiter
migrations/           berkas migrasi SQL (up dan down)
```

## Keputusan desain

**Keamanan**
- Password disimpan sebagai hash bcrypt. Hash tidak pernah ikut di respons (`json:"-"`)
- Login dengan email salah dan password salah dijawab sama, dan waktu responsnya disamakan (hash pengecoh) agar email terdaftar tidak bisa ditebak lewat waktu
- Token JWT hanya menerima algoritma HS256. Peran dibaca dari database, bukan dari token, sehingga pencabutan hak admin langsung berlaku
- Peran tidak pernah diterima dari klien. Body yang memuat field tak dikenal ditolak (400)
- Identitas pembeli pada pesanan diambil dari token, dan harga selalu dibaca dari database
- Semua query memakai placeholder (`$1`), tanpa penyambungan string dari input
- Pembatasan laju per IP, batas ukuran body, dan timeout server

**Data**
- Harga disimpan sebagai bilangan bulat (rupiah), bukan `float64`
- Aturan penting ditulis dua kali: di usecase dan sebagai `CHECK` di database
- Pengurangan stok memakai satu perintah atomik, bukan "baca lalu tulis":

```sql
UPDATE products SET stok = stok - $1
WHERE id = $2 AND stok >= $1
RETURNING nama, harga
```

- Pesanan dibuat dalam satu transaksi: kalau satu barang gagal, semua perubahan stok dibatalkan
- Produk diproses terurut menurut ID untuk mencegah *deadlock*

## Pengujian

```bash
# Unit test dan test handler (tidak butuh database)
go test ./... -cover

# Dengan pendeteksi data race
go test -race ./...

# Integration test ke PostgreSQL sungguhan, termasuk uji konkurensi
./test-integrasi.sh
```

`test-integrasi.sh` membuat database `toko_online_test`, memasang migrasi di sana, lalu menjalankan test. Integration test **menolak berjalan** kalau nama database tidak berakhiran `_test`, karena test itu mengosongkan tabel.

Uji ujung ke ujung untuk rute produk terhadap server yang sedang berjalan:

```bash
./smoke-produk.sh
```

Skrip ini memerlukan akun `siti@mail.com` (admin) dan `andi@mail.com` (pembeli) dengan password uji yang sama, yang bisa diubah lewat variabel `PASSWORD_UJI`.

## Skrip

| Skrip | Fungsi |
|---|---|
| `run.sh` | Menjalankan server dengan secret JWT yang tersimpan |
| `migrate.sh` | Pembungkus golang-migrate: `up`, `down [N]`, `version`, `create <nama>`, `force <versi>` |
| `test-integrasi.sh` | Menyiapkan database uji dan menjalankan integration test |
| `smoke-produk.sh` | Uji cepat rute produk ke server yang berjalan |

## Batasan dan rencana

Belum ada, dan dikerjakan bertahap:

- Pembatalan pesanan (pesanan `pending` saat ini menahan stok)
- Pembayaran lewat payment gateway (status `paid` sudah disiapkan)
- TLS dan koneksi database terenkripsi
- Token berumur pendek dengan *refresh token* yang bisa dicabut
- Daftar *proxy* tepercaya untuk limiter berbasis IP
- Verifikasi email dan penolakan password umum
- Penghapusan lunak produk
- Logging terstruktur, *graceful shutdown*, CI, dan Docker