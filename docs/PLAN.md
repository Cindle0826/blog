# 開發計畫

最後更新：2026-10-10

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
- [x] 0.6　`./scripts/03-local-dev-auth.sh` — 本機 ADC（本機已實際讀寫 Firestore）
- [x] 0.7　Console 開啟 Firebase Auth 的 Google 登入
- [x] 0.7b　設定 `admin_uids`（`RW2Cpmkv...XgT2`，provider `google.com`）
- [ ] 0.8　`npm i -g firebase-tools` 然後 `firebase login`
- [ ] 0.9　註冊 `cindle.dev` 並指向 Firebase Hosting
- [x] 0.10　建立 repo、`go mod init`、`.gitignore`

隨時可以跑 `./scripts/verify.sh` 看目前狀態（唯讀，約 8 秒）。

## Phase 1 — 凍結契約　`Claude`

- [x] 1.1　`docs/FIRESTORE.md`
- [x] 1.2　`internal/view/model.go`
- [x] 1.3　`docs/API.md`
- [x] 1.4　路由表（見下）
- [x] 1.5　**cindle review 上面三份**，確認實作得出來
  - [x] 契約① `internal/view/model.go`
  - [x] 契約② `docs/API.md`
  - [x] 契約③ `docs/FIRESTORE.md`

2026-09-27 三份都通過。但 review 只確認了「設計上講得通」，不代表實作
不會撞牆——真正的問題要寫下去才會浮出來。

### 契約要改的時候

契約不是凍結，是起點。發現某個欄位難填、某個端點該拆開、某個查詢太貴，
就改——但**改之前先講一聲**，因為另一邊會跟著壞掉。

流程：

1. 在對話裡說「契約② 的 X 我想改成 Y，因為 Z」
2. 兩邊確認影響範圍
3. 改文件 → 改各自的程式碼 → commit 訊息寫清楚為什麼

最糟的情況不是契約錯了，是**一邊偷偷改了而另一邊不知道**——那會變成
「我這邊明明是對的」的互相指責，而且很難查。

最想聽意見的三點：

| 檔案 | 判斷 |
|---|---|
| `FIRESTORE.md` | 存 `markdown` 也存 `html`——用空間換 CPU |
| `model.go` | `PostDetail` 的 `Related` / `Prev` / `Next` 都要額外查詢 |
| `API.md` | `PATCH` 一個端點帶六個連帶動作 |

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

> 實作時開著 [`http_tests/api_test.http`](../http_tests/api_test.http)——
> 契約② 的每個端點都有對應的請求與斷言，做到哪就有幾則變綠。
> Firestore 的 Go 寫法對照見 [`FIRESTORE-GO.md`](FIRESTORE-GO.md)。

- [ ] 2A.1　`internal/config` — 從環境變數組出 `view.Site`
      已讀 `GOOGLE_CLOUD_PROJECT`、`ADMIN_UIDS`、`BLOG_DEV` 與模擬器防呆；
      還沒讀 `BLOG_BASE_URL`、`BLOG_UPLOADS_BUCKET`，也還沒組 `view.Site`
- [ ] 2A.2　`internal/store` — Firestore client 與 CRUD（進度見 4.3 的表）
      已完成：`PostStore` 的 Create、Get、List（後台，Go 端搜尋／排序／分頁）、Delete、SetPublished、
      GetPublishedBySlug、ListPublished、CountPublished（公開頁，Firestore 分頁）
- [ ] 2A.3　`internal/markdown` — goldmark + chroma + bluemonday，產出 `html`/`toc`/`readingMin`
      目前 `Create` 存進去的 `html` 是空的，公開頁面要等這一項
- [ ] 2A.4　slug 產生與唯一性檢查
      產生已完成（`makeSlug` + 單元測試）；唯一性檢查（撞號加 `-2`、回 409）還沒做
- [ ] 2A.5　`internal/handler/public.go` — 填 `view.*VM`
- [ ] 2A.6　`internal/cache` — 記憶體快取 + `Cache-Control`
- [x] 2A.7　`cmd/server/main.go` — 本機跑得起來

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

- [x] 4.1　Firebase Auth Google 登入設定　`cindle`
- [x] 4.2　ID token 驗證 middleware + UID 白名單　`cindle`（寫法見 [`AUTH-GO.md`](AUTH-GO.md)）
      401 / 403 / 200 三條路徑已用真的 Firebase token 實測（2026-09-28）
- [ ] 4.3　`/api/*` 實作　`cindle`（進度見下表）
- [ ] 4.4　圖片上傳與縮圖　`cindle`
- [ ] 4.4b　`GET /images/*` 從私有 GCS bucket 串流　`cindle`（規格見 API.md）
- [x] 4.5　React 後台骨架（Vite + TS）　`Claude`　見 [`web/admin/`](../web/admin/)
- [ ] 4.6　登入流程與 token 附加　`Claude`
      程式碼完成；等 cindle 用真的 Google 帳號登入一次確認
- [x] 4.7　文章 CRUD 介面　`Claude`　列表 / 新增 / 編輯 / 發布 / 刪除 / 標籤改名，
      已用假資料模式走過完整流程；真的資料要等對應的 API 寫完
- [ ] 4.8　CodeMirror 6 分割即時預覽　`Claude`
- [ ] 4.9　自動存草稿、圖片拖放　`Claude`
- [ ] 4.10　後台 build 產物接到 Hosting `/admin`　`Claude`

### 4.3 後台 API 進度

對應 [`API.md`](API.md) 與 [`api_test.http`](../http_tests/api_test.http) 的測試編號。

| 端點 | store | handler | 測試 | 備註 |
|---|---|---|---|---|
| `POST /api/posts` | ✅ | ✅ | 3、11 | 撞號回 409（測試 10）要等 2A.4，**下一步**做這個 |
| `GET /api/posts/{id}` | ✅ | ✅ | 4、9 | |
| `GET /api/posts` | ✅ | ✅ | 5、5b、5c | 分頁在 Go 做，原因寫在 `PostStore.List` 的註解 |
| `DELETE /api/posts/{id}` | ✅ | ✅ | 18、19 | 用 `firestore.Exists` 讓不存在的文件回 NotFound，不然 Firestore 會當成功 |
| `POST /api/posts/{id}/publish` | ✅ | ✅ | 7、8 | 退回草稿不清 `publishedAt`；寫完重新 Get 再回傳，見下方說明 |
| `PATCH /api/posts/{id}` | ⬜ | ⬜ | 6 | slug 唯一性做完後的**下一支**。最大的一支：重算 html/toc、slug 改名寫 redirects、`updatedAt` 要顯式送 `ServerTimestamp` |
| `GET /api/tags` | ⬜ | ⬜ | 12 | |
| `PUT /api/tags/{slug}` | ⬜ | ⬜ | 13 | |
| `POST /api/uploads` | ⬜ | ⬜ | 14 | 4.4，需要 GCS 與縮圖 |
| `GET /images/{path...}` | ⬜ | ⬜ | 15、16 | 4.4b，公開路由，不經過 `RequireAdmin` |
| `POST /api/cache/purge` | — | ⬜ | 17 | 依賴 2A.6 `internal/cache` |

> `tags.count` 決定用聚合查詢即時算（2026-10-07），不存在 `tags` 文件裡
> （[`FIRESTORE-GO.md`](FIRESTORE-GO.md)「一個可能讓你不用維護 count 的選項」）。
> 所以 PATCH、DELETE、publish 都不用處理 count，只有 `GET /api/tags` 要數。
>
> 寫入後要回傳完整物件的 API（POST、publish、之後的 PATCH）一律**寫完再 Get 一次**。
> `ServerTimestamp` 存的是伺服器收到請求的時間（精確到毫秒），`WriteResult.UpdateTime`
> 是 commit 時間，兩者實測差了約 20ms，拿後者填回應會跟之後 GET 到的值對不上。

### 管理帳號

| | |
|---|---|
| UID | `RW2CpmkvuOWdTBMlbBnyco46XgT2` |
| email | `cindle0826@gmail.com` |
| provider | `google.com` |

- [x] UID 已填入 `terraform.tfvars`、Cloud Run、本機 `.env`（三處一致，`terraform plan` 無差異）
- [x] Email/Password 登入方式已停用（2026-10-04 查 Identity Toolkit 設定確認）
- 專案裡另有一個非白名單的 Google 帳號，是用來實測 403 的

最早的管理帳號（`KyYz0RfQ...`）是在 Console 以 password 方式建立的，
後來刪除，改成直接用 Google 登入建立現在這個帳號。過程中確認了一件事：
用 Google 登入同 email 的 password 帳號時，Firebase 會把 provider 換成
`google.com` 並保留原本的 UID。

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
- [ ] 6.4b　把 `cindle.dev` 加進 Firebase 的**授權網域**
      （Authentication → 設定 → 授權網域）。漏了的話正式站的登入會被擋。
      這跟 OAuth 用戶端的 redirect URI 是兩回事——後者永遠是
      `cindle-blog.firebaseapp.com/__/auth/handler`，不需要也不應該去改。
- [ ] 6.4c　替 Firebase 的 Browser API key 加 HTTP referrer 限制
      （目前只有 API 限制，沒有網域限制）。要包含 `localhost`、
      `cindle-blog.firebaseapp.com` 與正式網域
- [ ] 6.5　驗證 CDN 真的有命中
- [ ] 6.6　Search Console 驗證與提交 sitemap
- [ ] 6.7　一週後確認帳單為 $0

## Phase 7 — 之後

留言（giscus）、文章系列、three.js 模型、Analytics、閱讀進度條。

---

## 目前狀態

Phase 0、1、2B 完成。Phase 4 的登入與 middleware、後台 React（4.5、4.7）完成。
後台 API 完成 5 支：`POST /api/posts`、`GET /api/posts/{id}`、`GET /api/posts`、`DELETE /api/posts/{id}`、
`POST /api/posts/{id}/publish`，
皆已用真的 Firestore 實測。2026-10-07 Go 升到 1.27.1，govulncheck 0 個可呼叫漏洞。

下一步：cindle 依 4.3 的表繼續寫後台 API，建議順序是
slug 唯一性 → `PATCH` → 標籤 → 上傳與圖片 → 快取失效。
`tags.count` 改用聚合查詢即時算，寫入的 API 都不用維護它（見 4.3 表下方的說明）。

待確認：4.6 的真實 Google 登入還沒在後台實際走過一次（`npm run dev` + Go server）。

注意：Cloud Run 目前跑的還是佔位的 `hello` image，blog server 要到 Phase 6 才部署。

預覽站（公開站，假資料）：

```bash
BLOG_DEV=1 go run ./cmd/preview
```

`BLOG_DEV=1` 會讓模板每次請求重新讀取，改完存檔重新整理就看得到，不用重啟。
右下角的工具列可以切換字型方案（A/B/C/D）與明暗主題。

後台（`http://localhost:5173/admin/`）：

```bash
cd web/admin && npm install          # 第一次
npm run dev                          # 真的 Firebase 登入，/api 轉給 :8080 的 Go server
npm run dev:mock                     # 不登入、不打 API，用假資料看畫面
```

`dev` 需要 Go server 同時在 :8080 跑；Vite 會把 `/api` 轉過去，所以後端不用處理 CORS。
