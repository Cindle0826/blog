package util

import (
	"log/slog"
	"net/http"
	"time"
)

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)
		slog.Info("API Handle time",
			"method", r.Method,
			"url", r.URL.Path,
			"ip", r.RemoteAddr,
			"costs(MS)", time.Since(start).Milliseconds(),
		)
	})
}
