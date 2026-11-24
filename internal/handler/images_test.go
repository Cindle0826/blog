package handler

// GET /images/{path...} 的測試（API.md「GET /images/{path...}」）。
//
// 公開路由，不經過 RequireAdmin。把 GCS 的物件串流回去：
// 驗證路徑 → Stat 讀 metadata → 設標頭（ETag 加引號、一年的 immutable 快取）→
// If-None-Match 命中就回 304、不讀內容 → 否則 Open 並用 io.Copy 串流。
//
// 這裡用假的 imageReader，不連 GCS。測試假設的介面：
//
//	type ImageInfo struct {        // internal/store
//		ContentType string
//		Size        int64
//		ETag        string           // GCS 的 attrs.Etag，不帶引號
//	}
//	type imageReader interface {
//		Stat(ctx context.Context, object string) (store.ImageInfo, error)  // 不存在回 store.ErrNotFound
//		Open(ctx context.Context, object string) (io.ReadCloser, error)
//	}
//	func NewImageHandler(images imageReader) *ImageHandler
//	func (h *ImageHandler) Get(w http.ResponseWriter, r *http.Request)

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"cindle.dev/blog/internal/store"
)

// ── 假的 GCS ───────────────────────────────────────────────

type fakeObject struct {
	info store.ImageInfo
	data []byte
}

type fakeImageReader struct {
	objects map[string]fakeObject
	statErr error // 不是 nil 時，Stat 一律回這個錯誤
	openErr error // 不是 nil 時，Open 一律回這個錯誤

	stats, opens []string
	readers      []*trackedReader
}

func (f *fakeImageReader) Stat(_ context.Context, object string) (store.ImageInfo, error) {
	f.stats = append(f.stats, object)
	if f.statErr != nil {
		return store.ImageInfo{}, f.statErr
	}
	o, ok := f.objects[object]
	if !ok {
		return store.ImageInfo{}, store.ErrNotFound
	}
	return o.info, nil
}

func (f *fakeImageReader) Open(_ context.Context, object string) (io.ReadCloser, error) {
	f.opens = append(f.opens, object)
	if f.openErr != nil {
		return nil, f.openErr
	}
	o, ok := f.objects[object]
	if !ok {
		return nil, store.ErrNotFound
	}
	r := &trackedReader{Reader: bytes.NewReader(o.data)}
	f.readers = append(f.readers, r)
	return r, nil
}

// trackedReader 記錄有沒有被 Close，用來確認 handler 有 defer rc.Close()。
type trackedReader struct {
	io.Reader
	closed bool
}

func (r *trackedReader) Close() error {
	r.closed = true
	return nil
}

// ── 測試資料與請求 ─────────────────────────────────────────

const (
	testObject = "2026/10/0b6c8f1e-3a2d-4c5b-9e7f-1a2b3c4d5e6f.png"
	testETag   = "CJ+w4Zr8k4kDEAE=" // GCS 的 Etag 長這樣，不帶引號
)

func newFakeImages(object, contentType string, data []byte) *fakeImageReader {
	return &fakeImageReader{objects: map[string]fakeObject{
		object: {
			info: store.ImageInfo{ContentType: contentType, Size: int64(len(data)), ETag: testETag},
			data: data,
		},
	}}
}

// doImage 走跟 main.go 一樣的路由，所以 {path...} 是真的由 ServeMux 解析出來的。
func doImage(t *testing.T, f *fakeImageReader, method, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /images/{path...}", NewImageHandler(f).Get)

	req := httptest.NewRequest(method, path, nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func getImage(t *testing.T, f *fakeImageReader, object string) *httptest.ResponseRecorder {
	t.Helper()
	return doImage(t, f, http.MethodGet, "/images/"+object, nil)
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("狀態碼 = %d，預期 %d。body：%s", rec.Code, want, rec.Body.String())
	}
}

// ── 正常讀取 ───────────────────────────────────────────────

func TestImageServesContent(t *testing.T) {
	data := pngBytes(t, 40, 30)
	f := newFakeImages(testObject, "image/png", data)

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusOK)

	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Errorf("回傳的內容跟 GCS 裡的不一樣（%d bytes，預期 %d bytes）", rec.Body.Len(), len(data))
	}
	if got := rec.Result().Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q，預期 image/png（用 Stat 回來的值）", got)
	}
	if got := rec.Result().Header.Get("Content-Length"); got != strconv.Itoa(len(data)) {
		t.Errorf("Content-Length = %q，預期 %d（用 Stat 回來的 Size）", got, len(data))
	}
}

func TestImageObjectNameFromPath(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("x"))

	getImage(t, f, testObject)

	if !slices.Equal(f.stats, []string{testObject}) {
		t.Errorf("Stat 收到 %q，預期 [%s]。物件名稱是路徑去掉 /images/，前面不能有斜線", f.stats, testObject)
	}
	if !slices.Equal(f.opens, []string{testObject}) {
		t.Errorf("Open 收到 %q，預期 [%s]", f.opens, testObject)
	}
}

func TestImageContentTypeComesFromStore(t *testing.T) {
	// 回應的 Content-Type 用上傳時存進 GCS 的值，不要從副檔名猜
	cases := []struct{ ext, mime string }{
		{"jpg", "image/jpeg"},
		{"png", "image/png"},
		{"gif", "image/gif"},
		{"webp", "image/webp"},
	}
	for _, c := range cases {
		t.Run(c.ext, func(t *testing.T) {
			object := strings.TrimSuffix(testObject, ".png") + "." + c.ext
			f := newFakeImages(object, c.mime, []byte("data"))

			rec := getImage(t, f, object)
			assertStatus(t, rec, http.StatusOK)
			if got := rec.Result().Header.Get("Content-Type"); got != c.mime {
				t.Errorf("Content-Type = %q，預期 %q", got, c.mime)
			}
		})
	}
}

func TestImageCacheHeaders(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("x"))

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusOK)

	if got := rec.Result().Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q，預期 public, max-age=31536000, immutable。檔名帶 UUID，內容永遠不會變", got)
	}
}

func TestImageETagIsQuoted(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("x"))

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusOK)

	want := `"` + testETag + `"`
	if got := rec.Result().Header.Get("ETag"); got != want {
		t.Errorf("ETag = %s，預期 %s。HTTP 的 ETag 要有雙引號，GCS 給的沒有，要自己加", got, want)
	}
}

func TestImageStreamsLargeFile(t *testing.T) {
	// 3 MB，確認整份都送到了，而且讀完有 Close
	data := bytes.Repeat([]byte("0123456789abcdef"), 3<<20/16)
	f := newFakeImages(testObject, "image/png", data)

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusOK)

	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Errorf("內容不完整：收到 %d bytes，預期 %d bytes", rec.Body.Len(), len(data))
	}
	if len(f.readers) != 1 || !f.readers[0].closed {
		t.Error("Open 回來的 reader 沒有 Close。要 defer rc.Close()，不然 GCS 的連線會一直佔著")
	}
}

func TestImageNoAuthNeeded(t *testing.T) {
	// handler 本身不看 Authorization；公開路由要掛在 root，不能掛在 RequireAdmin 底下（main.go）
	f := newFakeImages(testObject, "image/png", []byte("x"))

	rec := doImage(t, f, http.MethodGet, "/images/"+testObject, http.Header{})
	assertStatus(t, rec, http.StatusOK)
}

// ── 304 ────────────────────────────────────────────────────

func TestImageNotModified(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("image data"))

	rec := doImage(t, f, http.MethodGet, "/images/"+testObject,
		http.Header{"If-None-Match": {`"` + testETag + `"`}})
	assertStatus(t, rec, http.StatusNotModified)

	if rec.Body.Len() != 0 {
		t.Errorf("304 不能有 body，收到 %d bytes", rec.Body.Len())
	}
	if len(f.opens) != 0 {
		t.Error("ETag 命中時不該呼叫 Open。內容不會變，連 GCS 的內容都不用讀")
	}
	if got := rec.Result().Header.Get("ETag"); got != `"`+testETag+`"` {
		t.Errorf("304 也要帶 ETag，得到 %q", got)
	}
	if got := rec.Result().Header.Get("Cache-Control"); got == "" {
		t.Error("304 也要帶 Cache-Control，不然瀏覽器會把快取時間重設掉")
	}
}

func TestImageETagMismatch(t *testing.T) {
	cases := map[string]string{
		"別的 ETag":   `"something-else"`,
		"沒帶引號的同一個值": testETag, // 瀏覽器一定會帶引號；沒帶就不算同一個
	}
	for name, inm := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeImages(testObject, "image/png", []byte("image data"))

			rec := doImage(t, f, http.MethodGet, "/images/"+testObject,
				http.Header{"If-None-Match": {inm}})
			assertStatus(t, rec, http.StatusOK)
			if rec.Body.String() != "image data" {
				t.Errorf("body = %q，預期完整內容", rec.Body.String())
			}
		})
	}
}

// ── HEAD ───────────────────────────────────────────────────

func TestImageHead(t *testing.T) {
	// "GET /images/{path...}" 這個 pattern 也會接 HEAD
	f := newFakeImages(testObject, "image/png", []byte("image data"))

	rec := doImage(t, f, http.MethodHead, "/images/"+testObject, nil)
	assertStatus(t, rec, http.StatusOK)

	if got := rec.Result().Header.Get("Content-Length"); got != "10" {
		t.Errorf("HEAD 的 Content-Length = %q，預期 10", got)
	}
	if got := rec.Result().Header.Get("ETag"); got == "" {
		t.Error("HEAD 也要帶 ETag")
	}
}

// ── 錯誤 ───────────────────────────────────────────────────

func TestImageNotFound(t *testing.T) {
	f := &fakeImageReader{objects: map[string]fakeObject{}}

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusNotFound)

	if len(f.opens) != 0 {
		t.Error("物件不存在時不該呼叫 Open")
	}
	if got := rec.Result().Header.Get("Cache-Control"); strings.Contains(got, "immutable") {
		t.Errorf("404 不能帶 immutable 的快取標頭（%q），不然 CDN 會把「找不到」快取一年", got)
	}
}

func TestImageRejectsInvalidPaths(t *testing.T) {
	// 這裡直接呼叫 Get 並用 SetPathValue 塞路徑，不經過 ServeMux：
	// ServeMux 會先把 .. 清掉再轉址，經過它就測不到 handler 自己的檢查。
	// 只接受上傳時產生的格式 {yyyy}/{mm}/{uuid}.{jpg|png|gif|webp}，其他一律 404，而且不能去查 GCS。
	cases := map[string]string{
		"空的":          "",
		"路徑穿越":        "../../etc/passwd",
		"中間夾 ..":      "2026/10/../../" + strings.TrimPrefix(testObject, "2026/10/"),
		"開頭多一個斜線":     "/" + testObject,
		"後面多一層":       testObject + "/extra",
		"不是 UUID":     "2026/10/sample.png",
		"沒有副檔名":       strings.TrimSuffix(testObject, ".png"),
		"不支援的副檔名":     strings.TrimSuffix(testObject, ".png") + ".svg",
		"jpeg 不是 jpg": strings.TrimSuffix(testObject, ".png") + ".jpeg",
		"年份不是四位數":     "26/10/0b6c8f1e-3a2d-4c5b-9e7f-1a2b3c4d5e6f.png",
		"少了月份":        "2026/0b6c8f1e-3a2d-4c5b-9e7f-1a2b3c4d5e6f.png",
		"反斜線":         `2026\10\0b6c8f1e-3a2d-4c5b-9e7f-1a2b3c4d5e6f.png`,
		"NUL 字元":      testObject + "\x00",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeImages(testObject, "image/png", []byte("x"))

			req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
			req.SetPathValue("path", path)
			rec := httptest.NewRecorder()
			NewImageHandler(f).Get(rec, req)

			assertStatus(t, rec, http.StatusNotFound)
			if len(f.stats) != 0 || len(f.opens) != 0 {
				t.Errorf("不合法的路徑不該查 GCS（Stat %q、Open %q）", f.stats, f.opens)
			}
		})
	}
}

func TestImageTraversalThroughRouter(t *testing.T) {
	// api_test.http 第 16 則：經過 ServeMux 的路徑穿越。不管是轉址還是 404，都不能是 200
	f := newFakeImages(testObject, "image/png", []byte("x"))

	rec := doImage(t, f, http.MethodGet, "/images/../../etc/passwd", nil)
	if rec.Code == http.StatusOK {
		t.Fatalf("路徑穿越回了 200")
	}
	if len(f.stats) != 0 {
		t.Errorf("路徑穿越不該查到 GCS（Stat %q）", f.stats)
	}
}

func TestImageStatError(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("x"))
	f.statErr = errors.New("googleapi: Error 403: blog-server does not have storage.objects.get access")

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusInternalServerError)

	if len(f.opens) != 0 {
		t.Error("Stat 失敗時不該呼叫 Open")
	}
	assertNoLeak(t, rec)
	if got := rec.Result().Header.Get("Cache-Control"); strings.Contains(got, "immutable") {
		t.Errorf("500 不能帶 immutable 的快取標頭（%q）", got)
	}
}

func TestImageOpenError(t *testing.T) {
	f := newFakeImages(testObject, "image/png", []byte("x"))
	f.openErr = errors.New("storage: connection reset by peer")

	rec := getImage(t, f, testObject)
	assertStatus(t, rec, http.StatusInternalServerError)
	assertNoLeak(t, rec)
	if got := rec.Result().Header.Get("Cache-Control"); strings.Contains(got, "immutable") {
		t.Errorf("500 不能帶 immutable 的快取標頭（%q）。先 Open 成功，再設 Cache-Control", got)
	}
}

func assertNoLeak(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := rec.Body.String()
	for _, s := range []string{"googleapi", "storage", "blog-server", "connection reset"} {
		if strings.Contains(body, s) {
			t.Errorf("回應洩漏了內部錯誤（含 %q）：%s", s, body)
		}
	}
}
