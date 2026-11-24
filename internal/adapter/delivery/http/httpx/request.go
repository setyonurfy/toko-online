package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14) // 16 KB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var terlaluBesar *http.MaxBytesError
		if errors.As(err, &terlaluBesar) {
			WriteError(w, http.StatusRequestEntityTooLarge, "body terlalu besar")
			return false
		}
		slog.Info("decode json gagal", "error", err)
		WriteError(w, http.StatusBadRequest, "body JSON tidak valid")
		return false
	}
	return true
}

func QueryInt(r *http.Request, key string) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

// IDFromPath mengambil ID dari path berbentuk "<prefix><id>" secara ketat:
// hanya angka desimal baku ("5" sah; "05", "+5", "5/x", "abc" ditolak).
func IDFromPath(path, prefix string) (int64, bool) {
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

func IDFromPathDenganAkhiran(path, prefix, akhiran string) (int64, bool) {
	if !strings.HasSuffix(path, akhiran) {
		return 0, false
	}
	return IDFromPath(strings.TrimSuffix(path, akhiran), prefix)
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
