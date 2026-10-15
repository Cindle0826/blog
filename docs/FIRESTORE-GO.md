# Firestore 在 Go 裡怎麼寫

對照 [`FIRESTORE.md`](FIRESTORE.md) 的資料結構，把每個會用到的操作寫成可以直接抄的形狀。

> 這裡的每一段程式碼都**實際編譯過**（`cloud.google.com/go/firestore v1.26.0`、Go 1.27.1，本專案 go.mod 用的版本），
> 不是憑印象寫的。API 形狀可以信。

---

## 安裝

```bash
go get cloud.google.com/go/firestore
go get google.golang.org/api/iterator
```

`google.golang.org/grpc/status` 與 `codes` 會被順帶拉進來，判斷 NotFound 要用。

## 初始化

```go
import "cloud.google.com/go/firestore"

client, err := firestore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
if err != nil {
    return nil, fmt.Errorf("firestore client: %w", err)
}
defer client.Close()
```

**不需要傳憑證。** 本機走 ADC（`03-local-dev-auth.sh` 設好的），Cloud Run 走附掛的
`blog-server` service account。同一份程式碼兩邊都跑得起來。

> 建立 client 會去 metadata server 拿 token，實測是冷啟動裡最肥的一段。
> 如果之後要優化冷啟動，把它搬到背景用 `sync.Once` 保護，讓 HTTP server 先開始聽。

---

## struct 與 tag

```go
type Post struct {
    // firestore:"-" 表示不寫進文件。document ID 存在 snapshot.Ref.ID，
    // 不是欄位，所以要在讀取後自己補上。
    ID string `firestore:"-"`

    Slug   string `firestore:"slug"`
    Kind   string `firestore:"kind"`     // "post" | "note"
    Status string `firestore:"status"`   // "draft" | "published"

    Title    string `firestore:"title"`
    Summary  string `firestore:"summary"`
    Markdown string `firestore:"markdown"`
    HTML     string `firestore:"html"`

    TOC        []TOCItem `firestore:"toc"`
    ReadingMin int       `firestore:"readingMin"`
    HasCode    bool      `firestore:"hasCode"`
    TagSlugs   []string  `firestore:"tagSlugs"`

    Cover  *Image `firestore:"cover"`   // 指標，沒封面就是 nil
    Pinned bool   `firestore:"pinned"`

    // 指標型別才能表達「草稿還沒有發佈時間」。用 time.Time 的話
    // 零值會被寫成 0001-01-01，查詢排序時混在一起很難處理。
    PublishedAt *time.Time `firestore:"publishedAt"`

    CreatedAt time.Time `firestore:"createdAt"`

    // serverTimestamp：寫入時**一律**用伺服器的時間，struct 裡填什麼都會被忽略
    // （SDK 的 to_value.go 看到這個 tag 就直接跳過該欄位）。所以寫入後你手上那份
    // struct 的這個欄位仍是舊值，要拿到真正的時間得再讀一次。
    // 好處是不依賴本機時鐘，而且多台實例寫入時間軸一致。
    // 注意它只在 Set / Add / Create 生效；Update 要顯式傳 firestore.ServerTimestamp。
    //
    // 不要拿 WriteResult.UpdateTime 當成這個欄位的值。ServerTimestamp 存的是伺服器
    // 「收到請求」的時間（精確到毫秒），UpdateTime 是「commit」的時間，實測差了約 20ms。
    UpdatedAt time.Time `firestore:"updatedAt,serverTimestamp"`
}

type TOCItem struct {
    Level  int    `firestore:"level"`
    Text   string `firestore:"text"`
    Anchor string `firestore:"anchor"`
}

// Image 會直接出現在 API 回應裡（postDetail.Cover），所以 json tag 也要寫。
// 沒寫的話 encoding/json 會輸出 "URL"、"Width" 這種大寫欄位名，跟契約對不上。
// （解析 request 時大小寫不敏感，所以只有輸出會出錯，很容易漏掉。）
type Image struct {
    URL    string `firestore:"url" json:"url"`
    Alt    string `firestore:"alt" json:"alt"`
    Width  int    `firestore:"width" json:"width"`
    Height int    `firestore:"height" json:"height"`
}
```

**欄位名一定要寫 tag。** 不寫的話 Firestore 會用 Go 的欄位名（`PublishedAt`），
跟 `FIRESTORE.md` 約定的 `publishedAt` 對不上，而且不會有任何錯誤 —— 你只會發現查詢永遠回空的。

---

## 讀取

### 用 document ID 取一篇

```go
snap, err := c.Collection("posts").Doc(id).Get(ctx)
if status.Code(err) == codes.NotFound {
    return nil, ErrNotFound      // 自己定義的哨兵錯誤
} else if err != nil {
    return nil, err
}

var p Post
if err := snap.DataTo(&p); err != nil {
    return nil, err
}
p.ID = snap.Ref.ID               // ID 不在文件裡，要自己補
return &p, nil
```

> **「找不到」不是 error，是一種正常結果。** Firestore 回的是 gRPC 的 NotFound，
> 用 `status.Code(err)` 判斷再轉成自己的哨兵錯誤，handler 才能乾淨地回 404 而不是 500。

### 用 slug 取一篇

slug 不是 document ID（見 `FIRESTORE.md` 的理由），所以要查詢：

```go
iter := c.Collection("posts").
    Where("slug", "==", slug).
    Limit(1).
    Documents(ctx)
defer iter.Stop()                // 忘了會漏連線

snap, err := iter.Next()
if errors.Is(err, iterator.Done) {
    return nil, ErrNotFound
} else if err != nil {
    return nil, err
}
```

注意這裡判斷的是 `iterator.Done` 而不是 `codes.NotFound` —— 查詢沒結果跟文件不存在，
在 Firestore 是兩種不同的訊號。

### 列表

```go
snaps, err := c.Collection("posts").
    Where("status", "==", "published").
    Where("kind", "==", kind).
    OrderBy("publishedAt", firestore.Desc).
    Limit(limit).
    Documents(ctx).GetAll()
```

`GetAll()` 一次拿完，適合有 `Limit` 的情況。沒有 limit 的大量資料要用 iterator 逐筆處理，
不然會一次全部進記憶體 —— Cloud Run 只有 512Mi。

### 置頂優先

```go
    Where("status", "==", "published").
    Where("kind", "==", kind).
    OrderBy("pinned", firestore.Desc).      // true 排在 false 前面
    OrderBy("publishedAt", firestore.Desc).
```

### 標籤頁

```go
    Where("status", "==", "published").
    Where("tagSlugs", "array-contains", tagSlug).
    OrderBy("publishedAt", firestore.Desc).
```

> 以上四種查詢**都需要複合索引**，已經由 Terraform 建好了（另外還有不帶 `kind` 的 `posts_published`）
> （`infrastruct_as_code/terraform/firestore.tf`）。如果你寫了新的欄位組合，
> Firestore 會在**執行期**回一個帶 Console 連結的錯誤——別在 Console 點一點就算了，
> 那樣建的索引不在 state 裡，下次 apply 不會知道它存在。寫進 `firestore.tf` 再 apply。

### 分頁

這個專案用**頁碼**：`Offset` 跳過前面幾頁，`Limit` 拿這一頁。

```go
    OrderBy("publishedAt", firestore.Desc).
    OrderBy(firestore.DocumentID, firestore.Desc).   // 同一時間發佈時，順序才固定
    Offset((page - 1) * perPage).                     // 第 3 頁、每頁 10 篇 → 跳過 20 篇
    Limit(perPage).
```

第二個 `OrderBy` 不能省。只照 `publishedAt` 排的話，兩篇同一時間發佈的文章每次查詢的
先後不保證一樣，換頁時可能一篇出現兩次、另一篇消失。方向要跟前一個 `OrderBy` 一致——
複合索引最後本來就隱含一個同方向的 document ID，所以 Terraform 建好的索引可以直接用。

總頁數要另外查總筆數，用下面「聚合查詢」那一節的 `WithCount`，數 100 篇只算 1 次讀取：

```go
totalPages := (total + perPage - 1) / perPage   // 無條件進位；total 為 0 時結果是 0
```

> **`Offset` 的代價**：被跳過的文件一樣會讀、一樣計費，第 N 頁要讀 N × perPage 筆。
> 這個 blog 文章數百篇以內，最後一頁也只讀幾百筆，公開頁面前面還有 CDN 快取，
> 這個代價可以忽略，換來的是可以直接跳到任何一頁。
>
> 資料量到幾萬筆以上、或是要做「往下滑自動載入更多」時，改用**游標分頁**：
> `StartAfter(上一頁最後一筆的 publishedAt, ID)` 直接從那個位置接著拿，前面的文件完全不讀。
> 代價是只能往下翻，不能跳頁，也不知道總共幾頁。

---

## 寫入

### 建立（自動產生 ID）

```go
ref := c.Collection("posts").NewDoc()   // 只在本機產生 ID，還沒寫入
p.ID = ref.ID
p.Slug = makeSlug(p.Title, ref.ID)      // 寫入前就能用到 ID
_, err := ref.Create(ctx, &p)           // 同 ID 已存在會回 codes.AlreadyExists
```

本專案用 `NewDoc` + `Create`，不用 `Add`：`Add` 要寫完才拿得到 ID，
但中文標題的 slug 是 `post-` + ID 前 8 碼，寫入前就要知道。

寫完要回傳完整物件的話，**再 `Get` 一次**，不要自己用 `WriteResult.UpdateTime` 補時間
（原因見上面 `UpdatedAt` 的註解）。

### 整份覆蓋

```go
_, err := c.Collection("posts").Doc(id).Set(ctx, p)
```

### 部分更新

```go
_, err := c.Collection("posts").Doc(id).Update(ctx, []firestore.Update{
    {Path: "markdown", Value: markdown},
    {Path: "html", Value: html},
    {Path: "updatedAt", Value: firestore.ServerTimestamp},
})
```

**`API.md` 的 `PATCH` 要用這個，不要用 `Set`。** `Set` 會把沒帶到的欄位清掉——
前端只送了 `markdown`，結果 `title`、`tagSlugs` 全部變空。

`Update` 的規則（v1.26.0 原始碼確認過，後兩條實測踩過）：

| 規則 | 細節 |
|---|---|
| 文件不存在 → `codes.NotFound` | 它預設就帶「文件必須存在」的前置條件，所以**不用先 Get** |
| **不要**再傳 `firestore.Exists` | 會直接回錯誤 `cannot use Exists with Update`（`options.go:92`） |
| `Path` 是 firestore tag 的名字 | 寫 `"tagSlugs"` 不是 `"TagSlugs"`。**拼錯不會報錯**，會在文件裡多出一個新欄位（實際發生過：`"updateAt"`） |
| 同一個 `Path` 只能出現一次 | 重複的話 SDK 送出前就回錯誤：`field path [updatedAt] cannot be used in the same update as [updatedAt]` |
| `Value: nil` → 存成 `null` | 用來表示「沒有封面」這種狀態（`to_value.go:66`） |
| nil slice → 存成 `null`，空 slice → 存成 `[]` | 整理陣列時用 `[]string{}` 起頭，不要用 `var s []string`（`to_value.go:200`） |
| 回傳值只有 `WriteResult` | 不含更新後的文件。要回傳完整物件就再 `Get` 一次 |

PATCH 的「沒送」要跟「送了零值」分開，所以要先把有送的欄位組成 slice，再一次 `Update`：

```go
ups := []firestore.Update{{Path: "updatedAt", Value: firestore.ServerTimestamp}}
if patch.Title != nil {
    ups = append(ups, firestore.Update{Path: "title", Value: *patch.Title})
}
// ...其他欄位
_, err := ref.Update(ctx, ups)
```

### 刪除

```go
_, err := c.Collection("posts").Doc(id).Delete(ctx)
```

刪不存在的文件**不會報錯**。要區分「刪掉了」和「本來就沒有」，加上 `firestore.Exists`
前置條件，文件不存在時會回 `codes.NotFound`，不用先 Get：

```go
_, err := c.Collection("posts").Doc(id).Delete(ctx, firestore.Exists)
if status.Code(err) == codes.NotFound {
    // 本來就沒有
}
```

---

## Transaction

> 本專案最後**沒有**用交易維護 `tags.count`，改用聚合查詢即時算（見下一節）。
> 這段留著當交易的寫法參考。

改 tag 的同時維護 `tags.count`，兩邊必須一起成功或一起失敗：

```go
err := c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
    postRef := c.Collection("posts").Doc(postID)

    snap, err := tx.Get(postRef)          // 交易內要用 tx.Get，不是 ref.Get
    if err != nil {
        return err
    }
    var p Post
    if err := snap.DataTo(&p); err != nil {
        return err
    }

    added, removed := diffTags(p.TagSlugs, newTags)

    for _, t := range added {
        if err := tx.Update(c.Collection("tags").Doc(t), []firestore.Update{
            {Path: "count", Value: firestore.Increment(1)},
        }); err != nil {
            return err
        }
    }
    for _, t := range removed {
        if err := tx.Update(c.Collection("tags").Doc(t), []firestore.Update{
            {Path: "count", Value: firestore.Increment(-1)},
        }); err != nil {
            return err
        }
    }

    return tx.Update(postRef, []firestore.Update{
        {Path: "tagSlugs", Value: newTags},
        {Path: "updatedAt", Value: firestore.ServerTimestamp},
    })
})
```

三條規則，違反了會出現很難查的 bug：

1. **所有讀取必須在所有寫入之前。** Firestore 的交易是先讀後寫，寫完再讀會直接報錯。
2. **交易函式可能被重跑好幾次**（遇到衝突時）。所以裡面不要有副作用——別在裡面寄信、
   寫 log 計數器、或改外部變數。
3. **用 `firestore.Increment` 而不是「讀出來 +1 再寫回去」。** Increment 是在伺服器端做的
   原子操作，不會因為兩個請求同時進來而少算。

---

## 一個可能讓你不用維護 count 的選項

`FIRESTORE.md` 原本的設計是用交易維護 `tags.count`。但 Firestore 有**聚合查詢**：

```go
q := c.Collection("posts").
    Where("status", "==", "published").
    Where("tagSlugs", "array-contains", tagSlug)

res, err := q.NewAggregationQuery().WithCount("n").Get(ctx)
if err != nil {
    return 0, err
}
n, _ := res.Data()["n"].(int64)
```

它**不會把文件撈回來**，只回一個數字。計費是按掃過的索引項目算（每 1000 筆算一次讀取），
所以數 100 篇文章只花 1 次讀取，而不是 100 次。分頁的總頁數也是用它算。

| | 交易維護 count | 聚合查詢 |
|---|---|---|
| 讀取成本 | 幾乎 0（直接讀 tags 文件） | 每次查 1 次讀取 |
| 寫入複雜度 | 每次改 tag 都要跑交易 | **不用維護** |
| 一致性 | 交易失敗或有 bug 就會飄掉 | **永遠正確** |
| 即時性 | 即時 | 即時 |

**你這個規模（< 50 個標籤、< 500 篇文章），我覺得聚合查詢比較划算** —— 少一個要維護一致性的
地方，而且 count 不可能算錯。交易那套的價值要在「每秒好幾次寫入」時才顯現。

**2026-10-07 決定採用聚合查詢。** `tags` 文件不存 `count`，寫入文章的 API 都不用處理它。

---

## 計費：三件會讓你意外的事

**1. `Select()` 省流量，不省讀取次數。**

```go
.Select("slug", "title")   // 網路傳輸變小，但還是算一次完整讀取
```

Firestore 按「文件」計費，不按欄位。`Select` 的價值是省頻寬和記憶體，不是省錢。

**2. 查詢的成本是「回傳的文件數」。**

撈 10 篇 = 10 次讀取。所以列表頁一定要有 `Limit`。
**沒有 `Limit` 的查詢是這個專案最可能燒掉免費額度的東西。**

**3. 空結果也要錢。**

查不到東西一樣計費（算 1 次）。所以「先查有沒有再決定要不要建立」這種模式，
每次都會付兩次的錢。

> 免費額度是每天 50,000 次讀取。加上記憶體快取和 CDN 之後，實際用量大概每天幾十次 ——
> 上面三點在正常情況下都碰不到，但寫錯一個查詢就可能在幾分鐘內燒光。

---

## 本機測試

`internal/store` 的測試跑在 **Firestore 模擬器**上，不碰正式資料庫：

```bash
./scripts/test-store.sh             # 啟動模擬器 → 跑測試 → 關掉模擬器
./scripts/test-store.sh -run Slug   # 其他參數原樣傳給 go test
```

需要兩樣東西（2026-10-15 已裝好）：

| | 安裝方式 | 為什麼 |
|---|---|---|
| 模擬器 | `gcloud components install cloud-firestore-emulator` | gcloud 的元件，不用另外裝 firebase CLI |
| Java 25 | `sdk install java 25.0.4.1+1-tem`（問要不要設成預設時回答 n） | 模擬器要 Java 21 以上，gcloud 577 已經在警告 21 會停止支援。腳本只在自己執行時用 25，預設的 Java 17 不受影響 |

**沒有模擬器時，這些測試會自動略過**，`go test ./...` 一樣會通過。
判斷依據是 `FIRESTORE_EMULATOR_HOST` 有沒有設：Firestore client 看到它就會改連模擬器，
程式碼一行都不用改；沒設的話測試直接略過，不可能連到正式環境。專案 ID 也用模擬器專用的 `demo-` 前綴。

每個測試開始前會清空模擬器（`DELETE /emulator/v1/projects/{id}/databases/(default)/documents`，
模擬器專用的 API），所以測試之間互不影響，資料存在記憶體，關掉就沒了。

測不到的東西：**模擬器不檢查複合索引**。正式環境缺索引會失敗的查詢，在模擬器上照樣成功，
索引要靠 `firestore.tf` 管理。

另外，`ServerTimestamp` 只精確到毫秒，模擬器又很快，「寫兩次、比較 updatedAt 有沒有前進」
的測試要在兩次寫入之間等幾毫秒（測試裡的 `nextMillisecond()`），不然偶爾會失敗。

`internal/handler` 的測試不需要模擬器：它用一個假的 store（實作 handler 自己定義的
`postStore` interface），記下 handler 傳了什麼、指定 store 要回什麼。

---

## 常見陷阱

| 症狀 | 原因 |
|---|---|
| 查詢永遠回空，但沒有錯誤 | struct tag 漏寫或拼錯，欄位名對不上 |
| `DataTo` 後欄位都是零值 | 同上，或用了 `Select` 沒選到那些欄位 |
| 查詢回錯誤，訊息裡有個 Console 連結 | 缺複合索引——寫進 `firestore.tf` 再 apply |
| 交易偶爾莫名其妙失敗 | 讀寫順序反了，或交易函式裡有副作用 |
| `PATCH` 之後其他欄位不見了 | 用了 `Set` 而不是 `Update` |
| `Update` 回成功，但欄位沒變、文件多了一個怪欄位 | `Path` 拼錯，Firestore 當成新欄位寫入 |
| `Update` 一定失敗：`cannot be used in the same update` | 同一個 `Path` 加了兩次 |
| 回應的 `updatedAt` 跟之後 GET 的差幾十毫秒 | 用 `WriteResult.UpdateTime` 填回應，應該寫完再 `Get` |
| 陣列欄位讀回來是 `null` | 寫入時是 nil slice |
| 同一篇文章在相鄰兩頁都出現 | 少了 `OrderBy(firestore.DocumentID, ...)` 當第二排序鍵 |
| 分頁越後面越慢越貴 | `Offset` 的正常代價，這個規模可以忽略；資料量真的很大時改用 `StartAfter` |
| 本機正常、線上 permission denied | Cloud Run 的 `blog-server` 只有 `roles/datastore.user`，不能改索引或資料庫本身 |
