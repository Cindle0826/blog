# 開發計畫

最後更新：2026-09-21

## 決策紀錄

| 項目 | 決定 |
|---|---|
| 架構 | Go + `html/template`（Cloud Run）＋ Firebase Hosting CDN ＋ Firestore ＋ Firebase Auth |
| 後台 | 獨立的 Vite + React + TypeScript SPA，掛在 `/admin` |
| 網域 | 待定（`cindle.dev` 為首選，DNS 查詢顯示未被註冊） |
| 語言 | 中英混排，`lang="zh-Hant"` |
| 字型 | 待定，預覽站有 A/B/C/D 四案可即時切換 |
| Blog vs 知識庫 | 同一個 collection，用 `kind` 欄位區分；標籤共用 |
| 留言 | 暫不做，全部完成後再評估 giscus |
| 優先序 | SEO > 成本 0 元 > 技術棧偏好 |

## 責任分界

| 範圍 | 負責 |
|---|---|
| `web/templates/`、`web/static/`、`internal/view/`、`web/admin/` | Claude |
| `cmd/server/`、`internal/{config,store,auth,markdown,cache,handler}/`、`deploy/` | cindle |

接縫由三份契約定義，改動前要先對齊：

- 契約① ViewModel → [`internal/view/model.go`](../internal/view/model.go)
- 契約② 後台 API → [`API.md`](API.md)
- 契約③ Firestore 結構 → [`FIRESTORE.md`](FIRESTORE.md)

---

## Phase 0 — 前置作業　`cindle`

- [ ] 0.1　建立新的 GCP 專案（不要用 `line-bot-503410`，計費要隔離）
- [ ] 0.2　升級 Blaze **並設定 Budget Alert $1** ← 一定要在 0.3 之前
- [ ] 0.3　啟用 API：Cloud Run / Firestore / Firebase Hosting / Storage / Artifact Registry
- [ ] 0.4　`npm i -g firebase-tools` 然後 `firebase login`
- [ ] 0.5　註冊網域並指向 Firebase Hosting
- [x] 0.6　建立 repo、`go mod init`、`.gitignore`

## Phase 1 — 凍結契約　`Claude`

- [x] 1.1　`docs/FIRESTORE.md`
- [x] 1.2　`internal/view/model.go`
- [x] 1.3　`docs/API.md`
- [x] 1.4　路由表（見下）
- [ ] 1.5　**cindle review 上面三份**，確認實作得出來

### 路由表

| 方法 | 路徑 | 模板 | ViewModel | 快取 |
|---|---|---|---|---|
| GET | `/` | `home` | `HomeVM` | `s-maxage=600` |
| GET | `/posts` | `list` | `ListVM` | `s-maxage=600` |
| GET | `/posts/{slug}` | `post` | `PostVM` | `s-maxage=3600` |
| GET | `/notes` | `list` | `ListVM` | `s-maxage=600` |
| GET | `/tags` | `tags` | `TagIndexVM` | `s-maxage=3600` |
| GET | `/tags/{slug}` | `list` | `ListVM` | `s-maxage=3600` |
| GET | `/about` | `about` | `AboutVM` | `s-maxage=86400` |
| GET | `/search` | `search` | `SearchVM` | `s-maxage=86400` |
| GET | `/sitemap.xml` | — | — | `s-maxage=3600` |
| GET | `/robots.txt` | — | — | `s-maxage=86400` |
| GET | `/feed.xml` | — | — | `s-maxage=3600` |
| GET | `/search-index.json` | — | — | `s-maxage=3600` |
| GET | `/preview/{id}` | `post` | `PostVM` | `no-store` + `noindex` |
| — | 其他 | `error` | `ErrorVM` | `no-store` |

> URL **一律不帶結尾斜線**。`/posts/` 要 301 到 `/posts`，
> 否則同一頁會有兩個網址，被當成重複內容。

## Phase 2A — 後端骨架　`cindle`

- [ ] 2A.1　`internal/config` — 從環境變數組出 `view.Site`
- [ ] 2A.2　`internal/store` — Firestore client 與 CRUD
- [ ] 2A.3　`internal/markdown` — goldmark + chroma + bluemonday，產出 `html`/`toc`/`readingMin`
- [ ] 2A.4　slug 產生與唯一性檢查
- [ ] 2A.5　`internal/handler/public.go` — 填 `view.*VM`
- [ ] 2A.6　`internal/cache` — 記憶體快取 + `Cache-Control`
- [ ] 2A.7　`cmd/server/main.go` — 本機跑得起來

## Phase 2B — 視覺原型　`Claude`

- [x] 2B.1　設計 token：色彩、尺標、間距、明暗雙主題
- [x] 2B.2　版面：首頁 / 列表 / 文章 / 標籤 / 關於 / 搜尋 / 404
- [x] 2B.3　`internal/view` 渲染層（含 SEO 的 JSON-LD 產生器）
- [x] 2B.4　`cmd/preview` 假資料伺服器
- [x] 2B.5　主題切換（無閃爍）
- [x] 2B.6　`.prose` 內文樣式：程式碼、表格、引言、目錄
- [x] 2B.7　響應式（手機兩列頁首、桌機浮動目錄）
- [ ] 2B.8　**cindle 確認風格與字型方案**
- [ ] 2B.9　依回饋調整

## Phase 3 — 接合　`一起`

- [ ] 3.1　真資料接上模板
- [ ] 3.2　修接縫問題
- [ ] 3.3　跨瀏覽器與無障礙檢查

## Phase 4 — 登入與後台

- [ ] 4.1　Firebase Auth Google 登入設定　`cindle`
- [ ] 4.2　ID token 驗證 middleware + UID 白名單　`cindle`
- [ ] 4.3　`/api/*` 實作　`cindle`
- [ ] 4.4　圖片上傳與縮圖　`cindle`
- [ ] 4.5　React 後台骨架（Vite + TS）　`Claude`
- [ ] 4.6　登入流程與 token 附加　`Claude`
- [ ] 4.7　文章 CRUD 介面　`Claude`
- [ ] 4.8　CodeMirror 6 分割即時預覽　`Claude`
- [ ] 4.9　自動存草稿、圖片拖放　`Claude`
- [ ] 4.10　後台 build 產物接到 Hosting `/admin`　`Claude`

## Phase 5 — SEO 全套

- [x] 5.1　每頁 title / description / canonical（模板端）
- [x] 5.2　Open Graph + Twitter Card
- [x] 5.3　JSON-LD `BlogPosting` / `BreadcrumbList` / `WebSite` / `Person`
- [ ] 5.4　`sitemap.xml` + `robots.txt`　`cindle`
- [ ] 5.5　Atom feed　`cindle`
- [ ] 5.6　OG 圖自動產生　`cindle`（版型由 Claude 提供）
- [ ] 5.7　字型自架 + subset + preload　`Claude`
- [ ] 5.8　Lighthouse 調校（目標 SEO 100 / Perf 95+）　`Claude`
- [ ] 5.9　站內搜尋（`search-index.json` + MiniSearch）　`兩邊`

## Phase 6 — 部署

- [ ] 6.1　多階段 Dockerfile（distroless，目標 < 20MB）
- [ ] 6.2　部署 Cloud Run（`min-instances=0`、256Mi）
- [ ] 6.3　`firebase.json` rewrite 設定
- [ ] 6.4　自訂網域 + SSL
- [ ] 6.5　驗證 CDN 真的有命中
- [ ] 6.6　Search Console 驗證與提交 sitemap
- [ ] 6.7　一週後確認帳單為 $0

## Phase 7 — 之後

留言（giscus）、文章系列、three.js 模型、Analytics、閱讀進度條。

---

## 目前狀態

Phase 1 完成，Phase 2B 完成待確認。

預覽站：

```bash
BLOG_DEV=1 go run ./cmd/preview
```

`BLOG_DEV=1` 會讓模板每次請求重新讀取，改完存檔重新整理就看得到，不用重啟。
右下角的工具列可以切換字型方案（A/B/C/D）與明暗主題。
