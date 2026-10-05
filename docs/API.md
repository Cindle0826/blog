# 契約② — 後台 REST API

> 這份文件定義後台（React SPA）跟後端（Go）怎麼溝通。
> 前端會完全照這份寫，所以你實作時**回應格式要一字不差**。
>
> **這份契約有可執行版本：[`../http_tests/api_test.http`](../http_tests/api_test.http)**
> （47 個測試 / 70 條斷言）。實作一個 handler 就跑一次，它會直接告訴你形狀對不對。

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

| HTTP | `code`            | 時機                         |
|------|-------------------|------------------------------|
| 400  | `invalid_request` | 欄位缺漏或格式錯             |
| 401  | `unauthorized`    | 沒帶 token、token 過期或無效 |
| 403  | `forbidden`       | token 有效但 UID 不在白名單  |
| 404  | `not_found`       | 文章不存在                   |
| 409  | `slug_conflict`   | slug 已被使用                |
| 500  | `internal`        | 其他。**訊息不要吐內部細節** |

---

## 文章

### `GET /api/posts`

列出文章（含草稿）。後台列表頁用。

查詢參數：

| 參數 | 型別 | 預設 | 說明 |
|---|---|---|---|
| `status` | `draft\|published\|all` | `all` | 其他值 → `400 invalid_request` |
| `kind` | `post\|note\|all` | `all` | 其他值 → `400 invalid_request` |
| `search` | string | — | 搜尋標題，見下方說明 |
| `page` | int | 1 | 從 1 開始 |
| `limit` | int | 50 | 每頁幾筆。超過 100 一律當成 100，不報錯 |

所有參數都是選填。前端在值等於預設時**不會送**（例如 `status=all`、`page=1`），
所以後端收到空字串要當成預設值。

`search` 的比對規則：前後空白先去掉，去掉後是空的就當沒帶；不分大小寫，
標題裡任何位置出現都算（`Cloud` 會找到「把 Cloud Run 冷啟動…」）。

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
  "page":       1,     // 目前頁碼，跟請求的 page 相同
  "totalPages": 3,     // 沒有任何符合的文章時為 0
  "total":      127    // 符合條件的總筆數（套用 status / kind / search 之後）
}
```

- `page` 超出範圍（只有 3 頁卻要第 5 頁）→ `200`，`items` 是**空陣列 `[]`**，不是 404，
  也不是 `null`。「這頁沒東西」是正常結果。
- `page` 或 `limit` 不是正整數 → `400 invalid_request`
- 有 `search` 時：Firestore 沒有子字串查詢，後端要先撈出符合 `status`、`kind` 的**全部**文章，
  在 Go 裡過濾標題，**過濾完再切頁**。`total` 以過濾後的數量為準。
  先切頁再過濾的話，每頁筆數會忽多忽少，`total` 也會是錯的。

> 用頁碼而不是游標：只有一個使用者、文章數百篇以內，`Offset` 多讀的那些文件可以忽略，
> 換來的是可以直接跳頁。取捨見 [`FIRESTORE-GO.md`](FIRESTORE-GO.md) 的「分頁」。

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
4. 存到 GCS，物件路徑 `{yyyy}/{mm}/{uuid}.{ext}`
5. 回傳**站台上的路徑**與尺寸

```jsonc
{
  "path":   "/images/2026/09/abc.webp",
  "width":  1600,
  "height": 900,
  "bytes":  184320
}
```

> ⚠️ 回傳的是 `path`，**不是 GCS 的絕對網址**。
>
> bucket 是私有的，物件從網際網路上根本拿不到。圖片一律經由
> `GET /images/*` 提供（見下一節）。
>
> 這個決定的理由：
> - **SEO** — Google 圖片是按託管網域歸屬的。用 `storage.googleapis.com`
>   的話，圖片權重全部累積到 Google 的網域上，不是你的
> - **成本** — GCS 對外流量逐 GB 計費；走 Firebase Hosting 則吃它的
>   免費流量額度，而且 CDN 會擋掉絕大多數回源
> - **可換性** — 網址會被寫進文章的 Markdown 裡。存 GCS 絕對網址等於把
>   儲存供應商寫死在內容裡，之後想換就得改每一篇文章

> 一定要回傳 `width`/`height`——前端要把它寫進 Markdown，
> 這樣渲染出來的 `<img>` 才有尺寸，不會造成版面位移。

Markdown 裡長這樣：

```markdown
![冷啟動分解圖](/images/2026/09/abc.webp)
```

---

### `GET /images/{path...}`

**這不是後台 API，是公開路由**，但跟上傳是同一個功能的兩半，所以寫在一起。

把 GCS 的物件串流回去。不需要認證。

|          |                                                      |
|----------|------------------------------------------------------|
| 物件位置 | `gs://{BLOG_UPLOADS_BUCKET}/{path}`                  |
| 認證     | 無（公開）                                           |
| 快取     | `Cache-Control: public, max-age=31536000, immutable` |

實作要點：

1. **`path` 必須驗證。** 只允許 `[a-zA-Z0-9/._-]`，而且**拒絕任何含 `..` 的路徑**。
   雖然 GCS 的物件名稱是扁平的（`..` 不會真的跳出目錄），但別依賴這點——
   這種檢查是寫給未來那個把儲存換成本機檔案系統的人看的。

2. **用 `io.Copy` 串流，不要先讀進記憶體。**
   Cloud Run 只配 512Mi，幾個人同時抓大圖就爆了。

3. **`immutable` 可以放心設。** 檔名帶 UUID，內容永遠不會變。
   這讓 CDN 和瀏覽器都能永久快取，每張圖每個節點只回源一次。

4. **轉發 `Content-Type` 與 `Content-Length`**，並處理
   `If-None-Match` / `ETag` 回 304。

5. **物件不存在要回 404，不要回 500。**

大致的形狀（你的範圍，但這是我提議的改動，規格給完整）：

```go
func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("path")
    if !safeObjectPath(name) {
        http.NotFound(w, r)
        return
    }

    obj := h.bucket.Object(name)
    attrs, err := obj.Attrs(r.Context())
    if errors.Is(err, storage.ErrObjectNotExist) {
        http.NotFound(w, r)
        return
    } else if err != nil {
        h.serverError(w, r, err)
        return
    }

    // 內容不會變，所以 ETag 比對命中就直接 304，連 GCS 都不用讀
    if match := r.Header.Get("If-None-Match"); match != "" && match == attrs.Etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }

    w.Header().Set("Content-Type", attrs.ContentType)
    w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
    w.Header().Set("ETag", attrs.Etag)
    w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

    rc, err := obj.NewReader(r.Context())
    if err != nil {
        h.serverError(w, r, err)
        return
    }
    defer rc.Close()

    io.Copy(w, rc)   // 串流，不要 io.ReadAll
}
```

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
4. **不需要 CORS**：正式環境後台與 API 同網域；本機開發時 Vite（`:5173`）
   會把 `/api` 與 `/images` 轉給 Go server（`:8080`），瀏覽器看到的也是同源。
   （2026-10-04 修改：原本要求本機開 CORS，改用 Vite proxy 後拿掉）
5. **錯誤一律用上面的統一格式**，前端有一個共用的錯誤處理函式會解析它
