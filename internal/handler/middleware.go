package handler

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"cindle.dev/blog/internal/httpx"
)

func Logger(next http.Handler) http.Handler {
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

// Recover 接住 handler 裡的 panic，改回 500 並用 slog 記錄。
//
// 不掛的話，net/http 也會接住 panic（server 不會掛），但它的做法是
// 印一段純文字到 stderr，然後直接切斷連線——前端收不到任何回應，
// fetch 只會得到 network error，看不出是後端有 bug。
//
// 它只保護「這個請求的 goroutine」。handler 裡自己 go func() 開的
// goroutine 如果 panic，沒有人接得住，整個 server 會結束。
//
// 限制：handler 在 panic 前已經寫出 header 的話，狀態碼改不了，
// WriteError 只會在 log 留下 superfluous WriteHeader。
// 這個專案的 handler 都在最後才寫回應，所以實際上不會遇到。
//
// main.go 要掛在 Logger 裡面：Logger(Recover(root))。
// 反過來的話，panic 會直接穿過 Logger，那筆請求就不會出現在 log。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// defer 的函式一定會在這個 handler 結束前執行，
		// 不論 next 是正常 return 還是 panic。panic 往上傳到這一層時，
		// 會先執行這裡，我們才有機會攔下它。
		defer func() {
			// recover() 只有在 defer 的函式裡直接呼叫才有用：
			//   - 有 panic：停止 panic，回傳 panic 的值
			//     （例如 "runtime error: invalid memory address or nil pointer dereference"）
			//   - 沒 panic：回傳 nil
			v := recover()
			if v == nil {
				return
			}
			// http.ErrAbortHandler 是 net/http 約定的「故意中止這個回應」，
			// 不是 bug，也不該記錄成錯誤。重新 panic 交回給 net/http 處理。
			if v == http.ErrAbortHandler {
				panic(v)
			}

			// debug.Stack() 是 panic 當下的呼叫堆疊，log 裡才看得到
			// 是哪個檔案第幾行出事；只有 v 的話只知道「nil pointer」。
			slog.Error("handler panic",
				"method", r.Method,
				"path", r.URL.Path,
				"panic", v,
				"stack", string(debug.Stack()))
			// 回給前端的訊息不帶 panic 內容，內部細節只留在 log
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "伺服器發生錯誤")
		}()

		// 真正執行後面的 handler。這行 panic 的話，會跳到上面的 defer
		next.ServeHTTP(w, r)
	})
}
