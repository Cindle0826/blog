# infrastruct_as_code

`cindle-blog` 這個 GCP 專案的基礎設施，全部由 Terraform 管理。
結構跟 `line-bot-ledger` 一樣：`bootstrap/` 單獨管 state bucket，其餘在 `terraform/`。

## 跑一遍

```bash
cd infrastruct_as_code

./scripts/00-prereqs.sh        # 接計費帳戶 + 開兩個 Terraform 自己需要的 API
./scripts/01-state-bucket.sh   # 建 GCS state bucket
./scripts/02-apply.sh          # 只看 plan（terraform.tfvars 不存在會自動產生）
./scripts/02-apply.sh --apply  # 確認後實際建立

./scripts/03-local-dev-auth.sh # 本機開發憑證
./scripts/verify.sh            # 唯讀檢查，隨時可跑
```

每支都可以重複執行，已經是目標狀態的步驟會直接跳過。

## Terraform 管了什麼

| 檔案 | 資源 |
|---|---|
| `apis.tf` | 10 個 API |
| `firestore.tf` | database（Native、asia-east1）＋ 4 個複合索引 |
| `artifact_registry.tf` | Docker repo ＋ 清理規則 |
| `storage.tf` | 圖片上傳 bucket（公開讀取）＋ CORS ＋ 版本保留 |
| `iam.tf` | Cloud Run 的執行身分與最小權限 |
| `cloud_run.tf` | `blog` 服務（scale-to-zero、上限 3 個實例） |
| `budget.tf` | NT$1 預算警示 |
| `firebase.tf` | 後台的 Firebase Web App 註冊（需要 `google-beta` provider） |

## 幾個刻意的決定

**只有 `00-prereqs.sh` 用 gcloud 直接改雲端狀態。**
Terraform 要能管專案，專案得先接計費、先啟用 `cloudresourcemanager`——
但沒接計費就不能啟用 API。這個循環只能從外面打破一次，之後全部交給 Terraform。

**`max_instances = 3`。**
這是成本防線，不是容量規劃。個人 blog 不需要擴展，而**沒有上限**是這個專案最可能
停止免費的方式——一次爬蟲風暴或一個無窮迴圈就會變成真的帳單。

**Artifact Registry 有清理規則。**
免費額度只有 0.5 GiB。distroless 的 Go image 約 18MB，聽起來還能放 28 個，
直到你發現每次部署都推一個、而且沒有東西會刪。

**Firestore 開了 delete protection。**
擋住手滑的 `terraform destroy`。真要刪必須先改成 `ABANDON`、apply、再 destroy。

**上傳 bucket 是公開讀取的，但不可列舉。**
文章圖片本來就是要被爬、被分享的，所以匿名 GET 必須能拿到。

角色用的是 `roles/storage.legacyObjectReader` 而不是 `objectViewer`——
這個差別很重要：`objectViewer` 包含 `storage.objects.list`，等於任何人都能
**列出整個 bucket 的所有檔名**，包含還沒發布的草稿圖片、以及誤傳的檔案。
`legacyObjectReader` 只給 `storage.objects.get`，知道網址才拿得到。

驗證方式：

```bash
# 想要 403/401
curl https://storage.googleapis.com/storage/v1/b/cindle-blog-uploads/o
```

不管用哪個角色，**東西一上傳就是全世界可讀**。不要把私密的東西放進這個 bucket。

**不用 service account 金鑰檔。**
本機用 ADC，Cloud Run 用附掛的 service account。金鑰檔會躺在硬碟上、會不小心進 git、
而且沒有有效期限。

## Terraform 管不到的

這兩件事沒有對應的 Terraform 資源，必須在 Console 手動做：

1. **Firebase Auth 的登入方式**
   <https://console.firebase.google.com/project/cindle-blog/authentication/providers>
   啟用 Google 登入，登入一次，再從 Authentication → Users 複製 UID，
   填進 `terraform.tfvars` 的 `admin_uids` 後 apply。

   > Terraform 其實有 `google_identity_platform_default_supported_idp_config`，
   > 但它必填 `client_id` / `client_secret`，而**建立 OAuth 用戶端沒有 API**——
   > 你還是得先去 Console 建，再把機密複製進 tfvars。Console 按一下反而是
   > 三步變一步，而且少一個機密要保管。所以這裡刻意不用 Terraform。

2. **Firebase Hosting 的網站與網域**
   `firebase init hosting` 之後在 `firebase.json` 設定 rewrite 指向 Cloud Run。
   這部分屬於 Phase 6，見 [`../docs/PLAN.md`](../docs/PLAN.md)。

## state

在 `gs://cindle-blog-tfstate`（有開版本控制）。
`bootstrap/` 自己的 state 留在本機，因為它管的就是那個 bucket——
不能把 state 放在自己還沒建立的 bucket 裡。
