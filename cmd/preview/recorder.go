package main

import (
	"bytes"
	"net/http"
)

// recorder 是一個把輸出收進 buffer 的 http.ResponseWriter，
// 讓我們有機會在送出去之前改寫 HTML（注入預覽工具列）。
type recorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
