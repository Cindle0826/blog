#!/usr/bin/env bash
#
# 步驟 1 — 建立存放 Terraform state 的 GCS bucket。
#
# 為什麼要獨立一支：主要的 Terraform 設定把 state 放在這個 bucket 裡，
# 所以它不能由自己建立自己的 backend（先有雞還是先有蛋）。
# bootstrap/ 用本機 state 管這一個資源就好——它只會跑一次。
#
# 可以重複執行。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need terraform

step "檢查環境"
check_account

bucket="gs://${PROJECT_ID}-tfstate"

if gcloud storage buckets describe "$bucket" --format='value(name)' >/dev/null 2>&1; then
  skip "state bucket $bucket 已存在"
  printf '\n下一步：%s./scripts/02-apply.sh%s\n' "$c_bold" "$c_reset"
  exit 0
fi

step "用 Terraform 建立 state bucket"
cd "$TF_DIR/bootstrap"

terraform init -input=false
terraform apply -input=false \
  -var="project_id=$PROJECT_ID" \
  -var="region=$REGION"

ok "state bucket 建立完成：$bucket"

warn "bootstrap/ 的 state 存在本機（bootstrap/terraform.tfstate），沒有進 git。"
warn "它只管一個 bucket，弄丟了重跑 terraform import 就好，但別刪掉 bucket 本身。"

printf '\n下一步：%s./scripts/02-apply.sh%s\n' "$c_bold" "$c_reset"
