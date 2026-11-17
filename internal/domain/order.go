package domain

import (
	"context"
	"errors"
	"time"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderPaid      OrderStatus = "paid"
	OrderCancelled OrderStatus = "cancel"
)

type OrderItem struct {
	ProductID  int64  `json:"product_id"`
	NamaProduk string `json:"nama_product"`
	Harga      int64  `json:"harga"`
	Jumlah     int    `json:"jumlah"`
	Subtotal   int64  `json:"subtotal"`
}

type Order struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"`
	Status    OrderStatus `json:"status"`
	Total     int64       `json:"total"`
	Items     []OrderItem `json:"items,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type OrderLine struct {
	ProductID int64
	Jumlah    int
}

var (
	ErrOrderNotFound     = errors.New("pesanan tidak di temukan")
	ErrInsufficientStock = errors.New("stok tidak mencukupi")
)

type OrderRepository interface {
	Create(ctx context.Context, userID int64, lines []OrderLine) (*Order, error)
	GetByID(ctx context.Context, id int64) (*Order, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]Order, error)
}
