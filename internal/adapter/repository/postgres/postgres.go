package postgres

import (
	"toko-online/internal/adapter/repository/postgres/order"
	"toko-online/internal/adapter/repository/postgres/payment"
	"toko-online/internal/adapter/repository/postgres/product"
	"toko-online/internal/adapter/repository/postgres/user"
)

var (
	NewUserPostgres    = user.NewUserPostgres
	NewProductPostgres = product.NewProductPostgres
	NewOrderPostgres   = order.NewOrderPostgres
	NewPaymentPostgres = payment.NewPaymentPostgres
)
