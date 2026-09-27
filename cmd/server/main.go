package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"cindle.dev/blog/internal/api"
	"cindle.dev/blog/internal/util"
)

func main() {
	// init slog

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/posts", api.PingHandler)

	// middleware
	var handler http.Handler
	handler = util.LoggerMiddleware(mux)

	addr := ":" + util.GetEnvOr("PORT", "8080")
	slog.Info("Server Running ...", "port", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}
