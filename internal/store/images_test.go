package store

// ImageStore 的測試。不需要模擬器，也不連真的 GCS：
// 用 httptest 開一個假的 GCS，讓真的 storage SDK 打過來，所以測到的是
// 「SDK 實際送出什麼請求、怎麼處理回應」，不是我們自己想像的行為。
//
// 假 GCS 只實作 ImageStore 用到的三種請求（2026-11 用 storage v1.69.0 實測記下來的）：
//
//	Put  → POST /upload/storage/v1/b/{bucket}/o?uploadType=multipart&ifGenerationMatch=0
//	       body 是 multipart/related：第一段 JSON（name、contentType、metadata），第二段檔案內容
//	Stat → GET  /storage/v1/b/{bucket}/o/{object}   回 JSON
//	Open → GET  /{bucket}/{object}                  回檔案內容（XML API）
//
// 物件名稱裡的斜線在網址上是 %2F，r.URL.Path 已經解碼回 2026/10/x.png。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

const testBucket = "test-uploads"

// ── 假的 GCS ───────────────────────────────────────────────

type gcsObject struct {
	data        []byte
	contentType string
	metadata    map[string]string
	etag        string
}

type fakeGCS struct {
	mu       sync.Mutex
	objects  map[string]*gcsObject
	fail     int      // 不是 0 時，每個請求都回這個狀態碼
	requests []string // "GET meta 2026/10/x.png" 這種格式，用來確認打了哪些請求
}

func (f *fakeGCS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail != 0 {
		writeGCSError(w, f.fail, "forced failure")
		return
	}

	metaPrefix := "/storage/v1/b/" + testBucket + "/o/"
	mediaPrefix := "/" + testBucket + "/"

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/upload/storage/v1/b/"+testBucket+"/o":
		f.upload(w, r)

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, metaPrefix):
		name := strings.TrimPrefix(r.URL.Path, metaPrefix)
		f.requests = append(f.requests, "GET meta "+name)
		o, ok := f.objects[name]
		if !ok {
			writeGCSError(w, http.StatusNotFound, "No such object")
			return
		}
		writeGCSResource(w, name, o)

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, mediaPrefix):
		name := strings.TrimPrefix(r.URL.Path, mediaPrefix)
		f.requests = append(f.requests, "GET media "+name)
		o, ok := f.objects[name]
		if !ok {
			writeGCSError(w, http.StatusNotFound, "No such object")
			return
		}
		w.Header().Set("Content-Type", o.contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(o.data)))
		w.Header().Set("ETag", `"`+o.etag+`"`)
		_, _ = w.Write(o.data)

	default:
		writeGCSError(w, http.StatusNotImplemented, "fake GCS does not handle "+r.Method+" "+r.URL.Path)
	}
}

func (f *fakeGCS) upload(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("uploadType") != "multipart" {
		writeGCSError(w, http.StatusNotImplemented, "only multipart uploads are faked")
		return
	}
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		writeGCSError(w, http.StatusBadRequest, err.Error())
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])

	metaPart, err := mr.NextPart()
	if err != nil {
		writeGCSError(w, http.StatusBadRequest, err.Error())
		return
	}
	var meta struct {
		Name        string            `json:"name"`
		ContentType string            `json:"contentType"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(metaPart).Decode(&meta); err != nil {
		writeGCSError(w, http.StatusBadRequest, err.Error())
		return
	}
	dataPart, err := mr.NextPart()
	if err != nil {
		writeGCSError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := io.ReadAll(dataPart)
	if err != nil {
		writeGCSError(w, http.StatusBadRequest, err.Error())
		return
	}

	f.requests = append(f.requests, "POST upload "+meta.Name+" ifGenerationMatch="+r.URL.Query().Get("ifGenerationMatch"))

	// ifGenerationMatch=0 就是 Conditions{DoesNotExist: true}：已經存在就 412
	if r.URL.Query().Get("ifGenerationMatch") == "0" {
		if _, exists := f.objects[meta.Name]; exists {
			writeGCSError(w, http.StatusPreconditionFailed, "At least one of the pre-conditions you specified did not hold.")
			return
		}
	}

	o := &gcsObject{
		data:        data,
		contentType: meta.ContentType,
		metadata:    meta.Metadata,
		etag:        fmt.Sprintf("CJ%dEAE=", len(f.objects)+1),
	}
	f.objects[meta.Name] = o
	writeGCSResource(w, meta.Name, o)
}

func writeGCSResource(w http.ResponseWriter, name string, o *gcsObject) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind":        "storage#object",
		"bucket":      testBucket,
		"name":        name,
		"contentType": o.contentType,
		"size":        strconv.Itoa(len(o.data)), // GCS 的 JSON 把 size 寫成字串
		"etag":        o.etag,
		"generation":  "1",
		"metadata":    o.metadata,
	})
}

func writeGCSError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

// newTestImageStore 回傳接到假 GCS 的 ImageStore。
func newTestImageStore(t *testing.T) (*ImageStore, *fakeGCS) {
	t.Helper()
	f := &fakeGCS{objects: map[string]*gcsObject{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	client, err := storage.NewClient(t.Context(),
		option.WithoutAuthentication(),
		option.WithEndpoint(srv.URL+"/storage/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return NewImageStore(client, testBucket), f
}

// seedImage 直接放一個物件進假 GCS，不經過 ImageStore。
func seedImage(f *fakeGCS, name, contentType string, data []byte, etag string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[name] = &gcsObject{data: data, contentType: contentType, etag: etag}
}

func (f *fakeGCS) object(name string) (*gcsObject, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[name]
	return o, ok
}

func (f *fakeGCS) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

const testImage = "2026/10/0b6c8f1e-3a2d-4c5b-9e7f-1a2b3c4d5e6f.png"

// ── Put ────────────────────────────────────────────────────

func TestImagePutStoresObject(t *testing.T) {
	s, f := newTestImageStore(t)
	data := []byte("\x89PNG fake image bytes")

	if err := s.Put(t.Context(), testImage, "image/png", data, 1600, 900); err != nil {
		t.Fatalf("Put 失敗：%v", err)
	}

	o, ok := f.object(testImage)
	if !ok {
		t.Fatalf("GCS 裡沒有 %s。物件名稱要原樣使用，不能加 images/ 之類的前綴", testImage)
	}
	if !bytes.Equal(o.data, data) {
		t.Errorf("存進去的內容 = %q，預期 %q", o.data, data)
	}
	if o.contentType != "image/png" {
		t.Errorf("ContentType = %q，預期 image/png。GET /images 會直接拿它當回應的 Content-Type", o.contentType)
	}
	if o.metadata["width"] != "1600" || o.metadata["height"] != "900" {
		t.Errorf("metadata = %v，預期 width=1600、height=900（字串）", o.metadata)
	}
}

func TestImagePutDoesNotOverwrite(t *testing.T) {
	s, f := newTestImageStore(t)
	seedImage(f, testImage, "image/png", []byte("original"), "CJ1EAE=")

	err := s.Put(t.Context(), testImage, "image/png", []byte("replacement"), 10, 10)
	if err == nil {
		t.Fatal("物件已經存在，Put 卻沒有回錯誤。要用 If(storage.Conditions{DoesNotExist: true})")
	}

	o, _ := f.object(testImage)
	if string(o.data) != "original" {
		t.Errorf("既有的物件被覆蓋了：內容變成 %q", o.data)
	}
}

func TestImagePutError(t *testing.T) {
	s, f := newTestImageStore(t)
	f.fail = http.StatusForbidden

	if err := s.Put(t.Context(), testImage, "image/png", []byte("x"), 1, 1); err == nil {
		t.Fatal("GCS 回 403，Put 卻沒有回錯誤")
	}
}

// ── Stat ───────────────────────────────────────────────────

func TestImageStat(t *testing.T) {
	s, f := newTestImageStore(t)
	data := []byte("0123456789")
	seedImage(f, testImage, "image/webp", data, "CJ+w4Zr8k4kDEAE=")

	info, err := s.Stat(t.Context(), testImage)
	if err != nil {
		t.Fatalf("Stat 失敗：%v", err)
	}
	if info.ContentType != "image/webp" {
		t.Errorf("ContentType = %q，預期 image/webp", info.ContentType)
	}
	if info.Size != int64(len(data)) {
		t.Errorf("Size = %d，預期 %d", info.Size, len(data))
	}
	if info.ETag != "CJ+w4Zr8k4kDEAE=" {
		t.Errorf("ETag = %q，預期 GCS 給的原始值 CJ+w4Zr8k4kDEAE=（不加引號，引號由 handler 加）", info.ETag)
	}
}

func TestImageStatDoesNotDownload(t *testing.T) {
	s, f := newTestImageStore(t)
	seedImage(f, testImage, "image/png", []byte("x"), "CJ1EAE=")

	if _, err := s.Stat(t.Context(), testImage); err != nil {
		t.Fatalf("Stat 失敗：%v", err)
	}
	for _, req := range f.log() {
		if strings.HasPrefix(req, "GET media") {
			t.Errorf("Stat 下載了檔案內容（%s）。它只該讀 metadata，handler 回 304 時才不用讀內容", req)
		}
	}
}

func TestImageStatNotFound(t *testing.T) {
	s, _ := newTestImageStore(t)

	_, err := s.Stat(t.Context(), testImage)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v，預期 ErrNotFound。storage.ErrObjectNotExist 要轉成 store 的哨兵錯誤，handler 才不用 import storage", err)
	}
}

func TestImageStatError(t *testing.T) {
	// 不存在以外的錯誤要原樣回傳，不能當成 ErrNotFound，也不能 panic（attrs 會是 nil）。
	// 只用 SDK 不會重試的 4xx：5xx 會一直重試到 Stat 的 30 秒 timeout，測試會卡 30 秒。
	cases := map[string]int{
		"沒有權限":  http.StatusForbidden,
		"沒有登入":  http.StatusUnauthorized,
		"請求不合法": http.StatusBadRequest,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			s, f := newTestImageStore(t)
			seedImage(f, testImage, "image/png", []byte("x"), "CJ1EAE=")
			f.fail = code

			_, err := s.Stat(t.Context(), testImage)
			if err == nil {
				t.Fatalf("GCS 回 %d，Stat 卻沒有回錯誤", code)
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("GCS 回 %d 被當成 ErrNotFound。這樣 handler 會回 404，問題就被藏起來了", code)
			}
		})
	}
}

// ── Open ───────────────────────────────────────────────────

func TestImageOpenReadsWholeFile(t *testing.T) {
	// 5 MB：Open 裡如果 defer cancel()，reader 讀到第一個緩衝區（約 16 KB）就會 context canceled。
	// 小檔案會一次進緩衝區，測不出來。
	s, f := newTestImageStore(t)
	data := bytes.Repeat([]byte("0123456789abcdef"), 5<<20/16)
	seedImage(f, testImage, "image/png", data, "CJ1EAE=")

	rc, err := s.Open(t.Context(), testImage)
	if err != nil {
		t.Fatalf("Open 失敗：%v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("讀到 %d bytes 時出錯：%v。Open 不要自己包 timeout 再 defer cancel，reader 是在 Open return 之後才讀的", len(got), err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("讀回 %d bytes，預期 %d bytes", len(got), len(data))
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close 失敗：%v", err)
	}
}

func TestImageOpenNotFound(t *testing.T) {
	// Stat 之後、Open 之前物件剛好被刪掉，handler 應該回 404 而不是 500
	s, _ := newTestImageStore(t)

	rc, err := s.Open(t.Context(), testImage)
	if rc != nil {
		_ = rc.Close()
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v，預期 ErrNotFound", err)
	}
}

func TestImageOpenError(t *testing.T) {
	s, f := newTestImageStore(t)
	seedImage(f, testImage, "image/png", []byte("x"), "CJ1EAE=")
	f.fail = http.StatusForbidden

	rc, err := s.Open(t.Context(), testImage)
	if rc != nil {
		_ = rc.Close()
	}
	if err == nil {
		t.Fatal("GCS 回 403，Open 卻沒有回錯誤")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("GCS 回 403 被當成 ErrNotFound")
	}
}

// ── 整條流程 ───────────────────────────────────────────────

func TestImagePutThenRead(t *testing.T) {
	// 上傳之後，GET /images 讀到的要跟上傳時一樣
	s, _ := newTestImageStore(t)
	data := []byte("GIF89a fake gif bytes")

	if err := s.Put(t.Context(), testImage, "image/gif", data, 3, 2); err != nil {
		t.Fatalf("Put 失敗：%v", err)
	}

	info, err := s.Stat(t.Context(), testImage)
	if err != nil {
		t.Fatalf("Stat 失敗：%v", err)
	}
	if info.ContentType != "image/gif" || info.Size != int64(len(data)) || info.ETag == "" {
		t.Errorf("Stat = %+v，預期 image/gif、%d bytes、有 ETag", info, len(data))
	}

	rc, err := s.Open(t.Context(), testImage)
	if err != nil {
		t.Fatalf("Open 失敗：%v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("讀取失敗：%v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("讀回 %q，預期 %q", got, data)
	}
}
