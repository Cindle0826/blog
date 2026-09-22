#!/usr/bin/env bash
#
# 步驟 0 — Terraform 跑得起來之前的前置作業。
#
# 這支是唯一一個「用 gcloud 直接改雲端狀態」的腳本，存在理由是先有雞還是先有蛋：
# Terraform 要能管理專案，專案得先接上計費帳戶、而且得先啟用
# cloudresourcemanager 這兩個 API——但沒接計費就不能啟用 API。
# 這個循環只能從外面打破，之後所有資源都交給 Terraform。
#
# 可以重複執行。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need gcloud

step "檢查環境"
check_account

step "檢查專案是否存在"
if gcloud projects describe "$PROJECT_ID" --format='value(projectId)' >/dev/null 2>&1; then
  ok "專案 $PROJECT_ID 存在"
else
  die "找不到專案 $PROJECT_ID。先建立：gcloud projects create $PROJECT_ID --name=\"Cindle Blog\""
fi

step "接上計費帳戶"
billing_enabled="$(gcloud billing projects describe "$PROJECT_ID" \
  --format='value(billingEnabled)' 2>/dev/null || echo False)"

if [[ "$billing_enabled" == "True" ]]; then
  skip "計費帳戶已接上"
else
  confirm "即將把專案 $PROJECT_ID 接上計費帳戶 $BILLING_ACCOUNT。
這會讓這個專案開始可以產生費用。本專案設計上會落在免費額度內，
而且下一步的 Terraform 會建立 NT\$1 的預算警示，但風險還是你在承擔。"

  gcloud billing projects link "$PROJECT_ID" --billing-account="$BILLING_ACCOUNT"
  ok "已接上計費帳戶"
fi

step "啟用 Terraform 自身需要的 API"
# 只開兩個：
#   cloudresourcemanager — Terraform 讀寫專案層級設定（含啟用其他 API）
#   serviceusage         — google_project_service 資源本身靠它運作
# 其餘的 API 全部寫在 terraform/apis.tf，由 Terraform 管，不要在這裡加。
for api in cloudresourcemanager.googleapis.com serviceusage.googleapis.com; do
  if gcloud services list --enabled --project="$PROJECT_ID" \
       --filter="config.name:$api" --format='value(config.name)' 2>/dev/null | grep -q .; then
    skip "$api"
  else
    gcloud services enable "$api" --project="$PROJECT_ID"
    ok "$api"
  fi
done

step "設定 gcloud 預設專案"
current="$(gcloud config get-value project 2>/dev/null || true)"
if [[ "$current" == "$PROJECT_ID" ]]; then
  skip "已經是 $PROJECT_ID"
else
  gcloud config set project "$PROJECT_ID" >/dev/null
  ok "預設專案改為 $PROJECT_ID（原本是 $current）"
fi

printf '\n%s前置作業完成。%s\n' "$c_green$c_bold" "$c_reset"
printf '下一步：%s./scripts/01-state-bucket.sh%s\n' "$c_bold" "$c_reset"
