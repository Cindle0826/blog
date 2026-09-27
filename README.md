# blog

個人部落格與知識庫。Go 後端直接伺服 server-rendered HTML，前面掛 Firebase Hosting 的 CDN。

## 為什麼是這個架構

SEO 是第一優先，所以公開頁面必須是**伺服器渲染的完整 HTML** —— 爬蟲和
Facebook / LINE / Slack 的預覽機器人都不執行 JavaScript，SPA 在它們眼中是空白頁。

Go 的 `html/template` 就是最單純的那種伺服器渲染：塞資料進模板、吐字串、回傳。
沒有 hydration、沒有 build step，跟 Next.js 那套 SSR 完全是兩回事。

後台是另一回事 —— 它不需要被搜尋，所以用 React SPA，該用什麼用什麼。

```
訪客 ─→ Firebase Hosting (CDN) ─→ Cloud Run (Go) ─→ Firestore
              │                      scale-to-zero
              └─ /admin/* 靜態 React SPA
```

## 專案結構

```
cmd/server/       正式站進入點
cmd/preview/      假資料預覽伺服器（開發用，不會進 image）
internal/
  config/         設定
  store/          Firestore 存取
  auth/           Firebase ID token 驗證
  markdown/       goldmark + chroma + bluemonday
  cache/          記憶體快取與 CDN 失效
  handler/        HTTP handler
  view/           渲染層：ViewModel、模板載入、SEO 結構化資料
web/
  templates/      .gohtml
  static/         css / js / img / fonts
  admin/          React + TypeScript 後台
deploy/           Dockerfile、firebase.json、cloudbuild.yaml
docs/             計畫與前後端契約
```

## 本機開發

前端（模板與 CSS）不需要 GCP 憑證，直接跑預覽伺服器：

```bash
BLOG_DEV=1 go run ./cmd/preview
```

開 <http://localhost:8080>。`BLOG_DEV=1` 讓模板每次請求重新讀取，
改完存檔重新整理就看得到。右下角工具列可以切換字型方案與主題。

正式站（需要 Firestore 憑證）：

```bash
gcloud auth application-default login
go run ./cmd/server
```

## 文件

| 文件 | 內容 |
|---|---|
| [docs/PLAN.md](docs/PLAN.md) | 開發計畫、責任分界、目前進度 |
| [docs/API.md](docs/API.md) | 後台 REST API 契約 |
| [docs/FIRESTORE.md](docs/FIRESTORE.md) | Firestore 資料結構契約 |
| [docs/FIRESTORE-GO.md](docs/FIRESTORE-GO.md) | Firestore 在 Go 裡的寫法（可直接抄） |
| [http_tests/api_test.http](http_tests/api_test.http) | 契約② 的可執行版本，43 個測試 |
| [docs/LOCAL-AUTH.md](docs/LOCAL-AUTH.md) | 本機怎麼用 Auth 模擬器測登入 |

## 成本

目標是每月 $0，靠的是各服務的免費額度：Cloud Run 2M requests、
Firestore 50k reads/day、Firebase Hosting 10GB/月。
CDN 擋在前面之後，實際打到 Cloud Run 與 Firestore 的請求會少一到兩個數量級。

需要 Blaze 方案（要綁卡），**務必設定 Budget Alert**。
