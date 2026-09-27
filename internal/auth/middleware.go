package auth

import (
	"encoding/json"
	"net/http"
)

type ErrorMsg struct {
	Error ErrorMsgDetail `json:"error"`
}

type ErrorMsgDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("Authorization"); len(v) == 0 {
			unauthorized := ErrorMsgDetail{http.StatusUnauthorized, "請重新登入"}

			if err := json.NewEncoder(w).Encode(unauthorized); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
