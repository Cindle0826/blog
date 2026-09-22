# 開發計畫

最後更新：2026-09-22

## 決策紀錄

| 項目 | 決定 |
|---|---|
| 架構 | Go + `html/template`（Cloud Run）＋ Firebase Hosting CDN ＋ Firestore ＋ Firebase Auth |
| 後台 | 獨立的 Vite + React + TypeScript SPA，掛在 `/admin` |
| 網域 | `cindle.dev`（DNS 查詢未被註冊）。等其餘項目確認後再購買 |
| 語言 | 中英混排，`lang="zh-Hant"` |
| 字型 | **方案 A** — 標題 JetBrains Mono ／ 內文 Inter ／ 中文思源黑體（2026-09-22 確認） |
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

基礎設施改由 Terraform 管理，見 [`../infrastruct_as_code/`](../infrastruct_as_code/)。
結構與 `line-bot-ledger` 相同。

- [x] 0.1　建立 GCP 專案 `cindle-blog`（專案編號 802326727219）
- [x] 0.2　寫好 IaC（Terraform + 腳本）
- [x] 0.3　`./scripts/00-prereqs.sh` — 接計費帳戶
- [x] 0.4　`./scripts/01-state-bucket.sh` — 建 state bucket
- [x] 0.5　`./scripts/02-apply.sh --apply` — 25 個資源建立完成（2026-09-22）
      Cloud Run：`https://blog-3e5cnz7acq-de.a.run.app`（目前是佔位 image）
- [ ] 0.6　`./scripts/03-local-dev-auth.sh` — 本機 ADC
- [x] 0.7　Console 開啟 Firebase Auth 的 Google 登入
- [ ] 0.7b　設定 `admin_uids` — 等 Phase 4.6 用 Google 登入後取得
- [ ] 0.8　`npm i -g firebase-tools` 然後 `firebase login`
- [ ] 0.9　註冊 `cindle.dev` 並指向 Firebase Hosting
- [x] 0.10　建立 repo、`go mod init`、`.gitignore`

隨時可以跑 `./scripts/verify.sh` 看目前狀態（唯讀，約 8 秒）。

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
| GET | `/images/{path...}` | — | — | `max-age=31536000, immutable` |
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
- [x] 2B.8　**cindle 確認風格與字型方案** — 風格通過；字型選 A
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
- [ ] 4.4b　`GET /images/*` 從私有 GCS bucket 串流　`cindle`（規格見 API.md）
- [ ] 4.5　React 後台骨架（Vite + TS）　`Claude`
- [ ] 4.6　登入流程與 token 附加　`Claude`

### 管理帳號的取得方式

一開始是在 Firebase Console 手動建一個 password 帳號來拿 UID。Console
在建立使用者時順手啟用了 Email/Password 登入方式（沒有詢問），等於替後台
多開一條密碼登入路徑，而那個帳號就是 admin。

改成更乾淨的做法：**刪掉那個帳號、停用 Email/Password，UID 等 Google
登入之後再拿。** 這樣拿到的是純 `google.com` provider 的帳號，也不必賭
Firebase 的跨 provider 帳號合併行為會不會保留 UID。

- [x] 刪除手動建立的 password 帳號
- [x] 停用 Email/Password 登入方式
- [x] `admin_uids` 清空，Cloud Run 與本機 `.env` 已同步
- [ ] Phase 4.6 登入後取得真正的 UID，填回 `terraform.tfvars`

在那之前 `ADMIN_UIDS` 是空的。這不擋任何事——`/api/*` 要等 4.3 才存在。
需要測 API 時用 Firebase Auth 模擬器，做法見 [`LOCAL-AUTH.md`](LOCAL-AUTH.md)。
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

Phase 1 完成（三份契約待 cindle review）。Phase 2B 完成，風格與字型已確認。

下一步：cindle 做 Phase 0（GCP 環境）與 1.5（review 契約）；
Claude 等契約確認後開始 Phase 4 的 React 後台。

預覽站：

```bash
BLOG_DEV=1 go run ./cmd/preview
```

`BLOG_DEV=1` 會讓模板每次請求重新讀取，改完存檔重新整理就看得到，不用重啟。
右下角的工具列可以切換字型方案（A/B/C/D）與明暗主題。
