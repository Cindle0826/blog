package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"cindle.dev/blog/internal/httpx"
	"cindle.dev/blog/internal/store"
)

var imageObjectPattern = regexp.MustCompile(`^\d{4}/\d{2}/[0-9a-f-]{36}\.(jpg|png|gif|webp)$`)

type imageReader interface {
	Stat(ctx context.Context, object string) (store.ImageInfo, error)
	Open(ctx context.Context, object string) (io.ReadCloser, error)
}

type ImageHandler struct {
	images imageReader
}

func NewImageHandler(images imageReader) *ImageHandler {
	return &ImageHandler{images: images}
}

func (h *ImageHandler) Get(w http.ResponseWriter, r *http.Request) {

	path := r.PathValue("path")

	if !imageObjectPattern.MatchString(path) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "檔案路徑不合法")
		return
	}

	ctx := r.Context()

	stat, err := h.images.Stat(ctx, path)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此圖片詳情")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "獲取圖片詳情失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Get", "[images_handler] errMsg", err)
		return
	}

	etag := "\"" + stat.ETag + "\""
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	img, err := h.images.Open(ctx, path)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "查無此圖片")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "獲取圖片失敗詳情請查看 Log")
		httpx.WriteErrLog("伺服器回應失敗", "Get", "[images_handler] errMsg", err)
		return
	}
	defer func() {
		_ = img.Close()
	}()

	w.Header().Set("Content-Type", stat.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)

	//if _, err := io.Copy(w, img); err != nil {
	//	httpx.WriteErrLog("圖片回傳失敗", "Get", "[images_handler] errMsg", err)
	//}

	_, _ = io.Copy(w, img)

}
