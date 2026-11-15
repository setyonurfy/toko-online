package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("response gagal: %v", err)
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
		log.Printf("decode json gagal: %v", err)
		writeError(w, http.StatusBadRequest, "body JSON tidak valid")
		return false
	}
	return true
}
