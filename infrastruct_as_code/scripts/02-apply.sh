#!/usr/bin/env bash
#
# 步驟 2 — 建立所有 GCP 資源。
#
# 包含：啟用 API、Firestore（含複合索引）、Artifact Registry、
# 上傳用的 GCS bucket、Cloud Run 服務與其執行身分、預算警示。
#
# 預設只做 plan，確認沒問題再帶 --apply 真的執行：
#
#   ./scripts/02-apply.sh            # 只看要改什麼
#   ./scripts/02-apply.sh --apply    # 實際執行
#
# 可以重複執行。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need terraform

DO_APPLY=0
[[ "${1:-}" == "--apply" ]] && DO_APPLY=1

step "檢查環境"
check_account

step "檢查 terraform.tfvars"
if [[ ! -f "$TF_DIR/terraform.tfvars" ]]; then
  warn "找不到 $TF_DIR/terraform.tfvars"
  printf '  先複製範本再填值：\n\n'
  printf '    cp %s/terraform.tfvars.example %s/terraform.tfvars\n\n' "$TF_DIR" "$TF_DIR"
  die "terraform.tfvars 不存在"
fi
ok "terraform.tfvars 存在（已 gitignore，不會進版控）"

cd "$TF_DIR"

step "terraform init"
terraform init -input=false

step "terraform fmt 檢查"
if terraform fmt -check -recursive >/dev/null 2>&1; then
  ok "格式正確"
else
  warn "格式不一致，自動修正中"
  terraform fmt -recursive
fi

step "terraform validate"
terraform validate

step "terraform plan"
terraform plan -input=false -out=tfplan

if [[ "$DO_APPLY" -eq 0 ]]; then
  printf '\n%s這只是 plan，沒有真的動到任何東西。%s\n' "$c_yellow" "$c_reset"
  printf '確認上面的變更沒問題後，執行：%s./scripts/02-apply.sh --apply%s\n' "$c_bold" "$c_reset"
  exit 0
fi

confirm "即將把上面的變更套用到 $PROJECT_ID。"

step "terraform apply"
terraform apply -input=false tfplan
rm -f tfplan

step "輸出"
terraform output

cat <<EOF

$c_green${c_bold}資源建立完成。$c_reset

${c_bold}還沒做完的兩件事：$c_reset

1. ${c_bold}site_base_url 現在應該還是空的。$c_reset
   從上面的 cloud_run_url 複製網址，填回 terraform.tfvars，再跑一次
   ./scripts/02-apply.sh --apply。自訂網域上線後再改成正式網址。

2. ${c_bold}Firebase Auth 的登入方式要在 Console 開啟。$c_reset
   這部分沒有 Terraform 資源可用，必須手動：
   https://console.firebase.google.com/project/$PROJECT_ID/authentication/providers
   啟用 Google 登入，然後把你自己的 UID 填進 Cloud Run 的 ADMIN_UIDS 環境變數。

下一步：${c_bold}./scripts/03-local-dev-auth.sh$c_reset
EOF
