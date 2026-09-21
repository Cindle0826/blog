package view

import (
	"bytes"
	"fmt"
	htmltmpl "html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"cindle.dev/blog/web"
)

// Renderer 負責載入與執行模板。
//
// 兩種模式：
//   - 正式環境：啟動時解析一次，之後重複使用（快）
//   - 開發模式：每次渲染都從磁碟重讀（改模板不用重啟 server）
//
// 開發模式由環境變數 BLOG_DEV=1 開啟。
type Renderer struct {
	dev     bool
	devRoot string // 開發模式下模板在磁碟上的位置（相對於專案根目錄）

	mu    sync.RWMutex
	set   map[string]*htmltmpl.Template // key 是頁面名稱，例 "post"
	funcs htmltmpl.FuncMap
}

// NewRenderer 建立渲染器並解析所有模板。
// 解析失敗會直接回錯，讓程式在啟動時就炸——不要等到有人連進來才發現模板壞了。
func NewRenderer(site Site) (*Renderer, error) {
	r := &Renderer{
		dev:     os.Getenv("BLOG_DEV") == "1",
		devRoot: "web/templates",
		funcs:   buildFuncMap(site),
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// load 解析 layouts + partials + 每一個 page，組成數組獨立的 template。
//
// 為什麼每頁要各自獨立一組？因為 html/template 裡同名的 {{define}} 會互相覆蓋。
// 每頁都各自載入「共用檔 + 自己這頁」，就不會打架。
func (r *Renderer) load() error {
	src, err := r.source()
	if err != nil {
		return err
	}

	shared, err := fs.Glob(src, "layouts/*.gohtml")
	if err != nil {
		return err
	}
	partials, err := fs.Glob(src, "partials/*.gohtml")
	if err != nil {
		return err
	}
	shared = append(shared, partials...)

	pages, err := fs.Glob(src, "pages/*.gohtml")
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return fmt.Errorf("view: 在 pages/ 找不到任何模板")
	}

	set := make(map[string]*htmltmpl.Template, len(pages))
	for _, page := range pages {
		files := make([]string, 0, len(shared)+1)
		files = append(files, shared...)
		files = append(files, page)

		t, err := htmltmpl.New(filepath.Base(page)).
			Funcs(r.funcs).
			ParseFS(src, files...)
		if err != nil {
			return fmt.Errorf("view: 解析 %s 失敗: %w", page, err)
		}
		set[pageName(page)] = t
	}

	r.mu.Lock()
	r.set = set
	r.mu.Unlock()
	return nil
}

// source 回傳模板的來源檔案系統：開發模式讀磁碟，正式環境讀 embed。
func (r *Renderer) source() (fs.FS, error) {
	if r.dev {
		return os.DirFS(r.devRoot), nil
	}
	sub, err := fs.Sub(web.Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("view: 取得 embed 模板子目錄失敗: %w", err)
	}
	return sub, nil
}

// Render 把 data 套進名為 page 的模板，以 status 狀態碼寫進 w。
//
// 重點：先渲染到記憶體 buffer，成功了才寫進 ResponseWriter。
// 否則模板渲染到一半出錯時，使用者已經收到半截 HTML、狀態碼也送出去了，
// 你再也沒機會改成 500。
func (r *Renderer) Render(w http.ResponseWriter, status int, page string, data any) error {
	if r.dev {
		// 開發模式每次重載，改完存檔重新整理就看得到
		if err := r.load(); err != nil {
			return err
		}
	}

	r.mu.RLock()
	t, ok := r.set[page]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("view: 找不到模板 %q", page)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		return fmt.Errorf("view: 渲染 %s 失敗: %w", page, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := buf.WriteTo(w)
	return err
}

// Pages 回傳目前載入的所有頁面名稱，排序過。測試與除錯用。
func (r *Renderer) Pages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.set))
	for n := range r.set {
		names = append(names, n)
	}
	return names
}

// pageName 把 "pages/post.gohtml" 變成 "post"。
func pageName(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}
