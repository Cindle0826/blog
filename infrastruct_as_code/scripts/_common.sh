#!/usr/bin/env bash
# 所有腳本共用的設定與工具函式。不要直接執行這支，用 source 載入。

set -euo pipefail

# ⚠️ 變數後面接中文時一定要寫成 ${VAR}，不能寫 $VAR。
#
# macOS 內建的是 bash 3.2，它判斷變數名結尾時會把非 ASCII 的高位元組
# 當成識別字的一部分。所以 "$PROJECT_ID（原本是…）" 會被解讀成要展開
# 一個叫 PROJECT_ID（ 的變數，配上 set -u 就是 unbound variable，腳本直接死掉。
#
# 全形括號、頓號、句號都會觸發。加大括號就沒事。

PROJECT_ID="${PROJECT_ID:-cindle-blog}"
REGION="${REGION:-asia-east1}"

# 計費帳戶 ID 不寫死。它不是憑證——光知道 ID 動不了任何東西，要有帳戶上的
# IAM 權限才行——但它是個帳戶識別碼，沒必要出現在一個會公開的 repo 裡。
#
# 解析順序：環境變數 → 專案已接的帳戶 → 帳戶清單裡第一個 open 的。
# 第二種在 00-prereqs.sh 首次執行時還不存在（還沒接），所以需要第三種。
resolve_billing_account() {
  [[ -n "${BILLING_ACCOUNT:-}" ]] && { printf '%s' "$BILLING_ACCOUNT"; return; }

  local acct
  acct="$(gcloud billing projects describe "$PROJECT_ID" \
    --format='value(billingAccountName)' 2>/dev/null | sed 's|billingAccounts/||')"
  [[ -n "$acct" ]] && { printf '%s' "$acct"; return; }

  gcloud billing accounts list --filter='open=true' \
    --format='value(name)' --limit=1 2>/dev/null | sed 's|billingAccounts/||'
}

BILLING_ACCOUNT="$(resolve_billing_account)"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$(cd "$SCRIPT_DIR/../terraform" && pwd)"

# ── 輸出 ──────────────────────────────────────────────────
c_reset=$'\033[0m'; c_bold=$'\033[1m'
c_red=$'\033[31m'; c_green=$'\033[32m'; c_yellow=$'\033[33m'; c_blue=$'\033[34m'

step() { printf '\n%s▶ %s%s\n' "$c_bold$c_blue" "$*" "$c_reset"; }
ok()   { printf '%s✓%s %s\n' "$c_green" "$c_reset" "$*"; }
warn() { printf '%s!%s %s\n' "$c_yellow" "$c_reset" "$*"; }
die()  { printf '%s✗ %s%s\n' "$c_red" "$*" "$c_reset" >&2; exit 1; }

# skip 用於「已經是目標狀態」的情況——腳本要能重複執行而不出錯。
skip() { printf '%s·%s %s（已完成，跳過）\n' "$c_yellow" "$c_reset" "$*"; }

# confirm 用在會產生費用或不可逆的動作前。
# 設 ASSUME_YES=1 可略過（給 CI 用，本機請不要）。
confirm() {
  [[ "${ASSUME_YES:-}" == "1" ]] && return 0
  local reply
  printf '\n%s%s%s\n' "$c_bold" "$*" "$c_reset"
  # 明講「按 Enter 等於取消」。危險動作預設 No 是對的，但不講清楚的話，
  # 取消之後腳本就直接結束，看起來跟「正常跑完」幾乎一樣——
  # 使用者會以為做完了，然後在下一支腳本撞到莫名其妙的錯誤。
  read -r -p "繼續？輸入 y 執行；直接按 Enter 取消 [y/N] " reply
  if [[ ! "$reply" =~ ^[Yy]$ ]]; then
    die "已取消。沒有做任何變更，這支腳本後面的步驟也都沒有執行。"
  fi
}

need() { command -v "$1" >/dev/null 2>&1 || die "找不到指令：$1"; }

# 確認 gcloud 目前登入的帳號不是空的，並印出來讓人有機會發現登錯帳號。
check_account() {
  local acct
  acct="$(gcloud config get-value account 2>/dev/null || true)"
  [[ -n "$acct" && "$acct" != "(unset)" ]] || die "gcloud 尚未登入，先跑 gcloud auth login"
  printf '  帳號：%s\n  專案：%s\n  區域：%s\n' "$acct" "$PROJECT_ID" "$REGION"
}
