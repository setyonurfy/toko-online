package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	var req orderRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	lines := make([]domain.OrderLine, 0, len(req.Items))
	for _, it := range req.Items {
		lines = append(lines, domain.OrderLine{ProductID: it.ProductID, Jumlah: it.Jumlah})
	}

	order, err := h.orders.Place(r.Context(), userID, lines)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			writeError(w, http.StatusUnprocessableEntity, "salah satu produk dalam pesanan tidak ditemukan")
			return
		}
		writeUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/orders/"+strconv.FormatInt(order.ID, 10))
	writeJSON(w, http.StatusCreated, order)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := idFromPath(r.URL.Path, "/orders/")
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	order, err := h.orders.Get(r.Context(), userID, orderID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	limit, err := queryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit harus berupa angka")
		return
	}
	offset, err := queryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset harus berupa angka")
		return
	}

	pesanan, err := h.orders.ListMine(r.Context(), userID, limit, offset)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	if pesanan == nil {
		pesanan = []domain.Order{}
	}
	writeJSON(w, http.StatusOK, orderListResponse{Data: pesanan})
}

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	orderID, ok := idFromPathDenganAkhiran(r.URL.Path, "/orders/", "/cancel")
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}

	order, err := h.orders.Cancel(r.Context(), userID, orderID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// idFromPath mengambil ID dari path berbentuk "<prefix><id>" secara ketat:
// hanya angka desimal baku ("5" sah; "05", "+5", "5/x", "abc" ditolak).
func idFromPath(path, prefix string) (int64, bool) {
	raw := strings.TrimPrefix(path, prefix)
	if raw == "" || strings.Contains(raw, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw {
		return 0, false
	}
	return id, true
}

func idFromPathDenganAkhiran(path, prefix, akhiran string) (int64, bool) {
	if !strings.HasSuffix(path, akhiran) {
		return 0, false
	}
	return idFromPath(strings.TrimSuffix(path, akhiran), prefix)
}
