package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLog 在測試期間把 slog 輸出導到 buffer，結束後還原成 TestMain 設的 io.Discard。
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestRecoverPassesThroughWithoutPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	Recover(next).ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "ok" {
		t.Errorf("沒有 panic 時要原封不動：得到 %d %q", rec.Code, rec.Body.String())
	}
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	logs := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p *string
		_ = *p // nil pointer dereference，跟 PATCH 開發時遇到的一樣
	})
	rec := httptest.NewRecorder()
	Recover(next).ServeHTTP(rec, httptest.NewRequest("PATCH", "/posts/abc", nil))

	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	if strings.Contains(msg, "nil pointer") {
		t.Errorf("回給前端的訊息不能帶 panic 內容：%q", msg)
	}

	out := logs.String()
	for _, want := range []string{"handler panic", "PATCH", "/posts/abc", "nil pointer", "stack="} {
		if !strings.Contains(out, want) {
			t.Errorf("log 少了 %q：\n%s", want, out)
		}
	}
	// stack 要指得到出事的那一行，不然 log 只知道「nil pointer」
	if !strings.Contains(out, "middleware_test.go") {
		t.Errorf("stack 裡應該有出事的檔案：\n%s", out)
	}
}

func TestRecoverPanicWithError(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something broke")
	})
	rec := httptest.NewRecorder()
	Recover(next).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assertError(t, rec, http.StatusInternalServerError, "internal")
}

// http.ErrAbortHandler 是 net/http 約定的「故意中止」，要原樣交還，不能吞掉變成 500。
func TestRecoverRepanicsErrAbortHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("recover() = %v，預期重新 panic http.ErrAbortHandler", v)
		}
	}()
	Recover(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	t.Error("沒有重新 panic")
}

func TestLoggerCallsNextAndLogs(t *testing.T) {
	logs := captureLog(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})
	rec := httptest.NewRecorder()
	Logger(next).ServeHTTP(rec, httptest.NewRequest("POST", "/api/posts", nil))

	if !called || rec.Code != http.StatusCreated {
		t.Errorf("Logger 要呼叫 next 並保留它的回應：called=%v code=%d", called, rec.Code)
	}
	out := logs.String()
	for _, want := range []string{"method=POST", "url=/api/posts"} {
		if !strings.Contains(out, want) {
			t.Errorf("log 少了 %q：\n%s", want, out)
		}
	}
}

// main.go 的順序是 Logger(Recover(root))：panic 被接住後，Logger 還記得到這筆請求。
func TestLoggerStillLogsWhenRecoverCatchesPanic(t *testing.T) {
	logs := captureLog(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	rec := httptest.NewRecorder()
	Logger(Recover(next)).ServeHTTP(rec, httptest.NewRequest("GET", "/api/posts/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("狀態碼 = %d，預期 500", rec.Code)
	}
	if !strings.Contains(logs.String(), "API Handle time") {
		t.Errorf("Logger 沒有記到發生 panic 的請求：\n%s", logs.String())
	}
}
