package payment

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"toko-online/internal/domain"
)

const toleransiWaktu = 5 * time.Minute

type FakeGateway struct {
	secret []byte
	now    func() time.Time
}

func NewFakeGateway(secret string) *FakeGateway {
	return &FakeGateway{secret: []byte(secret), now: time.Now}
}

func (g *FakeGateway) CreateCharge(ctx context.Context, orderID, amount int64) (domain.Charge, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return domain.Charge{}, fmt.Errorf("buat referensi tagihan: %w", err)
	}
	return domain.Charge{Reference: "SIM-" + hex.EncodeToString(buf)}, nil
}

func (g *FakeGateway) Sign(timestamp string, body []byte) string {
	return hex.EncodeToString(g.mac(timestamp, body))
}

func (g *FakeGateway) mac(timestamp string, body []byte) []byte {
	h := hmac.New(sha256.New, g.secret)
	h.Write([]byte(timestamp))
	h.Write([]byte("."))
	h.Write(body)
	return h.Sum(nil)
}

type webhookBody struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Amount    int64  `json:"amount"`
}

func (g *FakeGateway) ParseWebhook(body []byte, timestamp, signature string) (domain.GatewayEvent, error) {
	diberikan, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(diberikan, g.mac(timestamp, body)) {
		return domain.GatewayEvent{}, domain.ErrInvalidSignature
	}

	detik, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return domain.GatewayEvent{}, domain.ErrMalformedWebhook
	}
	selisih := g.now().Sub(time.Unix(detik, 0))
	if selisih < 0 {
		selisih = -selisih
	}
	if selisih > toleransiWaktu {
		return domain.GatewayEvent{}, domain.ErrStaleWebhook
	}

	var b webhookBody
	if err := json.Unmarshal(body, &b); err != nil || b.Type == "" || b.Reference == "" || b.Amount < 0 {
		return domain.GatewayEvent{}, domain.ErrMalformedWebhook
	}
	return domain.GatewayEvent{Type: b.Type, Reference: b.Reference, Amount: b.Amount}, nil
}
