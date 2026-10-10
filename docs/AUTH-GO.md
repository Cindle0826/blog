# Firebase Auth 在 Go 裡怎麼寫

寫 `internal/auth` 的 middleware 時看這份。測試怎麼跑看 [`LOCAL-AUTH.md`](LOCAL-AUTH.md)。

> 每一段程式碼都**實際編譯與 vet 過**（`firebase.google.com/go/v4 v4.22.0`）。

---

## 安裝

```bash
go get firebase.google.com/go/v4
```

Admin SDK。`VerifyIDToken` 在 `firebase.google.com/go/v4/auth`。

## 初始化

```go
import (
    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/auth"
)

app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID})
if err != nil {
    return nil, fmt.Errorf("firebase app: %w", err)
}

client, err := app.Auth(ctx)
if err != nil {
    return nil, fmt.Errorf("firebase auth client: %w", err)
}
```

**不需要憑證檔。** `ProjectID` 必填 —— 驗證 token 時要拿它比對 `aud` 宣告，
沒給的話 SDK 會回 `project id not available`。

> `firebase.NewApp` 的第二個參數是 `*firebase.Config`。很多網路上的範例寫
> `firebase.NewApp(ctx, nil, option.WithCredentialsFile(...))` —— 那是在指定
> service account 金鑰檔，**我們不用那條路**（見 `infrastruct_as_code/README.md`）。

---

## 驗證一個 token

```go
tok, err := client.VerifyIDToken(ctx, rawToken)
```

回傳的 `*auth.Token`：

```go
type Token struct {
    AuthTime int64
    Issuer   string                  // https://securetoken.google.com/cindle-blog
    Audience string                  // cindle-blog
    Expires  int64
    IssuedAt int64
    Subject  string                  // 跟 UID 一樣
    UID      string                  // ← 你要比對白名單的就是這個
    Firebase FirebaseInfo            // .SignInProvider = "google.com"
    Claims   map[string]interface{}  // email 等其他宣告在這裡
}
```

**這是離線驗證。** SDK 把 Google 的公鑰快取起來（約 5.5 小時更新一次），
每個請求只做本機的簽章運算 —— 微秒級，不是一次 API 往返。Google 掛掉你的站也還能驗 token。

---

## Middleware

```go
type Authenticator struct {
    client *auth.Client
    admins map[string]bool
}

type ctxKey int
const uidKey ctxKey = iota

func UIDFrom(ctx context.Context) (string, bool) {
    uid, ok := ctx.Value(uidKey).(string)
    return uid, ok
}

func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ① 取出 token
        raw, ok := bearerToken(r)
        if !ok {
            writeError(w, http.StatusUnauthorized, "unauthorized", "請先登入")
            return
        }

        // ② 你是誰 —— 驗簽章、發行者、對象、有效期
        tok, err := a.client.VerifyIDToken(r.Context(), raw)
        if err != nil {
            switch {
            case auth.IsIDTokenExpired(err):
                writeError(w, http.StatusUnauthorized, "unauthorized", "登入已過期，請重新登入")
            case auth.IsIDTokenInvalid(err):
                writeError(w, http.StatusUnauthorized, "unauthorized", "登入憑證無效")
            case auth.IsCertificateFetchFailed(err):
                // 這個不是使用者的錯——是我們連不到 Google 拿公鑰
                log.Printf("auth: 取得 Google 公鑰失敗: %v", err)
                writeError(w, http.StatusInternalServerError, "internal", "服務暫時無法使用")
            default:
                log.Printf("auth: 驗證失敗: %v", err)
                writeError(w, http.StatusUnauthorized, "unauthorized", "登入憑證無效")
            }
            return
        }

        // ③ 你能做什麼 —— 這一步不能省
        if !a.admins[tok.UID] {
            log.Printf("auth: 拒絕非白名單 UID %s（provider=%s）",
                tok.UID, tok.Firebase.SignInProvider)
            writeError(w, http.StatusForbidden, "forbidden", "沒有權限")
            return
        }

        ctx := context.WithValue(r.Context(), uidKey, tok.UID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

掛上去：

```go
mux.Handle("GET /api/posts", a.RequireAdmin(http.HandlerFunc(h.List)))
```

### 取出 Bearer token

```go
func bearerToken(r *http.Request) (string, bool) {
    h := r.Header.Get("Authorization")
    if h == "" {
        return "", false
    }
    const prefix = "Bearer "
    // EqualFold：RFC 7235 規定 scheme 不分大小寫，有些客戶端會送 "bearer"
    if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
        return "", false
    }
    tok := strings.TrimSpace(h[len(prefix):])
    return tok, tok != ""
}
```

---

## 為什麼 ② 和 ③ 是兩件事

這是整個後台安全的核心，值得再說一次：

```
② VerifyIDToken 成功  =  「這個請求確實來自 UID xxx」
③ 白名單比對成功      =  「UID xxx 可以操作後台」
```

**只做 ② 的話，全世界任何 Google 帳號都能發文。** Firebase Auth 是身分提供者，
不是門禁系統 —— 它很樂意幫陌生人建立帳號並發給他一個簽章完全正確的 token。

`api_test.http` 的第 2 則就是在測這件事。它如果回 200 而不是 403，代表 ③ 沒生效。

---

## 要不要拆成兩個 middleware

直覺會想拆成 `Authenticate`（驗身分）+ `RequireAdmin`（查權限）。**不要拆。**

拆開之後這行是合法的 Go 程式碼：

```go
mux.Handle("POST /api/posts", auth.Authenticate(handler))
//                            ↑ 少掛了 RequireAdmin
```

編譯過、跑得動、測試可能也過——但全世界任何 Google 帳號都能發文。

合成一個就不可能犯這個錯：**沒有「只驗身分不查權限」這個東西可以掛。**
而拆開的前提是「存在只需登入、不需管理員權限的路由」，這個專案不會有。

可讀性用函式切割解決，不是用 middleware 切割：

```go
raw, ok := bearerToken(r)                  // 可單獨測
tok, err := a.client.VerifyIDToken(...)
if err != nil {
    a.writeVerifyError(w, err)             // 錯誤三分類抽出來
    return
}
if !a.admins[tok.UID] { ... }
```

### 該分開的是橫切關注點

```go
Recover(next)      // panic → 500
RequestLog(next)   // 方法、路徑、狀態碼、耗時
CORS(next)         // 只在開發模式掛
```

差別在於**它們套用在所有路由上**，包含公開的文章頁，不是只有 `/api/*`。

```go
var chain http.Handler = mux
chain = h.RequestLog(chain)
chain = h.Recover(chain)        // 最外層——不然 log middleware 自己 panic 沒人接
if cfg.Dev {
    chain = h.CORS(chain)
}
```

**更新（2026-10-04）：這個專案不需要 CORS。** 後台的 Vite dev server 會把 `/api`
轉給 Go server，本機開發也是同源，所以上面的 `CORS(chain)` 那段可以整個拿掉。
以下是原本的說明，留著當作「為什麼線上不要掛 CORS」的理由。

~~CORS 只在開發模式掛。~~ 本機 Vite 在 5173、Go 在 8080，跨來源所以需要；
正式環境兩者都在同一個網域底下（Firebase Hosting 同時服務 `/admin` 與轉給
Cloud Run 的 `/api`），同源不需要。線上掛 CORS 是白白放寬瀏覽器本來會幫你擋的限制。

---

## 錯誤要分三類

| 情況 | 回應 | 為什麼 |
|---|---|---|
| 沒帶標頭、token 過期、token 無效 | `401 unauthorized` | 使用者可以自己解決——重新登入 |
| token 有效但 UID 不在白名單 | `403 forbidden` | 使用者做什麼都沒用 |
| 拿不到 Google 公鑰 | `500 internal` | **是我們的問題，不是使用者的** |

最後一類容易被忽略。`IsCertificateFetchFailed` 代表我們連不到 Google —— 這時候回 401
會讓使用者以為自己登入壞掉，反覆重登也沒用。要回 500 並且**寫 log**，因為那是需要有人去看的事故。

> 其餘 `default` 分支也要記 log。驗證失敗的原因很多（`aud` 不符、發行者不對、
> 格式壞掉），不記下來的話線上出問題你只會看到一堆 401，不知道為什麼。

---

## ⚠️ 模擬器會完全跳過驗簽

這不是傳聞，是 SDK 原始碼寫死的行為：

```go
// firebase.google.com/go/v4@v4.22.0/auth/token_verifier.go:172
// In emulator mode, skip signature verification
if isEmulator {
    return payload, nil
}
```

只要 `FIREBASE_AUTH_EMULATOR_HOST` 有值，**任何人隨手捏一個 JWT 都會通過驗證**。
本機是功能，線上是「認證機制完全不存在」。

所以 `internal/config` 一定要擋：

```go
if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" && os.Getenv("BLOG_DEV") != "1" {
    return nil, errors.New("偵測到 Auth 模擬器設定但不在開發模式，拒絕啟動")
}
```

`cloud_run.tf` 沒有設這個變數，所以正常流程不會發生。這道檢查防的是
「有人手動改了 Cloud Run 的環境變數」或「未來把 .env 打包進 image」。

---

## config 的另一道保險

```go
raw := strings.TrimSpace(os.Getenv("ADMIN_UIDS"))
if raw == "" {
    return nil, errors.New("ADMIN_UIDS 未設定，拒絕啟動")
}

admins := make(map[string]bool)
for _, s := range strings.Split(raw, ",") {
    if s = strings.TrimSpace(s); s != "" {
        admins[s] = true
    }
}
if len(admins) == 0 {
    return nil, errors.New("ADMIN_UIDS 只有逗號沒有內容")
}
```

用 `map` 而不是 slice —— 查白名單是 O(1)，而且 `if !a.admins[tok.UID]` 讀起來很直接。

**空值要拒絕啟動，不要退回「誰都可以」。** 變數沒設是部署疏失，而疏失的安全解讀是
「不給存取」。設錯把自己鎖在外面，五分鐘修好；設錯把全世界放進來，你可能幾個月都不知道。

第二個檢查（只有逗號）是防 `ADMIN_UIDS=,,,` 這種情況——`raw` 不是空字串，
但拆完之後一個有效 UID 都沒有。

---

## 需要更嚴格的時候

```go
tok, err := client.VerifyIDTokenAndCheckRevoked(ctx, raw)
```

多檢查一件事：這個 token 是否已被撤銷（使用者改密碼、或你手動呼叫 `RevokeRefreshTokens`）。

代價是**每次都要打一次 Firebase API**，失去離線驗證的優勢。

**這個 blog 不需要。** 只有你一個使用者，token 一小時就過期，而「撤銷後最多再撐一小時」
完全可以接受。如果哪天真的要緊急踢掉某個身分，把 UID 從 `admin_uids` 拿掉再 apply 更快也更徹底。

---

## 對照 `api_test.http`

實作完這個 middleware，這兩則應該變綠：

| 測試 | 檢查 |
|---|---|
| 1 | 不帶 token → 401，且錯誤格式符合契約 |
| 2 | 非白名單 token → 403，且 code 是 `forbidden` 不是 `unauthorized` |

第 2 則需要 `otherUserIdToken`：一個**簽章有效、但 UID 不在白名單**的 token。
實際做法是用另一個 Google 帳號在 `scripts/get-uid.html` 登入，把拿到的 ID token
貼進 `http-client.private.env.json`（2026-09-28 已用這個方式實測通過）。

> 沒有第二個帳號的話，也可以暫時把自己的 UID 從本機 `.env` 的 `ADMIN_UIDS` 拿掉，
> 用同一個 token 打一次，應該從 200 變成 403。測完記得改回來。
> 模擬器的 token 只有在你也設了 `FIREBASE_AUTH_EMULATOR_HOST` 時才驗得過。
