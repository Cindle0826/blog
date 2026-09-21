# 契約② — 後台 REST API

> 這份文件定義後台（React SPA）跟後端（Go）怎麼溝通。
> 前端會完全照這份寫，所以你實作時**回應格式要一字不差**。

- Base path：`/api`
- 格式：JSON（`Content-Type: application/json`）
- 認證：**每一個 `/api/*` 端點都需要**，公開站的頁面則完全不經過這裡

---

## 認證

前端用 Firebase Auth 的 Google 登入拿到 ID token，放在標頭送出：

```
Authorization: Bearer <firebase-id-token>
```

後端 middleware 要做三件事：

1. 用 Firebase Admin SDK 驗證 token 簽章與有效期
2. **比對 UID 是否在白名單內**（環境變數 `ADMIN_UIDS`，逗號分隔）
3. 不符合 → `403`

> ⚠️ 第 2 步不能省。只驗 token 的話，**任何有 Google 帳號的人**都能過關——
> Firebase Auth 驗證的是「這個人是誰」，不是「這個人有沒有權限」。

### 錯誤回應（全站統一格式）

```jsonc
{
  "error": {
    "code":    "unauthorized",       // 機器判讀用
    "message": "請重新登入"           // 直接顯示給使用者
  }
}
```

| HTTP | `code` | 時機 |
|---|---|---|
| 400 | `invalid_request` | 欄位缺漏或格式錯 |
| 401 | `unauthorized` | 沒帶 token、token 過期或無效 |
| 403 | `forbidden` | token 有效但 UID 不在白名單 |
| 404 | `not_found` | 文章不存在 |
| 409 | `slug_conflict` | slug 已被使用 |
| 500 | `internal` | 其他。**訊息不要吐內部細節** |

---

## 文章

### `GET /api/posts`

列出文章（含草稿）。後台列表頁用。

查詢參數：

| 參數 | 型別 | 預設 | 說明 |
|---|---|---|---|
| `status` | `draft\|published\|all` | `all` | |
| `kind` | `post\|note\|all` | `all` | |
| `q` | string | — | 標題模糊比對 |
| `limit` | int | 50 | 上限 100 |
| `cursor` | string | — | 上一頁回傳的 `nextCursor` |

回應：

```jsonc
{
  "items": [
    {
      "id":          "aBc123XyZ",
      "slug":        "cloud-run-cold-start",
      "kind":        "post",
      "status":      "published",
      "title":       "把 Cloud Run 冷啟動從 1.8s 壓到 240ms",
      "summary":     "scale-to-zero 很香，但……",
      "tagSlugs":    ["gcp", "go"],
      "pinned":      true,
      "publishedAt": "2026-09-18T02:00:00Z",   // draft 為 null
      "updatedAt":   "2026-09-20T09:10:00Z"
    }
  ],
  "nextCursor": "eyJ..."   // 沒有下一頁時為 null
}
```

> 列表**不回傳** `markdown` 與 `html`。50 篇文章的全文會讓回應變成好幾 MB。

### `GET /api/posts/{id}`

單篇完整資料，編輯頁用。比列表多了：

```jsonc
{
  "markdown": "## 問題從哪來\n\n...",
  "cover":    { "url": "...", "alt": "...", "width": 1600, "height": 900 },
  "createdAt":"2026-09-17T14:22:00Z"
}
```

不回傳 `html`（前端自己即時預覽，不需要後端那份）。

### `POST /api/posts`

建立。必填 `title`；其他可省略。

```jsonc
{
  "title":    "新文章",
  "kind":     "post",          // 預設 "post"
  "markdown": "",
  "slug":     "",              // 留空由後端從標題產生
  "tagSlugs": [],
  "summary":  "",
  "cover":    null,
  "pinned":   false
}
```

→ `201`，回傳與 `GET /api/posts/{id}` 相同的完整物件。
**新建立的文章一律是 `draft`**，不能直接建成已發布。

### `PATCH /api/posts/{id}`

部分更新。**只送有改動的欄位**，沒送的欄位維持原狀。

```jsonc
{ "markdown": "改過的內容", "tagSlugs": ["go"] }
```

後端在這裡要做的事：
1. 重新跑 goldmark + bluemonday → 更新 `html`
2. 重算 `toc`、`readingMin`、`hasCode`
3. 更新 `updatedAt`
4. `tagSlugs` 有變 → transaction 更新 `tags.count`
5. `slug` 有變 → 寫一筆 `redirects`
6. **失效快取**（見下）

→ `200`，回傳完整物件。

### `DELETE /api/posts/{id}`

→ `204`，無內容。

真刪除，不做軟刪除——這是個人 blog，不需要回收桶。
（後台 UI 會有二次確認。）

### `POST /api/posts/{id}/publish`

```jsonc
{ "publish": true }    // false 代表退回草稿
```

`true` 時：`status → published`，若 `publishedAt` 為 null 則設為現在。
`false` 時：`status → draft`，`publishedAt` **保持不變**（重新發布時日期才不會跳掉）。

→ `200`，回傳完整物件。

---

## 標籤

### `GET /api/tags`

```jsonc
{ "items": [ { "slug": "go", "name": "Go", "count": 12 } ] }
```

### `PUT /api/tags/{slug}`

```jsonc
{ "name": "Go" }
```

只能改顯示名稱。slug 是 document ID，不可改。

---

## 圖片上傳

### `POST /api/uploads`

`multipart/form-data`，欄位名 `file`。

後端要做：
1. 檢查 MIME（只收 `image/jpeg`、`image/png`、`image/webp`、`image/gif`）
2. 檢查大小上限 10 MB
3. **長邊超過 1600px 就縮圖**（用 `golang.org/x/image/draw`，純 Go 不用 cgo）
4. 存到 Firebase Storage，路徑 `uploads/{yyyy}/{mm}/{uuid}.{ext}`
5. 回傳網址與尺寸

```jsonc
{
  "url":    "https://storage.googleapis.com/…/uploads/2026/09/abc.webp",
  "width":  1600,
  "height": 900,
  "bytes":  184320
}
```

> 一定要回傳 `width`/`height`——前端要把它寫進 Markdown，
> 這樣渲染出來的 `<img>` 才有尺寸，不會造成版面位移。

---

## 快取失效

### `POST /api/cache/purge`

```jsonc
{ "paths": ["/", "/posts", "/posts/cloud-run-cold-start"] }
```

任何會改變公開頁面的操作（發布、修改、刪除）之後都要呼叫。
不呼叫的話，CDN 會繼續送舊內容最長 1 小時。

後端實作：清掉記憶體快取，並對 Firebase Hosting 發 purge 請求。

---

## 給前端的備註

以下是我（前端）會依賴的行為，請務必維持：

1. **所有時間都是 UTC 的 RFC3339 字串**，不要回 Unix timestamp，也不要回本地時間
2. **`null` 和「欄位不存在」意義不同**——`publishedAt: null` 表示草稿，欄位缺漏表示 bug
3. **`PATCH` 一定要回傳更新後的完整物件**，前端才不用再發一次 GET
4. **CORS**：正式環境同源不需要處理；本機開發時前端跑在 `localhost:5173`，
   要允許該來源並且 `Access-Control-Allow-Credentials: true`
5. **錯誤一律用上面的統一格式**，前端有一個共用的錯誤處理函式會解析它
