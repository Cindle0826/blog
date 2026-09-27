package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"cindle.dev/blog/internal/auth"
	h "cindle.dev/blog/internal/handler"
	"cindle.dev/blog/internal/util"
)

func main() {
	// init slog

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	api := http.NewServeMux()
	api.HandleFunc("POST /posts", h.PingHandler)

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", auth.RequireAdmin(api)))

	// middleware
	var handler http.Handler = root
	handler = h.Logger(handler)

	addr := ":" + util.GetEnvOr("PORT", "8080")
	slog.Info("Server Running ...", "port", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}
