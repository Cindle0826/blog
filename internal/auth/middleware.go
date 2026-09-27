package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type ErrorMsg struct {
	Error ErrorMsgDetail `json:"error"`
}

type ErrorMsgDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		if token := resolveToken(r); len(token) == 0 {
			unauthorized := ErrorMsgDetail{
				"unauthorized", "請重新登入"}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(ErrorMsg{unauthorized}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}
		slog.Info("resolve token", "token", token)

		next.ServeHTTP(w, r)
	})
}

func resolveToken(r *http.Request) string {
	token := r.Header.Get("Authorization")

	if !strings.Contains(token, "Bearer") {
		return ""
	}
	start := strings.Index(token, "Bearer ")
	return token[start:]
}
