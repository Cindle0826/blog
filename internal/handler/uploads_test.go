package handler

// POST /api/uploads 的測試（API.md「POST /api/uploads」）。
//
// 縮圖由後台在瀏覽器裡做（2026-10-31 決定），後端只負責檢查與存檔：
// 讀進整個檔案 → image.DecodeConfig 判斷格式與寬高 → 長邊超過 1600 回 400 →
// 用自己產生的 {yyyy}/{mm}/{uuid}.{ext} 寫進 GCS（寬高寫進 metadata）→ 回傳 path 與尺寸。
//
// 這裡用假的 imageWriter，不連 GCS。測試假設的介面：
//
//	type imageWriter interface {
//		Put(ctx context.Context, object, contentType string, data []byte, width, height int) error
//	}
//	func NewUploadHandler(images imageWriter) *UploadHandler
//	func (h *UploadHandler) Create(w http.ResponseWriter, r *http.Request)

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// ── 假的 GCS ───────────────────────────────────────────────

type putCall struct {
	object, contentType string
	data                []byte
	width, height       int
}

type fakeImageWriter struct {
	puts []putCall
	err  error
}

func (f *fakeImageWriter) Put(_ context.Context, object, contentType string, data []byte, width, height int) error {
	f.puts = append(f.puts, putCall{object, contentType, bytes.Clone(data), width, height})
	return f.err
}

// ── 產生測試用的圖片 ───────────────────────────────────────

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, solid(w, h)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, solid(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gifBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, solid(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Go 沒有 WebP 編碼器，用一張現成的 1×1 無損 WebP。
func webpBytes(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func solid(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 40, A: 255})
		}
	}
	return img
}

// ── 組 multipart 請求 ──────────────────────────────────────

type part struct {
	field, filename, contentType string
	data                         []byte
}

// multipartBody 組出跟瀏覽器 FormData 一樣的 body，回傳 body 與含 boundary 的 Content-Type。
func multipartBody(t *testing.T, parts ...part) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p.field+`"; filename="`+p.filename+`"`)
		if p.contentType != "" {
			h.Set("Content-Type", p.contentType)
		}
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(p.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func doUpload(t *testing.T, f *fakeImageWriter, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewUploadHandler(f)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /uploads", h.Create)

	req := httptest.NewRequest("POST", "/uploads", body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// uploadFile 是最常見的情況：欄位 file 只放一張圖。
func uploadFile(t *testing.T, f *fakeImageWriter, filename, partType string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, part{"file", filename, partType, data})
	return doUpload(t, f, body, ct)
}

func assertNotStored(t *testing.T, f *fakeImageWriter) {
	t.Helper()
	if len(f.puts) != 0 {
		t.Errorf("寫進 GCS 了 %d 次，驗證失敗時不能寫入", len(f.puts))
	}
}

// path 的格式：/images/{yyyy}/{mm}/{uuid}.{ext}
var uploadPathPattern = regexp.MustCompile(`^/images/(\d{4})/(\d{2})/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.([a-z]+)$`)

var uploadKeys = []string{"bytes", "height", "path", "width"}

// assertUploaded 檢查一次成功的上傳：回應格式、寫進 GCS 的內容、兩邊對得上。
// 回傳回應裡的 path。
func assertUploaded(t *testing.T, rec *httptest.ResponseRecorder, f *fakeImageWriter,
	data []byte, wantExt, wantType string, wantW, wantH int) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("狀態碼 = %d，預期 200；body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q，預期 application/json", ct)
	}

	m := decodeMap(t, rec)
	if got := sortedKeys(m); !slices.Equal(got, uploadKeys) {
		t.Errorf("欄位 = %v\n預期    %v", got, uploadKeys)
	}
	path, _ := m["path"].(string)
	match := uploadPathPattern.FindStringSubmatch(path)
	if match == nil {
		t.Fatalf("path = %q，格式應為 /images/{yyyy}/{mm}/{uuid}.{ext}", path)
	}
	if match[4] != wantExt {
		t.Errorf("副檔名 = %q，預期 %q（要依檔案內容決定，不是使用者的檔名）", match[4], wantExt)
	}
	// 年月是上傳當下的時間。測試剛好跨月的話前後兩個月都算對
	now := time.Now()
	months := []string{now.Format("2006/01"), now.Add(-time.Minute).Format("2006/01")}
	if ym := match[1] + "/" + match[2]; !slices.Contains(months, ym) {
		t.Errorf("年月 = %q，預期 %q（注意 Go 的月份格式是 01，04 是分鐘）", ym, months[0])
	}

	if w, _ := m["width"].(float64); int(w) != wantW {
		t.Errorf("width = %v，預期 %d", m["width"], wantW)
	}
	if h, _ := m["height"].(float64); int(h) != wantH {
		t.Errorf("height = %v，預期 %d", m["height"], wantH)
	}
	if b, _ := m["bytes"].(float64); int(b) != len(data) {
		t.Errorf("bytes = %v，預期 %d（實際收到的檔案大小）", m["bytes"], len(data))
	}

	if len(f.puts) != 1 {
		t.Fatalf("寫進 GCS %d 次，預期 1 次", len(f.puts))
	}
	put := f.puts[0]
	// GCS 的物件名稱沒有 /images/ 前綴：GET /images/{path} 會把 path 對應到物件名稱
	if wantObject := strings.TrimPrefix(path, "/images/"); put.object != wantObject {
		t.Errorf("GCS 物件名稱 = %q，預期 %q（回應的 path 去掉 /images/）", put.object, wantObject)
	}
	if put.contentType != wantType {
		t.Errorf("GCS 的 ContentType = %q，預期 %q（要依檔案內容判斷，不是請求的 Content-Type）",
			put.contentType, wantType)
	}
	if !bytes.Equal(put.data, data) {
		t.Errorf("寫進 GCS 的資料跟上傳的不一樣（%d bytes，上傳的是 %d bytes）。"+
			"常見原因：先用 file.Read 讀了前 512 bytes 判斷格式，之後沒有從頭讀", len(put.data), len(data))
	}
	if put.width != wantW || put.height != wantH {
		t.Errorf("寫進 metadata 的寬高 = %d×%d，預期 %d×%d", put.width, put.height, wantW, wantH)
	}
	return path
}

// ── 成功 ───────────────────────────────────────────────────

func TestUploadPNG(t *testing.T) {
	f := &fakeImageWriter{}
	data := pngBytes(t, 800, 600)
	rec := uploadFile(t, f, "screenshot.png", "image/png", data)
	assertUploaded(t, rec, f, data, "png", "image/png", 800, 600)
}

func TestUploadEachFormat(t *testing.T) {
	tests := []struct {
		name, ext, mime string
		data            func(t *testing.T) []byte
		w, h            int
	}{
		{"JPEG", "jpg", "image/jpeg", func(t *testing.T) []byte { return jpegBytes(t, 640, 480) }, 640, 480},
		{"PNG", "png", "image/png", func(t *testing.T) []byte { return pngBytes(t, 320, 200) }, 320, 200},
		{"GIF", "gif", "image/gif", func(t *testing.T) []byte { return gifBytes(t, 100, 50) }, 100, 50},
		// WebP 的解碼器不在標準庫，要 import _ "golang.org/x/image/webp"
		{"WebP", "webp", "image/webp", webpBytes, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeImageWriter{}
			data := tt.data(t)
			rec := uploadFile(t, f, "img."+tt.ext, tt.mime, data)
			assertUploaded(t, rec, f, data, tt.ext, tt.mime, tt.w, tt.h)
		})
	}
}

// 縮圖由前端做，後端不重新編碼：存進去的 bytes 要跟上傳的完全一樣。
// assertUploaded 已經比對過內容，這裡用比較大的圖再確認一次（避免只有小檔案剛好過）。
func TestUploadStoresBytesUnchanged(t *testing.T) {
	f := &fakeImageWriter{}
	data := jpegBytes(t, 1600, 1200)
	rec := uploadFile(t, f, "photo.jpg", "image/jpeg", data)
	assertUploaded(t, rec, f, data, "jpg", "image/jpeg", 1600, 1200)
}

func TestUploadLongSideLimit(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		ok   bool
	}{
		{"寬剛好 1600", 1600, 900, true},
		{"高剛好 1600（直式）", 900, 1600, true},
		{"正方形 1600", 1600, 1600, true},
		{"寬 1601", 1601, 10, false},
		{"高 1601（直式）", 10, 1601, false},
		{"兩邊都超過", 2000, 1700, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeImageWriter{}
			data := pngBytes(t, tt.w, tt.h)
			rec := uploadFile(t, f, "x.png", "image/png", data)
			if tt.ok {
				assertUploaded(t, rec, f, data, "png", "image/png", tt.w, tt.h)
				return
			}
			// 不是把 height 改成 400，而是整個請求回 HTTP 400：縮圖是前端的事，後端只負責擋
			msg := assertError(t, rec, http.StatusBadRequest, "invalid_request")
			if !strings.Contains(msg, "1600") {
				t.Errorf("訊息 %q 應該說明上限是 1600", msg)
			}
			assertNotStored(t, f)
		})
	}
}

// ── 不能相信前端給的東西 ───────────────────────────────────

func TestUploadIgnoresFilename(t *testing.T) {
	for _, name := range []string{
		"../../etc/passwd.png",
		"我的 截圖 (1).png",
		"a/b/c.png",
		".png",
		strings.Repeat("a", 300) + ".png",
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeImageWriter{}
			data := pngBytes(t, 10, 10)
			rec := uploadFile(t, f, name, "image/png", data)
			path := assertUploaded(t, rec, f, data, "png", "image/png", 10, 10)
			if base := strings.TrimSuffix(name, ".png"); base != "" && strings.Contains(path, base) {
				t.Errorf("path %q 用到了使用者的檔名", path)
			}
		})
	}
}

// 副檔名與 ContentType 要看檔案內容，不看檔名，也不看 part 的 Content-Type（都可以亂填）。
func TestUploadTypeComesFromContent(t *testing.T) {
	tests := []struct {
		name, filename, partType string
	}{
		{"檔名說 jpg", "photo.jpg", "image/png"},
		{"part 說 jpeg", "x.png", "image/jpeg"},
		{"part 說 text/plain", "x.png", "text/plain"},
		{"part 沒有 Content-Type", "x.png", ""},
		{"檔名沒有副檔名", "x", "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeImageWriter{}
			data := pngBytes(t, 20, 10) // 內容一律是 PNG
			rec := uploadFile(t, f, tt.filename, tt.partType, data)
			assertUploaded(t, rec, f, data, "png", "image/png", 20, 10)
		})
	}
}

func TestUploadPathsAreUnique(t *testing.T) {
	f := &fakeImageWriter{}
	data := pngBytes(t, 10, 10)
	first := assertUploaded(t, uploadFile(t, f, "same.png", "image/png", data), f, data, "png", "image/png", 10, 10)
	f.puts = nil
	second := assertUploaded(t, uploadFile(t, f, "same.png", "image/png", data), f, data, "png", "image/png", 10, 10)
	if first == second {
		t.Errorf("同一個檔案上傳兩次得到同一個 path %q，後一次會蓋掉前一次", first)
	}
}

// ── 不是支援的圖片 ─────────────────────────────────────────

func TestUploadRejectsNonImages(t *testing.T) {
	png20 := pngBytes(t, 10, 10)[:20] // 截斷到 IHDR 之前，讀不出寬高

	tests := []struct {
		name string
		data []byte
	}{
		{"純文字", []byte("hello, this is not an image")},
		{"PDF", []byte("%PDF-1.7\n%âãÏÓ\n1 0 obj\n<<>>\nendobj\n")},
		{"HTML", []byte("<!DOCTYPE html><html><body>x</body></html>")},
		{"SVG（可能夾帶 script，不收）", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><script>alert(1)</script></svg>`)},
		{"BMP（不在支援清單）", append([]byte("BM"), make([]byte, 64)...)},
		{"截斷的 PNG", png20},
		{"只有 PNG 簽章", []byte("\x89PNG\r\n\x1a\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeImageWriter{}
			// part 的 Content-Type 謊稱是圖片，後端不能被騙
			rec := uploadFile(t, f, "x.png", "image/png", tt.data)
			assertError(t, rec, http.StatusBadRequest, "invalid_request")
			assertNotStored(t, f)
		})
	}
}

// ── 請求格式錯誤 ───────────────────────────────────────────

func TestUploadBadRequests(t *testing.T) {
	img := pngBytes(t, 10, 10)

	t.Run("欄位名不是 file", func(t *testing.T) {
		f := &fakeImageWriter{}
		body, ct := multipartBody(t, part{"image", "x.png", "image/png", img})
		assertError(t, doUpload(t, f, body, ct), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("沒有任何欄位", func(t *testing.T) {
		f := &fakeImageWriter{}
		body, ct := multipartBody(t)
		assertError(t, doUpload(t, f, body, ct), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("檔案是空的", func(t *testing.T) {
		f := &fakeImageWriter{}
		assertError(t, uploadFile(t, f, "x.png", "image/png", nil), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("一次傳兩張", func(t *testing.T) {
		// 契約是一次一張；多張由前端分開送。不能默默只存第一張
		f := &fakeImageWriter{}
		body, ct := multipartBody(t,
			part{"file", "a.png", "image/png", img},
			part{"file", "b.png", "image/png", img})
		assertError(t, doUpload(t, f, body, ct), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("不是 multipart，是 JSON", func(t *testing.T) {
		f := &fakeImageWriter{}
		body := bytes.NewBufferString(`{"file":"x.png"}`)
		assertError(t, doUpload(t, f, body, "application/json"), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("直接把圖片當 body", func(t *testing.T) {
		f := &fakeImageWriter{}
		assertError(t, doUpload(t, f, bytes.NewBuffer(img), "image/png"), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("沒有 body", func(t *testing.T) {
		f := &fakeImageWriter{}
		_, ct := multipartBody(t)
		assertError(t, doUpload(t, f, &bytes.Buffer{}, ct), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("multipart 但沒有 boundary", func(t *testing.T) {
		f := &fakeImageWriter{}
		body, _ := multipartBody(t, part{"file", "x.png", "image/png", img})
		assertError(t, doUpload(t, f, body, "multipart/form-data"), http.StatusBadRequest, "invalid_request")
		assertNotStored(t, f)
	})
}

// ── 大小上限 10 MB ─────────────────────────────────────────

// paddedPNG 回傳剛好 size bytes 的檔案：合法的 PNG 檔頭，後面補零。
// DecodeConfig 只讀檔頭，所以它會被當成 10×10 的 PNG。
func paddedPNG(t *testing.T, size int) []byte {
	t.Helper()
	data := pngBytes(t, 10, 10)
	if len(data) > size {
		t.Fatalf("PNG 本身就有 %d bytes，比 %d 大", len(data), size)
	}
	return append(data, make([]byte, size-len(data))...)
}

func TestUploadSizeLimit(t *testing.T) {
	t.Run("剛好 10 MB 可以", func(t *testing.T) {
		// 10 MB 是檔案本身的上限；multipart 的分隔線與標頭不能算進去
		f := &fakeImageWriter{}
		data := paddedPNG(t, 10<<20)
		rec := uploadFile(t, f, "x.png", "image/png", data)
		assertUploaded(t, rec, f, data, "png", "image/png", 10, 10)
	})
	t.Run("超過 10 MB 是 413", func(t *testing.T) {
		f := &fakeImageWriter{}
		rec := uploadFile(t, f, "x.png", "image/png", paddedPNG(t, 10<<20+1))
		assertError(t, rec, http.StatusRequestEntityTooLarge, "invalid_request")
		assertNotStored(t, f)
	})
	t.Run("遠超過也是 413，不能是 500", func(t *testing.T) {
		f := &fakeImageWriter{}
		rec := uploadFile(t, f, "x.png", "image/png", paddedPNG(t, 30<<20))
		assertError(t, rec, http.StatusRequestEntityTooLarge, "invalid_request")
		assertNotStored(t, f)
	})
}

// ── 寫入 GCS 失敗 ──────────────────────────────────────────

func TestUploadStoreError(t *testing.T) {
	f := &fakeImageWriter{err: errors.New("googleapi: Error 403: blog-server does not have storage.objects.create access")}
	rec := uploadFile(t, f, "x.png", "image/png", pngBytes(t, 10, 10))
	msg := assertError(t, rec, http.StatusInternalServerError, "internal")
	for _, leak := range []string{"googleapi", "storage.objects", "blog-server"} {
		if strings.Contains(msg, leak) {
			t.Errorf("500 的訊息不能透露內部錯誤：%q", msg)
		}
	}
}
