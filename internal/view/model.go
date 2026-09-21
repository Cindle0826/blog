// Package view 是「渲染層」——負責把後端給的資料變成 HTML。
//
// 這個 package 的型別就是前後端的契約①：
//   - handler（後端）的工作是「把這些 struct 填滿」
//   - templates（前端）的工作是「把這些 struct 畫出來」
//
// 所以 handler 不需要知道 HTML 長什麼樣，template 也不需要知道 Firestore 長什麼樣。
// 要改畫面 → 只動 web/templates/；要改資料來源 → 只動 internal/store/。
//
// ⚠️ 這裡的欄位如果要改，兩邊都會受影響，改之前先喊一聲。
package view

import "time"

// ─────────────────────────────────────────────────────────────
// 一、每頁都會用到的共用結構
// ─────────────────────────────────────────────────────────────

// Base 是所有頁面 ViewModel 的共同前綴，用嵌入的方式帶進各頁。
//
// 用法（handler 端）：
//
//	vm := view.PostVM{
//	    Base: view.Base{Site: cfg.Site, Meta: meta},
//	    Post: detail,
//	}
type Base struct {
	Site Site // 全站設定，每次請求都一樣
	Meta Meta // 這一頁專屬的 SEO 資料
}

// Site 是全站不變的設定。建議在程式啟動時從環境變數組一次，之後重複使用，
// 不要每個請求重算。
type Site struct {
	Name     string // 站名，例 "Cindle"
	Tagline  string // 一句話介紹，首頁與 og:description 預設值
	BaseURL  string // 正式網址，**不含結尾斜線**，例 "https://cindle.dev"
	Lang     string // <html lang>，中英混排建議 "zh-Hant"
	Author   Author
	Nav      []NavItem    // 頁首導覽列
	Social   []SocialLink // 頁尾社群連結
	BuildTag string       // 靜態資源版本戳，用來 cache busting（例 git short sha）
}

// Author 用於頁尾、about 頁，以及 JSON-LD 的 author 欄位。
type Author struct {
	Name   string
	Bio    string // 短簡介，1–2 句
	Avatar string // 相對路徑或絕對 URL
	Email  string
}

// NavItem 是頁首的一個導覽項目。
type NavItem struct {
	Label string // 顯示文字
	Href  string // 相對路徑，例 "/posts"
}

// SocialLink 是頁尾的一個社群連結。Icon 對應 web/static/img/icons 裡的 sprite id。
type SocialLink struct {
	Label string
	Href  string
	Icon  string // "github" | "x" | "linkedin" | "rss" | "email"
}

// Meta 是這一頁的 SEO 資料。**每個頁面都必須填**，這是 SEO 的命脈。
//
// 填寫規則：
//   - Title 不要自己加站名，模板會自動補成 "頁面標題 · Cindle"
//   - Description 建議 80–160 字元，中文約 40–80 字
//   - Canonical 必須是完整絕對 URL（含 https://），不可留空
type Meta struct {
	Title       string // <title> 與 og:title 的主體
	Description string // <meta name="description"> 與 og:description
	Canonical   string // <link rel="canonical">，完整絕對 URL

	OGType  string // "website"（一般頁）或 "article"（文章頁）
	OGImage string // 完整絕對 URL；留空則模板套用站台預設圖

	NoIndex bool // true 會輸出 <meta name="robots" content="noindex">（草稿預覽用）

	// 以下只有文章頁需要填，用來產生 JSON-LD BlogPosting
	PublishedAt *time.Time
	UpdatedAt   *time.Time
	Keywords    []string

	// Breadcrumb 用於 JSON-LD BreadcrumbList 與頁面麵包屑。
	// 不含「首頁」，模板會自動補在最前面。
	Breadcrumb []Crumb
}

// Crumb 是麵包屑的一節。最後一節的 Href 可以留空（代表當前頁）。
type Crumb struct {
	Label string
	Href  string
}

// ─────────────────────────────────────────────────────────────
// 二、文章相關
// ─────────────────────────────────────────────────────────────

// PostSummary 是文章在「列表」中的樣子——首頁、文章列表、標籤頁、相關文章都用它。
// 刻意不包含文章內文，列表頁不應該把全文撈出來。
type PostSummary struct {
	Slug        string // URL 片段，例 "why-i-chose-go"，不含斜線
	Title       string
	Summary     string // 摘要；後端可從內文前 N 字自動產生
	Cover       *Image // 封面圖，可為 nil
	Tags        []Tag
	PublishedAt time.Time
	ReadingMin  int  // 預估閱讀分鐘數，後端算好（中文 ÷350、英文 ÷220 字/分）
	Pinned      bool // 置頂
}

// PostDetail 是單篇文章頁的完整資料。
type PostDetail struct {
	PostSummary

	// HTML 是 Markdown 轉換後的內文。
	//
	// ⚠️ 安全性：這個字串會用 template.HTML 原樣輸出，**不會**被跳脫。
	//    後端務必在 internal/markdown 裡跑過 bluemonday 清洗，否則就是 XSS 破口。
	//    雖然只有你能發文，但圖片 alt、外部引用都可能夾帶東西，不要省這步。
	HTML string

	UpdatedAt *time.Time // 有修改過才填，沒有就 nil（模板才知道要不要顯示「最後更新」）

	TOC     []TOCItem     // 目錄，從 h2/h3 擷取；空的話模板不顯示目錄欄
	Related []PostSummary // 相關文章，建議同 tag 取 3 篇
	Prev    *PostSummary  // 上一篇（較舊），可為 nil
	Next    *PostSummary  // 下一篇（較新），可為 nil
	HasCode bool          // 內文有程式碼區塊時才載入 highlight 的 CSS
	HasMath bool          // 保留給之後的數學公式支援
	IsDraft bool          // 草稿預覽模式，模板會顯示醒目橫幅
}

// TOCItem 是目錄的一個項目。Level 是 2 或 3（對應 h2 / h3）。
type TOCItem struct {
	Level  int
	Text   string
	Anchor string // 不含 # 的錨點 id
}

// Tag 同時扮演 blog 分類與知識庫分類。
// 「哪些 tag 算知識庫」由 Site 設定或 tag 本身的旗標決定，不在這層硬寫。
type Tag struct {
	Slug  string // URL 用，例 "kubernetes"
	Name  string // 顯示用，例 "Kubernetes"
	Count int    // 文章數；列表頁用得到，文章頁可以是 0
}

// Image 是一張圖。Width/Height 必填——沒有的話瀏覽器無法預留空間，
// 會造成版面位移（CLS），直接扣 Core Web Vitals 分數。
type Image struct {
	URL    string
	Alt    string
	Width  int
	Height int
}

// ─────────────────────────────────────────────────────────────
// 三、各頁面的 ViewModel
//     每個 struct 對應 web/templates/pages/ 底下的一個檔案
// ─────────────────────────────────────────────────────────────

// HomeVM → pages/home.gohtml　　路由 GET /
type HomeVM struct {
	Base
	Intro       string        // 首頁自我介紹段落（可含 HTML）
	RecentPosts []PostSummary // 最新文章，建議 5 篇
	RecentNotes []PostSummary // 最新知識庫筆記，建議 5 篇
	TopTags     []Tag         // 熱門標籤，建議 8 個
}

// ListVM → pages/list.gohtml
// 同時服務三種路由，靠 Heading/Breadcrumb 區分：
//
//	GET /posts          全部文章
//	GET /notes          知識庫
//	GET /tags/{slug}    某個標籤
type ListVM struct {
	Base
	Heading     string // 頁面大標，例 "所有文章" / "Kubernetes"
	Description string // 大標底下的說明，可留空
	Posts       []PostSummary
	Pagination  Pagination
	ActiveTag   *Tag // 標籤頁才填，用來高亮
}

// PostVM → pages/post.gohtml　　路由 GET /posts/{slug}
type PostVM struct {
	Base
	Post PostDetail
}

// TagIndexVM → pages/tags.gohtml　　路由 GET /tags
type TagIndexVM struct {
	Base
	Tags []Tag // 已依 Count 由多到少排序
}

// AboutVM → pages/about.gohtml　　路由 GET /about
type AboutVM struct {
	Base
	HTML string // 關於頁內文，一樣是清洗過的 Markdown 輸出
}

// SearchVM → pages/search.gohtml　　路由 GET /search
// 搜尋完全在瀏覽器端做（MiniSearch 讀 /search-index.json），
// 所以這個 VM 幾乎是空的，只是提供一個有 SEO meta 的殼。
type SearchVM struct {
	Base
}

// ErrorVM → pages/error.gohtml　　404 / 500
type ErrorVM struct {
	Base
	Code    int    // HTTP 狀態碼
	Message string // 給人看的訊息，不要吐內部錯誤細節
}

// Pagination 是分頁狀態。PrevURL/NextURL 留空代表沒有上/下一頁。
type Pagination struct {
	Page       int // 目前頁碼，從 1 開始
	TotalPages int
	PrevURL    string
	NextURL    string
}

// HasPages 回報是否需要顯示分頁元件。
func (p Pagination) HasPages() bool { return p.TotalPages > 1 }
