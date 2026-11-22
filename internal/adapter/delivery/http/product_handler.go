package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"toko-online/internal/domain"
)

type ProductService interface {
	Create(ctx context.Context, nama, deskripsi string, harga int64, stok int) (*domain.Product, error)
	Get(ctx context.Context, id int64) (*domain.Product, error)
	List(ctx context.Context, limit, offset int) ([]domain.Product, error)
	Update(ctx context.Context, id int64, nama, deskripsi string, harga int64, stok int) (*domain.Product, error)
	Delete(ctx context.Context, id int64) error
}

type ProductHandler struct {
	products ProductService
}

func NewProductHandler(products ProductService) *ProductHandler {
	return &ProductHandler{products: products}
}

type productRequest struct {
	Nama      string `json:"nama"`
	Deskripsi string `json:"deskripsi"`
	Harga     int64  `json:"harga"`
	Stok      *int   `json:"stok"`
}

type productListResponse struct {
	Data []domain.Product `json:"data"`
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
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

	produk, err := h.products.List(r.Context(), limit, offset)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	if produk == nil {
		produk = []domain.Product{}
	}
	writeJSON(w, http.StatusOK, productListResponse{Data: produk})
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := productIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrProductNotFound.Error())
		return
	}

	p, err := h.products.Get(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Stok == nil {
		writeError(w, http.StatusBadRequest, "field stok wajib diisi")
		return
	}

	p, err := h.products.Create(r.Context(), req.Nama, req.Deskripsi, req.Harga, *req.Stok)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.Header().Set("Location", "/products/"+strconv.FormatInt(p.ID, 10))
	writeJSON(w, http.StatusCreated, p)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := productIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrProductNotFound.Error())
		return
	}

	var req productRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Stok == nil {
		writeError(w, http.StatusBadRequest, "field stok wajib diisi")
		return
	}

	p, err := h.products.Update(r.Context(), id, req.Nama, req.Deskripsi, req.Harga, *req.Stok)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := productIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrProductNotFound.Error())
		return
	}

	if err := h.products.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func productIDFromPath(path string) (int64, bool) {
	raw := strings.TrimPrefix(path, "/products/")
	if raw == "" || strings.Contains(raw, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw {
		return 0, false
	}
	return id, true
}

func queryInt(r *http.Request, key string) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
