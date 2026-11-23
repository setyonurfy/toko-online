package usecase

import (
	"toko-online/internal/usecase/auth"
	"toko-online/internal/usecase/order"
	"toko-online/internal/usecase/payment"
	"toko-online/internal/usecase/product"
)

type (
	AuthUsecase        = auth.AuthUsecase
	ProductUsecase     = product.ProductUsecase
	OrderUsecase       = order.OrderUsecase
	OrderExpiryUsecase = order.OrderExpiryUsecase
	PaymentUsecase     = payment.PaymentUsecase
)

var (
	NewAuthUsecase        = auth.NewAuthUsecase
	NewProductUsecase     = product.NewProductUsecase
	NewOrderUsecase       = order.NewOrderUsecase
	NewOrderExpiryUsecase = order.NewOrderExpiryUsecase
	NewPaymentUsecase     = payment.NewPaymentUsecase
)
