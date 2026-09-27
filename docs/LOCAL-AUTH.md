# 本機怎麼測登入

> 這份講「怎麼拿到 token 來測」。**middleware 本身怎麼寫看
> [`AUTH-GO.md`](AUTH-GO.md)** ——那裡有編譯驗證過的程式碼。

要測 `/api/*`，你需要一個 **Firebase ID token** 放進 `Authorization` 標頭。
這份文件說明怎麼在本機拿到它。

---

## 用模擬器，不要打正式的 Firebase

Firebase 提供一個本機的 Auth 模擬器。用它的三個理由：

1. **不需要任何真實憑證** —— 不用把密碼存在 API 工具的設定檔裡
2. **完全隔離** —— 測試產生的帳號不會出現在正式專案的使用者清單
3. **想建幾個帳號就建幾個** —— 測「非管理員被擋掉」這種情境很方便

> 對照組：打正式 Firebase 需要啟用 Email/Password 登入方式，那等於替後台
> 多開一條密碼登入路徑。我們刻意把它關掉了，見 [PLAN.md](PLAN.md)。

---

## 一、安裝與啟動

```bash
npm i -g firebase-tools
```

```bash
cd ~/go-projects/blog && firebase emulators:start --only auth --project cindle-blog
```

模擬器需要 Java（你的 SDKMAN 已經有 17）。啟動後：

| 位置 | 用途 |
|---|---|
| `http://localhost:9099` | Auth API |
| `http://localhost:4000/auth` | 網頁介面，可以看到所有測試帳號 |

設定在專案根目錄的 `firebase.json`。**保持這個視窗開著**，關掉就沒了 ——
模擬器的資料存在記憶體，重啟一次全部歸零（這是好事，測試環境本來就該可拋棄）。

---

## 二、建立測試帳號並拿到 token

用 curl、Postman、Insomnia、GoLand 內建的 HTTP Client 都可以。

```http
POST http://localhost:9099/identitytoolkit.googleapis.com/v1/accounts:signUp?key=any
Content-Type: application/json

{
  "email": "dev@local.test",
  "password": "dev12345",
  "returnSecureToken": true
}
```

> `key=any` 不是筆誤。模擬器不驗證 API key，隨便填什麼都通過。

回應：

```jsonc
{
  "idToken":      "eyJhbGciOiJub25lIiwi...",   // ← 這個放進 Authorization
  "refreshToken": "...",
  "expiresIn":    "3600",
  "localId":      "abc123XYZ..."               // ← 這個就是 UID
}
```

**把 `localId` 記下來**，下一步要用。

帳號已經存在的話，`accounts:signUp` 會回 `EMAIL_EXISTS`，改用登入：

```http
POST http://localhost:9099/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=any
Content-Type: application/json

{ "email": "dev@local.test", "password": "dev12345", "returnSecureToken": true }
```

---

## 三、設定 `.env`

```bash
# 本機 .env，兩行都要
FIREBASE_AUTH_EMULATOR_HOST=localhost:9099
ADMIN_UIDS=<上一步的 localId>
```

`FIREBASE_AUTH_EMULATOR_HOST` 是 **Firebase Admin SDK 自己會讀的變數** ——
設了之後它就改連模擬器，你的 Go 程式碼一行都不用改。

重啟 server：

```bash
set -a; . ./.env; set +a; go run ./cmd/server
```

---

## 四、打 API

```http
GET http://localhost:8080/api/posts
Authorization: Bearer eyJhbGciOiJub25lIiwi...
```

驗證這三種情況都對：

| 測試 | 預期 |
|---|---|
| 帶有效 token 且 UID 在 `ADMIN_UIDS` | `200` |
| 不帶 `Authorization` 標頭 | `401 unauthorized` |
| 帶另一個帳號的 token（UID 不在白名單） | `403 forbidden` |

**第三項最重要。** 它驗證的是「驗證身分」與「檢查權限」確實是兩道關卡 ——
只做第一道的話，全世界有 Google 帳號的人都能發文。

測法：再跑一次第二步建一個 `other@local.test`，用它的 token 打同一個端點。

---

## 五、token 過期了

有效期一小時。過期後重跑第二步的 `signInWithPassword` 就好。

或者用 refresh token 換新的：

```http
POST http://localhost:9099/securetoken.googleapis.com/v1/token?key=any
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&refresh_token=<refreshToken>
```

---

## ⚠️ 一個必須加的保險

設了 `FIREBASE_AUTH_EMULATOR_HOST` 之後，**Admin SDK 會完全跳過簽章驗證**。
任何人隨手捏一個 token 都會通過。

這在本機是功能，在正式環境是災難 —— 等於整個認證機制不存在。所以
`internal/config` 要擋一道：

```go
if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" && os.Getenv("BLOG_DEV") != "1" {
    log.Fatal("偵測到 Auth 模擬器設定但不在開發模式，拒絕啟動")
}
```

這個變數跟著部署上去的機率不高，但後果嚴重到值得多寫這三行。
`cloud_run.tf` 沒有設定這個變數，所以正常流程不會發生 —— 這道檢查防的是
「有人手動改了 Cloud Run 的環境變數」或「未來某天有人把 .env 打包進 image」。

---

## 附錄：測真正的 Google 登入

模擬器測不到真實的 Google OAuth 流程（那需要瀏覽器跳轉和使用者同意），
所以有兩件事只能用真的 Firebase 驗：

- Google 登入本身能不能跑通
- 你的真實 UID 是多少

這兩件都會在 **Phase 4.6** 的後台登入頁完成。那時候拿到的 UID 才是要填進
`terraform.tfvars` 的 `admin_uids` 的值。

在那之前，模擬器足以開發並測試所有 `/api/*` 的邏輯。
