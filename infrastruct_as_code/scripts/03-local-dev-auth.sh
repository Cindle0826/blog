#!/usr/bin/env bash
#
# 步驟 3 — 設定本機開發用的憑證。
#
# 讓本機跑的 `go run ./cmd/server` 能連上 Firestore。
# 用的是 Application Default Credentials（ADC），不是 service account 金鑰檔——
# 金鑰檔會躺在硬碟上、會不小心進 git、而且沒有有效期限。ADC 綁你的 Google 帳號，
# 會過期、可以撤銷，而且上了 Cloud Run 會自動換成附掛的 service account，
# 程式碼一行都不用改。
#
# 可以重複執行。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need gcloud

step "檢查環境"
check_account

adc_file="$HOME/.config/gcloud/application_default_credentials.json"

step "Application Default Credentials"
if [[ -f "$adc_file" ]]; then
  skip "ADC 已存在（${adc_file}）"
  warn "如果之後出現 403 或 quota project 相關錯誤，重跑一次："
  printf '    gcloud auth application-default login\n'
else
  printf '  瀏覽器會開起來要你登入 Google 帳號。\n'
  gcloud auth application-default login
  ok "ADC 設定完成"
fi

step "設定 ADC 的 quota project"
# ADC 本身沒有綁定計費專案，某些 API 會因此拒絕請求。
gcloud auth application-default set-quota-project "$PROJECT_ID" 2>/dev/null \
  && ok "quota project 設為 $PROJECT_ID" \
  || warn "設定 quota project 失敗，通常不影響 Firestore，出問題再回來處理"

step "驗證真的連得上 Firestore"
if gcloud firestore databases describe --database='(default)' \
     --project="$PROJECT_ID" --format='value(name)' >/dev/null 2>&1; then
  ok "Firestore 可存取"
else
  warn "讀不到 Firestore。先確認 ./scripts/02-apply.sh --apply 已經跑過。"
fi

cat <<EOF

$c_green${c_bold}本機開發環境就緒。$c_reset

跑起來看看：

  ${c_bold}# 前端預覽（假資料，不需要 GCP）$c_reset
  BLOG_DEV=1 go run ./cmd/preview

  ${c_bold}# 正式站（需要 Firestore）$c_reset
  GOOGLE_CLOUD_PROJECT=$PROJECT_ID go run ./cmd/server

${c_bold}不要$c_reset 下載 service account 金鑰檔。上面這個流程不需要，
Cloud Run 上也不需要，而且金鑰外洩是這類專案最常見的事故來源。
EOF
