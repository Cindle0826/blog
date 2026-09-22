#!/usr/bin/env bash
#
# 唯讀的狀態檢查。不會改動任何東西，隨時可以跑。
#
#   ./scripts/verify.sh
#
# 表格的第一欄刻意用 ASCII：macOS 內建的是 bash 3.2，printf 的欄寬是按 byte 算的，
# 中文字在 UTF-8 佔 3 bytes 卻只顯示 2 欄，混排一定對不齊。
# 中文留在區塊標題和狀態文字，那些不需要對齊。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need gcloud

ok_mark="${c_green}✓${c_reset}"
no_mark="${c_red}✗${c_reset}"

row() { printf '  %-22s %s\n' "$1" "$2"; }

# have <label> <value> [缺少時的提示]
have() {
  if [[ -n "$2" ]]; then
    row "$1" "$ok_mark $2"
  else
    row "$1" "$no_mark 未建立${3:+　—　$3}"
  fi
}

step "帳號與專案"
check_account

step "專案與計費"
state="$(gcloud projects describe "$PROJECT_ID" --format='value(lifecycleState)' 2>/dev/null || true)"
have "project" "$state" "gcloud projects create $PROJECT_ID"

billing="$(gcloud billing projects describe "$PROJECT_ID" --format='value(billingEnabled)' 2>/dev/null || echo False)"
if [[ "$billing" == "True" ]]; then
  row "billing" "$ok_mark 已接上"
else
  row "billing" "$no_mark 未接上　—　跑 00-prereqs.sh"
fi

budget="$(gcloud billing budgets list --billing-account="$BILLING_ACCOUNT" \
  --format='value(displayName)' --quiet 2>/dev/null </dev/null | grep -c '^cindle-blog ' || true)"
if [[ "${budget:-0}" -gt 0 ]]; then
  row "budget-alert" "$ok_mark 已建立"
else
  row "budget-alert" "$no_mark 未建立　—　跑 02-apply.sh --apply"
fi

step "API"
# 一次把已啟用的 API 全撈回來在本機比對，而不是每個 API 各打一次 gcloud。
# 之前的寫法要打 7 次，每次幾秒，整支腳本跑超過兩分鐘。
enabled="$(gcloud services list --enabled --project="$PROJECT_ID" \
  --format='value(config.name)' --quiet 2>/dev/null </dev/null || true)"

for api in run firestore artifactregistry firebase firebasehosting identitytoolkit storage; do
  # grep -Fx 做「整行完全相等」比對。用子字串比對的話
  # storage 會誤中 bigquerystorage.googleapis.com。
  if printf '%s\n' "$enabled" | grep -Fxq "${api}.googleapis.com"; then
    row "$api" "$ok_mark"
  else
    row "$api" "$no_mark"
  fi
done

step "資源"
have "firestore-db" \
  "$(gcloud firestore databases describe --database='(default)' --project="$PROJECT_ID" \
     --format='value(locationId)' --quiet 2>/dev/null </dev/null || true)"

ar="$(gcloud artifacts repositories describe blog --location="$REGION" --project="$PROJECT_ID" \
  --format='value(name)' --quiet 2>/dev/null </dev/null || true)"
have "artifact-registry" "${ar##*/}"

have "uploads-bucket" \
  "$(gcloud storage buckets describe "gs://${PROJECT_ID}-uploads" \
     --format='value(name)' --quiet 2>/dev/null </dev/null || true)"

have "tfstate-bucket" \
  "$(gcloud storage buckets describe "gs://${PROJECT_ID}-tfstate" \
     --format='value(name)' --quiet 2>/dev/null </dev/null || true)" \
  "跑 01-state-bucket.sh"

have "cloud-run" \
  "$(gcloud run services describe blog --region="$REGION" --project="$PROJECT_ID" \
     --format='value(status.url)' --quiet 2>/dev/null </dev/null || true)"

step "本機開發"
adc="$HOME/.config/gcloud/application_default_credentials.json"
if [[ -f "$adc" ]]; then
  row "adc" "$ok_mark 已設定"
else
  row "adc" "$no_mark 未設定　—　跑 03-local-dev-auth.sh"
fi

echo
