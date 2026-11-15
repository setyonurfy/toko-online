package handler

import "net/http"

func NewRouter(auth *AuthHandler, tokens TokenParser, authLimiter *RateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/auth/register", authLimiter.Middleware(http.HandlerFunc(auth.Register)))
	mux.Handle("/auth/login", authLimiter.Middleware(http.HandlerFunc(auth.Login)))
	mux.Handle("/me", Authenticate(tokens, http.HandlerFunc(auth.Me)))
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
