package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"cindle.dev/blog/internal/httpx"
	"github.com/google/uuid"
)

type imageWriter interface {
	Put(ctx context.Context, object, contentType string, data []byte, width, height int) error
}

type UploadHandler struct {
	images imageWriter
}

type uploadResponse struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int    `json:"bytes"`
}

func NewUploadHandler(images imageWriter) *UploadHandler {
	return &UploadHandler{
		images: images,
	}
}

// 只接受 jpeg、png、gif、webp
var imageFormats = map[string]struct{ ext, mime string }{
	"jpeg": {"jpg", "image/jpeg"},
	"png":  {"png", "image/png"},
	"gif":  {"gif", "image/gif"},
	"webp": {"webp", "image/webp"},
}

func (h *UploadHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxMemory+_1MB)

	// 驗證 Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "請求類型必須為 multipart/form-data")
		return
	}

	// 驗證 multipart 是否超過設置大小
	if err := r.ParseMultipartForm(uploadMaxMemory); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "檔案傳送超過 10MB")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "請用 multipart/form-data 上傳，欄位名稱為 file")
		return
	}
	defer func() {
		err := r.MultipartForm.RemoveAll()
		if err != nil {
			httpx.WriteErrLog("清除 MultipartForm 資料失敗", "Uploads", "[uploads_handler] errMsg", err)
		}
	}()

	// 驗證檔案上傳數量
	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "檔案一次只能上傳一個")
		return
	}

	// 開啟檔案
	file, err := files[0].Open()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "檔案開啟失敗")
		httpx.WriteErrLog("檔案開啟失敗", "Uploads", "[uploads_handler] errMsg", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	// 讀取檔案
	data, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "檔案讀取失敗")
		httpx.WriteErrLog("檔案讀取失敗", "Uploads", "[uploads_handler] errMsg", err)
		return
	}

	if len(data) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "檔案不允許上傳為空")
		return
	}

	// 驗證是否超過 body 預設大小
	if len(data) > uploadMaxMemory {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "檔案大小不能超過 10MB")
		return
	}

	// 驗證檔案類型是否允許
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	f, ok := imageFormats[format]
	if err != nil || !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "檔案上傳類型有誤")
		httpx.WriteErrLog("檔案讀取失敗", "Uploads", "[uploads_handler] errMsg", fmt.Errorf("format=%q: %v", format, err))
		return
	}

	if cfg.Width > 1600 || cfg.Height > 1600 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "檔案上傳上限應為 1600")
		return
	}

	// 組裝要上傳的 object
	object := fmt.Sprintf(
		"%s/%s.%s",
		time.Now().Format("2006/01"),
		uuid.NewString(),
		f.ext,
	)

	// 要上返回的 path
	path := "/images/" + object

	err = h.images.Put(r.Context(), object, f.mime, data, cfg.Width, cfg.Height)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "上傳圖檔至 GCS 出現問題")
		httpx.WriteErrLog("上傳圖檔至 GCS 出現問題", "Uploads", "[uploads_handler] errMsg", fmt.Errorf("format=%q: %v", format, err))
		return
	}

	resp := &uploadResponse{
		Path:   path,
		Height: cfg.Height,
		Width:  cfg.Width,
		Bytes:  len(data),
	}

	httpx.WriteJson(w, http.StatusOK, "Uploads", resp)
}
