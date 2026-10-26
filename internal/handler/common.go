package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cindle.dev/blog/internal/httpx"
)

func handleError(err error, w http.ResponseWriter) {
	var (
		syntaxErr        *json.SyntaxError
		unmarshalTypeErr *json.UnmarshalTypeError
		maxBytesErr      *http.MaxBytesError
	)

	switch {
	case errors.As(err, &syntaxErr):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料傳遞有誤")
	case errors.As(err, &unmarshalTypeErr):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料解析有誤")
	case errors.Is(err, io.EOF):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料傳遞為空")
	case errors.Is(err, io.ErrUnexpectedEOF):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "資料不完整")
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("%s 為不合法參數", fieldName))
	case errors.As(err, &maxBytesErr):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "資料傳遞過大")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "資料其他錯誤，請查看Log")
		httpx.WriteErrLog("接收請求失敗", "handleError", "[handler] errMsg", err)

	}
}

func handleDecodeBody(body io.ReadCloser, req interface{}) error {
	decoder := json.NewDecoder(body)
	// 嚴格判斷參數
	decoder.DisallowUnknownFields()

	return decoder.Decode(req)
}
