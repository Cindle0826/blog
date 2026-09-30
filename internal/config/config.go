package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Server   Server
	Firebase Firebase

	// BLOG_DEV=1。模板每次請求重讀、開 CORS，以及允許連 Auth 模擬器。
	Dev bool
}

type Server struct {
	Port string
}

type Firebase struct {
	ProjectID string

	// /api/* 的白名單。用 map 而不是 slice——查詢 O(1)，
	// 而且 `if !admins[uid]` 讀起來就是「不在白名單」。
	AdminUIDs map[string]bool
}

// Load 讀環境變數。設定不完整就回 error，不要退回預設值——
// 白名單沒設是部署疏失，而疏失的安全解讀是「不給存取」。
func Load() (Config, error) {
	cfg := Config{
		Server: Server{
			Port: GetEnvOr("PORT", "8080"),
		},
		Firebase: Firebase{
			// Cloud Run 會自己注入 GOOGLE_CLOUD_PROJECT，本機由 .env 提供。
			// Firebase project ID 跟 GCP project ID 是同一個字串，不要另開變數。
			ProjectID: GetEnvOr("GOOGLE_CLOUD_PROJECT", ""),
		},
		Dev: os.Getenv("BLOG_DEV") == "1",
	}

	if cfg.Firebase.ProjectID == "" {
		return Config{}, errors.New("GOOGLE_CLOUD_PROJECT 未設定：Admin SDK 要用它比對 ID token 的 aud 宣告")
	}

	admins, err := parseAdminUIDs(os.Getenv("ADMIN_UIDS"))
	if err != nil {
		return Config{}, err
	}
	cfg.Firebase.AdminUIDs = admins

	// 設了這個變數，Admin SDK 就完全跳過簽章驗證（auth/token_verifier.go:172）。
	// 本機是功能，線上等於認證機制不存在，所以在這裡擋掉。
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" && !cfg.Dev {
		return Config{}, errors.New("偵測到 FIREBASE_AUTH_EMULATOR_HOST 但 BLOG_DEV != 1，拒絕啟動")
	}

	return cfg, nil
}

func parseAdminUIDs(raw string) (map[string]bool, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("ADMIN_UIDS 未設定，拒絕啟動")
	}

	admins := make(map[string]bool)
	for s := range strings.SplitSeq(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			admins[s] = true
		}
	}
	// 防 ADMIN_UIDS=,,, ——raw 不是空字串，但拆完一個有效 UID 都沒有。
	if len(admins) == 0 {
		return nil, errors.New("ADMIN_UIDS 只有逗號沒有內容")
	}
	return admins, nil
}

func GetEnvOr(key, def string) string {
	if v := os.Getenv(key); len(strings.TrimSpace(v)) != 0 {
		return v
	}
	return def
}
