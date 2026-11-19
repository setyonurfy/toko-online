package payment

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"toko-online/internal/domain"
)

const secretUji = "secret-webhook-untuk-test-minimal-32-karakter"

const bodySah = `{"type":"payment.paid","reference":"SIM-abc123","amount":150000}`

var waktuUji = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func gatewayUji() *FakeGateway {
	g := NewFakeGateway(secretUji)
	g.now = func() time.Time { return waktuUji }
	return g
}

func stempel(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func TestFakeGateway_CreateCharge(t *testing.T) {
	g := gatewayUji()

	a, err := g.CreateCharge(context.Background(), 1, 1000)
	if err != nil {
		t.Fatalf("create charge gagal: %v", err)
	}
	b, err := g.CreateCharge(context.Background(), 1, 1000)
	if err != nil {
		t.Fatalf("create charge gagal: %v", err)
	}

	if !strings.HasPrefix(a.Reference, "SIM-") || len(a.Reference) != 20 {
		t.Errorf("referensi = %q, ingin awalan SIM- dan panjang 20", a.Reference)
	}
	if a.Reference == b.Reference {
		t.Errorf("referensi harus unik, dapat %q dua kali", a.Reference)
	}
}

func TestFakeGateway_Sign(t *testing.T) {
	g := gatewayUji()
	body := []byte(bodySah)
	ts := stempel(waktuUji)

	asli := g.Sign(ts, body)

	if asli != g.Sign(ts, body) {
		t.Error("tanda tangan harus sama untuk masukan yang sama")
	}
	if len(asli) != 64 {
		t.Errorf("panjang = %d, ingin 64 (SHA-256 dalam heksadesimal)", len(asli))
	}
	if asli == g.Sign(stempel(waktuUji.Add(time.Second)), body) {
		t.Error("cap waktu berbeda harus menghasilkan tanda tangan berbeda")
	}
	if asli == g.Sign(ts, []byte(`{"type":"payment.paid","reference":"SIM-abc123","amount":1}`)) {
		t.Error("isi berbeda harus menghasilkan tanda tangan berbeda")
	}
	lain := NewFakeGateway("secret-lain-yang-juga-cukup-panjang-32-karakter")
	if asli == lain.Sign(ts, body) {
		t.Error("secret berbeda harus menghasilkan tanda tangan berbeda")
	}
}

func TestFakeGateway_ParseWebhook(t *testing.T) {
	tests := []struct {
		name    string
		buat    func(g *FakeGateway) (body []byte, ts, sig string)
		wantErr error
	}{
		{
			name: "sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				ts := stempel(waktuUji)
				return b, ts, g.Sign(ts, b)
			},
		},
		{
			name: "sah, 4 menit lalu masih dalam toleransi",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				ts := stempel(waktuUji.Add(-4 * time.Minute))
				return b, ts, g.Sign(ts, b)
			},
		},
		{
			name: "tanda tangan salah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				return []byte(bodySah), stempel(waktuUji), strings.Repeat("0", 64)
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "tanda tangan kosong",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				return []byte(bodySah), stempel(waktuUji), ""
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "tanda tangan bukan heksadesimal",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				return []byte(bodySah), stempel(waktuUji), "bukan-heksadesimal"
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "isi diubah setelah ditandatangani",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				ts := stempel(waktuUji)
				sig := g.Sign(ts, []byte(bodySah))
				diubah := []byte(`{"type":"payment.paid","reference":"SIM-abc123","amount":1}`)
				return diubah, ts, sig
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "cap waktu diubah setelah ditandatangani",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				sig := g.Sign(stempel(waktuUji), b)
				return b, stempel(waktuUji.Add(-time.Minute)), sig
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "ditandatangani dengan secret berbeda",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				lain := NewFakeGateway("secret-lain-yang-juga-cukup-panjang-32-karakter")
				b := []byte(bodySah)
				ts := stempel(waktuUji)
				return b, ts, lain.Sign(ts, b)
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "tanda tangan salah dan waktu kedaluwarsa: tetap dijawab tanda tangan tidak valid",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				return []byte(bodySah), stempel(waktuUji.Add(-time.Hour)), strings.Repeat("0", 64)
			},
			wantErr: domain.ErrInvalidSignature,
		},
		{
			name: "kedaluwarsa (10 menit lalu), tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				ts := stempel(waktuUji.Add(-10 * time.Minute))
				return b, ts, g.Sign(ts, b)
			},
			wantErr: domain.ErrStaleWebhook,
		},
		{
			name: "dari masa depan (10 menit lagi), tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				ts := stempel(waktuUji.Add(10 * time.Minute))
				return b, ts, g.Sign(ts, b)
			},
			wantErr: domain.ErrStaleWebhook,
		},
		{
			name: "cap waktu bukan angka, tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(bodySah)
				return b, "kemarin", g.Sign("kemarin", b)
			},
			wantErr: domain.ErrMalformedWebhook,
		},
		{
			name: "json rusak, tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(`{bukan json`)
				ts := stempel(waktuUji)
				return b, ts, g.Sign(ts, b)
			},
			wantErr: domain.ErrMalformedWebhook,
		},
		{
			name: "referensi kosong, tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(`{"type":"payment.paid","reference":"","amount":1000}`)
				ts := stempel(waktuUji)
				return b, ts, g.Sign(ts, b)
			},
			wantErr: domain.ErrMalformedWebhook,
		},
		{
			name: "jumlah negatif, tanda tangan sah",
			buat: func(g *FakeGateway) ([]byte, string, string) {
				b := []byte(`{"type":"payment.paid","reference":"SIM-abc123","amount":-5}`)
				ts := stempel(waktuUji)
				return b, ts, g.Sign(ts, b)
			},
			wantErr: domain.ErrMalformedWebhook,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gatewayUji()
			body, ts, sig := tt.buat(g)

			ev, err := g.ParseWebhook(body, ts, sig)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, ingin %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if ev != (domain.GatewayEvent{}) {
					t.Errorf("event harus kosong saat ditolak, dapat %+v", ev)
				}
				return
			}
			want := domain.GatewayEvent{Type: domain.EventPaymentPaid, Reference: "SIM-abc123", Amount: 150000}
			if ev != want {
				t.Errorf("event = %+v, ingin %+v", ev, want)
			}
		})
	}
}
