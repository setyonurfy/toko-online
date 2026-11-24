package handler

import (
	"toko-online/internal/adapter/delivery/http/auth"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/adapter/delivery/http/order"
	"toko-online/internal/adapter/delivery/http/payment"
	"toko-online/internal/adapter/delivery/http/product"
)

var (
	NewAuthHandler    = auth.NewAuthHandler
	NewProductHandler = product.NewProductHandler
	NewOrderHandler   = order.NewOrderHandler
	NewPaymentHandler = payment.NewPaymentHandler
	NewRateLimiter    = middleware.NewRateLimiter
)
