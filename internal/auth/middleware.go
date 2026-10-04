package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"cindle.dev/blog/internal/httpx"
	"firebase.google.com/go/v4/auth"
)

type Authenticator struct {
	client tokenVerifier
	admins map[string]bool
}

type ctxKey int

const uidKey ctxKey = iota

func UIDFrom(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(uidKey).(string)
	return uid, ok
}

type tokenVerifier interface {
	VerifyIDToken(ctx context.Context, token string) (*auth.Token, error)
}

func NewAuthenticator(client tokenVerifier, admins map[string]bool) *Authenticator {
	return &Authenticator{client: client, admins: admins}
}

func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := resolveToken(r)

		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "請重新登入")
			return
		}

		tok, err := a.client.VerifyIDToken(r.Context(), token)
		if err != nil {
			switch {
			case auth.IsIDTokenExpired(err):
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "登入已過期，請重新登入")
			case auth.IsIDTokenInvalid(err):
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "登入憑證無效")
			case auth.IsCertificateFetchFailed(err):
				// 這個不是使用者的錯——是我們連不到 Google 拿公鑰
				slog.Error("取得 Google 公鑰失敗", "errMsg", err.Error())
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "服務暫時無法使用")
			default:
				slog.Error("驗證失敗", "errMsg", err.Error())
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "登入憑證無效")
			}
			return
		}

		if !a.admins[tok.UID] {
			slog.Warn("拒絕非白名單",
				"UID", tok.UID,
				"provider", tok.Firebase.SignInProvider,
			)
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "沒有權限")
			return
		}

		ctx := context.WithValue(r.Context(), uidKey, tok.UID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func resolveToken(r *http.Request) (string, bool) {
	token := r.Header.Get("Authorization")
	const prefix = "bearer"

	if len(token) <= len(prefix) {
		return "", false
	}

	if !strings.EqualFold(token[:len(prefix)], prefix) {
		return "", false
	}

	token = strings.TrimSpace(token[len(prefix):])
	if len(token) == 0 {
		return "", false
	}

	return token, true
}
