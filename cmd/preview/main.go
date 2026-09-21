// Command preview 是「只給開發用」的假資料伺服器。
//
// 它存在的理由：前端（模板 + CSS）和後端（Firestore + handler）要能平行開工。
// 這支程式用寫死的假資料把每一頁都渲染出來，所以在 internal/store 還沒寫好之前，
// 畫面就可以先做到定案。
//
// 它同時也是模板的煙霧測試——模板語法打錯、欄位名稱拼錯，這裡會馬上炸出來。
//
// 用法：
//
//	go run ./cmd/preview          # http://localhost:8080
//	BLOG_DEV=1 go run ./cmd/preview   # 改模板不用重啟
//
// ⚠️ 這支程式不會進 Docker image（Dockerfile 只 build ./cmd/server）。
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cindle.dev/blog/internal/view"
)

const baseURL = "http://localhost:8080"

func main() {
	site := fixtureSite()

	r, err := view.NewRenderer(site)
	if err != nil {
		log.Fatalf("模板載入失敗: %v", err)
	}
	log.Printf("已載入頁面模板: %v", r.Pages())

	mux := http.NewServeMux()

	// 靜態資源直接從磁碟服務，改了 CSS 重新整理就看得到
	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("web/static"))))

	mux.HandleFunc("GET /{$}", page(r, "home", func() any { return fixtureHome(site) }))
	mux.HandleFunc("GET /posts", page(r, "list", func() any {
		return fixtureList(site, "所有文章", "技術筆記、踩坑紀錄，還有一些雜念。", "/posts")
	}))
	mux.HandleFunc("GET /notes", page(r, "list", func() any {
		return fixtureList(site, "知識庫", "整理過、會持續更新的長青內容。", "/notes")
	}))
	mux.HandleFunc("GET /tags", page(r, "tags", func() any { return fixtureTags(site) }))
	mux.HandleFunc("GET /about", page(r, "about", func() any { return fixtureAbout(site) }))
	mux.HandleFunc("GET /search", page(r, "search", func() any { return fixtureSearch(site) }))

	// 字型方案並排比較頁，只有預覽伺服器有
	mux.HandleFunc("GET /fonts", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fontsPage))
	})

	mux.HandleFunc("GET /posts/{slug}", func(w http.ResponseWriter, req *http.Request) {
		render(w, r, http.StatusOK, "post", fixturePost(site, req.PathValue("slug")))
	})
	mux.HandleFunc("GET /tags/{slug}", func(w http.ResponseWriter, req *http.Request) {
		slug := req.PathValue("slug")
		render(w, r, http.StatusOK, "list", fixtureList(site, titleCase(slug), "", "/tags/"+slug))
	})

	// 其他路徑一律 404，順便把錯誤頁也做出來
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		render(w, r, http.StatusNotFound, "error", fixtureError(site))
	})

	addr := ":" + envOr("PORT", "8080")
	log.Printf("預覽伺服器啟動 → http://localhost%s", addr)
	log.Printf("字型方案並排比較 → http://localhost%s/fonts", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// page 把「產生假資料 → 渲染」包成 handler。
func page(r *view.Renderer, name string, build func() any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		render(w, r, http.StatusOK, name, build())
	}
}

// render 渲染後注入預覽工具列。
//
// 用攔截 ResponseWriter 的方式注入，而不是把工具列寫進模板——
// 這樣正式站的模板保持乾淨，不會有任何只在開發時才需要的東西。
func render(w http.ResponseWriter, r *view.Renderer, status int, name string, data any) {
	rec := &recorder{header: make(http.Header)}
	if err := r.Render(rec, status, name, data); err != nil {
		log.Printf("渲染 %s 失敗: %v", name, err)
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	html := strings.Replace(rec.body.String(), "</body>", devToolbar+"</body>", 1)

	for k, v := range rec.header {
		w.Header()[k] = v
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(html)))
	w.WriteHeader(rec.status)
	_, _ = w.Write([]byte(html))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// titleCase 只是預覽用的簡易首字大寫；正式站的 tag 顯示名稱由 Firestore 直接存。
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func daysAgo(n int) time.Time {
	return time.Now().AddDate(0, 0, -n).Truncate(time.Hour)
}
