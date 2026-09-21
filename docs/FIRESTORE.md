# 契約③ — Firestore 資料結構

> 這份文件定義後端存什麼。改欄位之前先確認 `internal/view/model.go` 要不要跟著改。

Firestore 用 **Native mode**（不是 Datastore mode）。

---

## Collection 總覽

| Collection | 用途 | 預估筆數 |
|---|---|---|
| `posts` | 文章與知識庫筆記（同一個 collection，靠 `kind` 區分） | < 500 |
| `tags` | 標籤與計數 | < 50 |
| `meta` | 單例設定（站台資訊、about 內文） | 3 |

**為什麼文章和筆記放同一個 collection**：兩者的欄位完全一樣，查詢也一樣。
分成兩個 collection 只會讓「最新的 N 篇（不分類型）」這種查詢變得很痛苦。

---

## `posts/{postId}`

`postId` 用 Firestore 自動產生的 ID，**不要**用 slug 當 document ID
（slug 可能要改，document ID 不能改）。

```jsonc
{
  // ── 識別 ──────────────────────────────────────────
  "slug":        "cloud-run-cold-start",   // URL 用，全站唯一，見下方「slug 規則」
  "kind":        "post",                   // "post" | "note"
  "status":      "published",              // "draft" | "published"

  // ── 內容 ──────────────────────────────────────────
  "title":       "把 Cloud Run 冷啟動從 1.8s 壓到 240ms",
  "summary":     "scale-to-zero 很香，但……",  // 留空時後端自動從內文前 120 字產生
  "markdown":    "## 問題從哪來\n\n...",      // 原始 Markdown，唯一的真相來源
  "html":        "<h2 id=\"...\">...",       // goldmark 轉換＋bluemonday 清洗後的結果

  // ── 為什麼要存 html？ ─────────────────────────────
  // 因為 Markdown→HTML 每次都轉會浪費 CPU，而 Cloud Run 是按 CPU 秒計費的。
  // 存檔時轉一次，之後直接讀。代價是改了渲染規則要重跑一次全部文章
  // （scripts/ 會放一支 re-render 的小工具）。

  // ── 衍生資料（存檔時算好，不要在讀取時算）────────────
  "toc": [
    { "level": 2, "text": "問題從哪來", "anchor": "問題從哪來" }
  ],
  "readingMin":  11,        // 中文 ÷350、英文 ÷220 字/分，無條件進位
  "hasCode":     true,      // 內文有 <pre> 時為 true

  // ── 分類 ──────────────────────────────────────────
  "tagSlugs":    ["gcp", "go", "performance"],   // 只存 slug，顯示名稱去 tags 查

  // ── 封面圖 ────────────────────────────────────────
  "cover": {
    "url":    "https://storage.googleapis.com/…/cover.webp",
    "alt":    "冷啟動時間分解圖",
    "width":  1600,
    "height": 900
  },

  // ── 旗標與時間 ────────────────────────────────────
  "pinned":      true,
  "publishedAt": "2026-09-18T02:00:00Z",   // Firestore Timestamp；draft 時為 null
  "createdAt":   "2026-09-17T14:22:00Z",
  "updatedAt":   "2026-09-20T09:10:00Z"    // 只有「發布後又修改」才更新，見下
}
```

### 欄位規則

| 欄位 | 規則 |
|---|---|
| `slug` | 小寫英數與連字號。中文標題無法音譯時，退回 `post-{短ID}` |
| `status` | `draft` 的文章**不得**出現在任何公開查詢、sitemap、feed |
| `publishedAt` | 第一次 `draft → published` 時才設定，之後不再變動 |
| `updatedAt` | 每次存檔都更新，但**只有在已發布後才顯示給讀者** |
| `html` | 必須是清洗過的。這是 XSS 的唯一防線 |
| `cover.width/height` | 必填。少了它瀏覽器無法預留空間，會造成版面位移（扣 CLS 分數） |

### slug 規則

1. 由標題產生：轉小寫、空白換連字號、去掉非英數字元
2. 純中文標題 → 後端無法音譯 → 用 `post-` + document ID 前 8 碼
3. 存檔前查 `where slug == ?`，撞到就在後面加 `-2`、`-3`
4. **slug 改掉時必須在 `redirects` 留紀錄**（見下），否則舊連結全部 404，SEO 分數會掉

---

## `tags/{slug}`

document ID 直接用 slug，方便直接 `Get`。

```jsonc
{
  "name":  "Kubernetes",   // 顯示名稱，大小寫由你決定
  "count": 8,              // 已發布文章數（不含草稿）
  "order": 0               // 手動排序用，0 表示依 count 排
}
```

`count` 在文章發布／取消發布／改 tag 時用 transaction 更新。
**不要**每次列表頁都去 count 一次——那是 N 次讀取，免費額度會燒很快。

---

## `meta/{docId}`

三個固定的 document：

| docId | 內容 |
|---|---|
| `site` | 站名、標語、導覽列、社群連結 — 對應 `view.Site` |
| `about` | 關於頁的 `markdown` 與 `html` |
| `stats` | `postCount`、`lastPublishedAt`，給 sitemap 的 lastmod 用 |

---

## `redirects/{oldSlug}`

```jsonc
{ "to": "new-slug", "createdAt": "2026-09-20T09:10:00Z" }
```

改 slug 時寫一筆，handler 查到就回 **301**（不是 302）。
301 會讓搜尋引擎把舊網址的權重轉移到新網址，302 不會。

---

## 索引

Firestore 單欄位索引是自動的，但複合查詢要手動建。需要的有：

| 查詢 | 索引 |
|---|---|
| 首頁 / 文章列表 | `status ASC, kind ASC, publishedAt DESC` |
| 標籤頁 | `status ASC, tagSlugs ARRAY, publishedAt DESC` |
| 置頂優先 | `status ASC, kind ASC, pinned DESC, publishedAt DESC` |

建索引的指令會在第一次跑到該查詢時，由 Firestore 的錯誤訊息直接給你
（錯誤訊息裡有一個可以點的網址）。建好之後匯出成 `deploy/firestore.indexes.json` 進版控。

---

## 讀取次數預算

免費額度是 **50,000 reads/day**。粗估：

| 動作 | 讀取次數 |
|---|---|
| 首頁（5 文章 + 3 筆記 + 6 標籤 + site） | ~15 |
| 文章頁（1 文章 + 3 相關 + site） | ~5 |
| 列表頁（10 篇 + site） | ~11 |

即使完全不快取，一天也要有 **3,000+ PV** 才會碰到上限。
加上記憶體快取與 CDN 之後，實際讀取次數大概會落在每天 **幾十次**。

不過還是要注意 [Firestore 三個陷阱](#) — 沒有 `limit()` 的查詢、
在迴圈裡逐筆 `Get`、以及忘記關掉的 realtime listener。
