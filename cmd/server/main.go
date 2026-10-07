package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"cindle.dev/blog/internal/auth"
	"cindle.dev/blog/internal/config"
	h "cindle.dev/blog/internal/handler"
	"cindle.dev/blog/internal/store"
	"firebase.google.com/go/v4"

	"cloud.google.com/go/firestore"
	fa "firebase.google.com/go/v4/auth"
)

func main() {
	// init slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// init config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置啟動失敗", "[main] errMsg", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()

	// init firebase
	fbClient, err := initFirebase(ctx, cfg)
	if err != nil {
		slog.Error("初始化 Firebase 失敗", "[main] errMsg", err.Error())
		os.Exit(1)
	}

	// init firestore
	fsClient, err := initFireStore(ctx, cfg)
	if err != nil {
		slog.Error("初始化 Firestore 失敗", "[main] errMsg", err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = fsClient.Close()
	}()

	// init PostBlog handler
	pbRepo := store.NewPostBlog(fsClient)
	pbApi := h.NewPostBlogController(pbRepo)

	// init api handler
	api := http.NewServeMux()
	api.HandleFunc("POST /posts", pbApi.CreatePost)
	api.HandleFunc("GET /posts/{id}", pbApi.GetPostByID)
	api.HandleFunc("GET /posts", pbApi.GetPosts)
	api.HandleFunc("DELETE /posts/{id}", pbApi.DeletePostByID)

	root := http.NewServeMux()

	// init Auth middleware
	authenticator := auth.NewAuthenticator(fbClient, cfg.Firebase.AdminUIDs)
	root.Handle("/api/", http.StripPrefix("/api", authenticator.RequireAdmin(api)))

	// middleware
	var handler http.Handler = root
	handler = h.Logger(handler)

	addr := ":" + cfg.Server.Port
	slog.Info("Server Running ...", "port", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}

func initFirebase(ctx context.Context, cfg config.Config) (*fa.Client, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.Firebase.ProjectID})
	if err != nil {
		return nil, fmt.Errorf("firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase auth client: %w", err)
	}
	return client, nil
}

func initFireStore(ctx context.Context, cfg config.Config) (*firestore.Client, error) {
	client, err := firestore.NewClient(ctx, cfg.Firestore.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("firestore client: %w", err)
	}
	//defer client.Close()

	return client, nil
}
