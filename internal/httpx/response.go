package httpx

import (
	"encoding/json"
	"net/http"
)

func WriteError(w http.ResponseWriter, code int, codeMsg string, msg string) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	type ErrorMsgDetail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	type ErrorMsg struct {
		Error ErrorMsgDetail `json:"error"`
	}

	errMsg := ErrorMsg{
		Error: ErrorMsgDetail{
			Code:    codeMsg,
			Message: msg,
		},
	}

	_ = json.NewEncoder(w).Encode(errMsg)
}

func WriteJson(w http.ResponseWriter, code int, method string, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		WriteErrLog("伺服器回應失敗", method, "[handler] errMsg", err)
		return
	}
}
