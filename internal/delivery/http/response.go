package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("response gagal", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14) // 16 KB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var terlaluBesar *http.MaxBytesError
		if errors.As(err, &terlaluBesar) {
			writeError(w, http.StatusRequestEntityTooLarge, "body terlalu besar")
			return false
		}
		slog.Info("decode json gagal", "error", err)
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
}
