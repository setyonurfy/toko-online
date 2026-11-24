package order

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"toko-online/internal/adapter/delivery/http/httpx"
	"toko-online/internal/adapter/delivery/http/middleware"
	"toko-online/internal/domain"
)

type OrderService interface {
	Place(ctx context.Context, userID int64, lines []domain.OrderLine) (*domain.Order, error)
	Get(ctx context.Context, userID, orderID int64) (*domain.Order, error)
	ListMine(ctx context.Context, userID int64, limit, offset int) ([]domain.Order, error)
	Cancel(ctx context.Context, userID, orderID int64) (*domain.Order, error)
}

type OrderHandler struct {
	orders OrderService
}

func NewOrderHandler(orders OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

type orderItemRequest struct {
	ProductID int64 `json:"product_id"`
	Jumlah    int   `json:"jumlah"`
}

type orderRequest struct {
	Items []orderItemRequest `json:"items"`
}

type orderListResponse struct {
	Data []domain.Order `json:"data"`
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	var req orderRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	lines := make([]domain.OrderLine, 0, len(req.Items))
	for _, it := range req.Items {
		lines = append(lines, domain.OrderLine{ProductID: it.ProductID, Jumlah: it.Jumlah})
	}

	order, err := h.orders.Place(r.Context(), userID, lines)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "salah satu produk dalam pesanan tidak ditemukan")
			return
		}
		httpx.WriteUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/orders/"+strconv.FormatInt(order.ID, 10))
	httpx.WriteJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := httpx.IDFromPath(r.URL.Path, "/orders/")
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	order, err := h.orders.Get(r.Context(), userID, orderID)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	limit, err := httpx.QueryInt(r, "limit")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "limit harus berupa angka")
		return
	}
	offset, err := httpx.QueryInt(r, "offset")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "offset harus berupa angka")
		return
	}

	pesanan, err := h.orders.ListMine(r.Context(), userID, limit, offset)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	if pesanan == nil {
		pesanan = []domain.Order{}
	}
	httpx.WriteJSON(w, http.StatusOK, orderListResponse{Data: pesanan})
}

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := httpx.IDFromPathDenganAkhiran(r.URL.Path, "/orders/", "/cancel")
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	order, err := h.orders.Cancel(r.Context(), userID, orderID)
	if err != nil {
		httpx.WriteUsecaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, order)
}
