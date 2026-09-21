package view

import (
	"encoding/json"
	"fmt"
	htmltmpl "html/template"
	"strings"
	"time"
)

// buildFuncMap 組出模板可以用的函式。
// 這些函式是「畫面邏輯」——只做格式化，不碰資料庫、不做商業邏輯。
// 如果你發現需要在模板裡算什麼複雜的東西，那件事應該搬到 handler 去算好再傳進來。
func buildFuncMap(site Site) htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		// ── 安全輸出 ──────────────────────────────
		"safeHTML": safeHTML,
		"jsonLD":   jsonLD,

		// ── URL ──────────────────────────────────
		"abs":   func(p string) string { return absURL(site.BaseURL, p) },
		"asset": func(p string) string { return assetURL(site.BuildTag, p) },

		// ── 文字 ─────────────────────────────────
		"pageTitle": func(t string) string { return pageTitle(site.Name, t) },
		"truncate":  truncate,
		"joinTags":  joinTags,

		// ── 時間 ─────────────────────────────────
		"isoDate":   isoDate,
		"humanDate": func(t time.Time) string { return humanDate(site.Lang, t) },
		"year":      func() int { return time.Now().Year() },

		// ── 雜項 ─────────────────────────────────
		"dict": dict,
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
		"seq":  seq,
	}
}

// safeHTML 把字串原樣輸出，不做跳脫。
//
// ⚠️ 只能用在「已經被 bluemonday 清洗過」的內容上（文章內文、about 內文）。
//
//	任何使用者可控又沒清洗過的字串丟進來，就是 XSS。
func safeHTML(s string) htmltmpl.HTML { return htmltmpl.HTML(s) }

// jsonLD 把任意結構序列化成 JSON-LD，放進 <script type="application/ld+json">。
//
// Go 的 json.Marshal 預設會把 < > & 轉成 < 之類的跳脫序列，
// 所以內容裡就算有 </script> 也不會提前結束標籤。這是安全的。
func jsonLD(v any) htmltmpl.JS {
	b, err := json.Marshal(v)
	if err != nil {
		// 結構化資料壞掉不該讓整頁掛掉，輸出空物件就好
		return htmltmpl.JS("{}")
	}
	return htmltmpl.JS(b)
}

// absURL 把相對路徑補成完整絕對網址。
// canonical、og:image、sitemap、RSS 都必須是絕對網址，不能用相對路徑。
func absURL(base, p string) string {
	if p == "" {
		return base
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
}

// assetURL 在靜態資源後面掛版本戳，例 /static/css/main.css?v=a1b2c3。
//
// 為什麼需要：CDN 會長時間快取靜態檔（我們會設 max-age=31536000），
// 改了 CSS 卻沒換 URL 的話，使用者會一直看到舊版。
func assetURL(buildTag, p string) string {
	p = "/static/" + strings.TrimLeft(p, "/")
	if buildTag == "" {
		return p
	}
	return p + "?v=" + buildTag
}

// pageTitle 組出 <title>：首頁只顯示站名，其他頁顯示「頁名 · 站名」。
func pageTitle(siteName, t string) string {
	t = strings.TrimSpace(t)
	if t == "" || t == siteName {
		return siteName
	}
	return t + " · " + siteName
}

// truncate 依「字元數」裁切（不是 byte 數），中文才不會被切一半變亂碼。
func truncate(n int, s string) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n]), " ，、。") + "…"
}

// joinTags 把 tag 名稱串成逗號分隔字串，給 meta keywords 用。
func joinTags(tags []Tag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

// isoDate 輸出 RFC3339，給 <time datetime="..."> 與 JSON-LD 用。
// 搜尋引擎讀的是這個，不是給人看的那個格式。
func isoDate(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// humanDate 輸出給人看的日期，依站台語系切換格式。
func humanDate(lang string, t time.Time) string {
	t = t.Local()
	if strings.HasPrefix(lang, "zh") {
		return fmt.Sprintf("%d 年 %d 月 %d 日", t.Year(), int(t.Month()), t.Day())
	}
	return t.Format("Jan 2, 2006")
}

// dict 讓模板可以傳多個參數給 partial：
//
//	{{ template "post-card" dict "Post" . "Compact" true }}
//
// 參數必須成對，key 必須是字串。
func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("dict: 參數必須成對，收到 %d 個", len(kv))
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: 第 %d 個參數不是字串 key", i+1)
		}
		m[k] = kv[i+1]
	}
	return m, nil
}

// seq 產生 1..n 的整數切片，給分頁的頁碼迴圈用。
func seq(n int) []int {
	if n < 1 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}
