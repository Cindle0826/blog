#!/usr/bin/env bash
#
# 步驟 4 — 從 Terraform output 生成本機開發用的 .env。
#
# 為什麼要用腳本而不是手動複製：這些值散在 terraform output 裡，手抄一次
# 就有抄錯和走味的風險。之後改了基礎設施（換 bucket、換網域），重跑這支
# 就同步了，不用回頭想「當初是從哪裡複製的」。
#
# ⚠️ 這裡產生的檔案**不含任何機密**。
#    GCP 的憑證走 Application Default Credentials（~/.config/gcloud/），
#    由 03-local-dev-auth.sh 設定，永遠不會出現在 .env 裡。
#    如果哪天你發現自己想把 service account 金鑰塞進 .env，那就是走錯路了。
#
# 產生兩個檔案，因為前後端的讀取方式不同：
#   .env                  後端（cmd/server）
#   web/admin/.env.local  前端（Vite 只讀它自己根目錄下的 .env*，
#                         而且只暴露 VITE_ 開頭的變數給瀏覽器）
#
# 可以重複執行；既有檔案會先備份。

source "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

need terraform

REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

step "檢查環境"
check_account

cd "$TF_DIR"

step "讀取 Terraform output"
if ! terraform output -json >/dev/null 2>&1; then
  die "讀不到 terraform output。先跑 ./scripts/02-apply.sh --apply"
fi

tf() { terraform output -raw "$1" 2>/dev/null || true; }

BUCKET="$(tf uploads_bucket)"
RUN_URL="$(tf cloud_run_url)"
FB_API_KEY="$(tf firebase_api_key)"
FB_AUTH_DOMAIN="$(tf firebase_auth_domain)"
FB_APP_ID="$(tf firebase_app_id)"

# admin_uids 是 list，-raw 取不出來，用 json 轉成逗號分隔
ADMIN_UIDS="$(terraform output -json 2>/dev/null \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print(','.join(d.get('admin_uids',{}).get('value',[])))" 2>/dev/null || true)"

[[ -n "$BUCKET" ]] || die "uploads_bucket 是空的，Terraform 可能還沒 apply"
ok "已取得 ${PROJECT_ID} 的設定"

# backup <檔案> — 既有檔案改名保留，不要默默覆蓋別人手動改過的值
backup() {
  [[ -f "$1" ]] || return 0
  local bak="$1.bak.$(date +%Y%m%d%H%M%S)"
  mv "$1" "$bak"
  warn "原檔已備份為 $(basename "$bak")"
}

step "產生後端 .env"
backup "$REPO_ROOT/.env"
cat > "$REPO_ROOT/.env" <<EOF
# 由 infrastruct_as_code/scripts/04-local-env.sh 產生
# $(date '+%Y-%m-%d %H:%M:%S')
#
# 這個檔案已被 .gitignore 排除。裡面沒有機密——GCP 憑證走 ADC，
# 不在這裡。可以安心貼進 issue 或聊天室求助。

GOOGLE_CLOUD_PROJECT=${PROJECT_ID}
BLOG_UPLOADS_BUCKET=${BUCKET}

# 本機跑的時候，站台網址就是本機。
# 這個值會變成 view.Site.BaseURL，所有 canonical URL 都從它組出來——
# 本機設成正式網址的話，你會在 localhost 上看到指向正式站的連結。
BLOG_BASE_URL=http://localhost:8080

# 改模板不用重啟 server
BLOG_DEV=1

# /api/* 的白名單，逗號分隔。
# 空的時候伺服器應該拒絕啟動，而不是退回「誰都可以」。
ADMIN_UIDS=${ADMIN_UIDS}

# ── Firebase ────────────────────────────────────────────
# Admin SDK 不需要憑證檔，也不需要另一個 project 變數——
# 它把上面的 GOOGLE_CLOUD_PROJECT 當 Firebase project ID（兩者本來就是
# 同一個字串），憑證走 ADC。所以這一區只有一個變數，而且平常要留空。
#
# 設了它，Admin SDK 就改連本機模擬器，並且**完全跳過簽章驗證**
# （auth/token_verifier.go:172 寫死的行為）——任何人隨手捏一個 JWT 都會通過。
# 本機是功能，線上等於認證機制不存在。所以 internal/config 要擋一道：
# 有值但 BLOG_DEV != 1 就拒絕啟動。
#
# FIREBASE_AUTH_EMULATOR_HOST=localhost:9099

# 參考用，本機開發不需要：
# CLOUD_RUN_URL=${RUN_URL}
EOF
ok "$REPO_ROOT/.env"

step "產生前端 web/admin/.env.local"
mkdir -p "$REPO_ROOT/web/admin"
backup "$REPO_ROOT/web/admin/.env.local"
cat > "$REPO_ROOT/web/admin/.env.local" <<EOF
# 由 infrastruct_as_code/scripts/04-local-env.sh 產生
# $(date '+%Y-%m-%d %H:%M:%S')
#
# Vite 只把 VITE_ 開頭的變數暴露給瀏覽器，而且是在 build 時**直接編進
# bundle**——所以這裡的每一個值最終都是公開的。這是設計如此，不是疏失：
# Firebase 的 web 設定本來就該公開，真正的安全來自後端驗證 ID token
# 並比對 ADMIN_UIDS。
#
# 絕對不要把任何真正的機密加上 VITE_ 前綴。

VITE_FIREBASE_API_KEY=${FB_API_KEY}
VITE_FIREBASE_AUTH_DOMAIN=${FB_AUTH_DOMAIN}
VITE_FIREBASE_PROJECT_ID=${PROJECT_ID}
VITE_FIREBASE_APP_ID=${FB_APP_ID}

# 後台 API 的位置。本機開發時 Vite 跑在 5173，Go 跑在 8080。
VITE_API_BASE=http://localhost:8080
EOF
ok "$REPO_ROOT/web/admin/.env.local"

step "產生 http_tests/http-client.private.env.json"
# 保留既有的 token——那些是手動從 get-uid.html 貼進來的，
# 每次重跑腳本都清掉的話會很煩。這裡只更新從 Terraform 來的值。
python3 - "$REPO_ROOT" "$FB_API_KEY" <<'PYEOF'
import json, pathlib, sys

root, api_key = sys.argv[1], sys.argv[2]
path = pathlib.Path(root) / "http_tests" / "http-client.private.env.json"

data = {}
if path.exists():
    try:
        data = json.loads(path.read_text())
    except json.JSONDecodeError:
        pass  # 檔案壞掉就重建，不要讓腳本死在這裡

# 環境名稱要跟 http-client.env.json 對得上，GoLand 才會把兩邊合併。
# emulator 的 key 是 "any"——模擬器不驗 API key。
for env, key in (("local", api_key), ("cloudrun", api_key), ("emulator", "any")):
    e = data.setdefault(env, {})
    e["firebaseApiKey"] = key
    for k in ("idToken", "refreshToken", "otherUserIdToken"):
        e.setdefault(k, "")

data["local"]["_comment"] = ("由 infrastruct_as_code/scripts/04-local-env.sh 產生。"
                             "firebaseApiKey 每次重跑都會更新；idToken / refreshToken "
                             "請自己從 scripts/get-uid.html 貼進來，不會被清掉。")

path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
print(f"  已寫入 {path}（local / cloudrun / emulator 三個環境）")
PYEOF
ok "http_tests/http-client.private.env.json"

step "產生 scripts/get-uid.config.js"
cat > "$REPO_ROOT/scripts/get-uid.config.js" <<EOF
// 由 infrastruct_as_code/scripts/04-local-env.sh 產生，不進版控。
window.FIREBASE_CONFIG = {
  apiKey:     "${FB_API_KEY}",
  authDomain: "${FB_AUTH_DOMAIN}",
  projectId:  "${PROJECT_ID}",
  appId:      "${FB_APP_ID}",
};
EOF
ok "scripts/get-uid.config.js"

step "確認沒有進版控"
cd "$REPO_ROOT"
tracked=0
for f in .env web/admin/.env.local http_tests/http-client.private.env.json scripts/get-uid.config.js; do
  if git check-ignore -q "$f" 2>/dev/null; then
    ok "$f 已被 .gitignore 排除"
  else
    warn "$f 沒有被忽略！"
    tracked=1
  fi
done
[[ "$tracked" -eq 0 ]] || die ".env 檔案可能會被 commit，先修 .gitignore"

cat <<EOF

$c_green${c_bold}完成。$c_reset

${c_bold}後端讀 .env 的方式$c_reset（internal/config 是你的範圍，選一個就好）：

  1. 終端機          set -a; . ./.env; set +a; go run ./cmd/server
  2. GoLand          Run Configuration → Environment variables → 選 .env 檔
  3. 程式裡載入       go get github.com/joho/godotenv
                     只在開發模式載入，正式環境用 Cloud Run 的環境變數

${c_bold}前端$c_reset Vite 會自動讀 web/admin/.env.local，不用額外設定。

EOF

if [[ -z "$ADMIN_UIDS" ]]; then
  warn "ADMIN_UIDS 目前是空的——還沒有人登入過，拿不到 UID。"
  warn "登入一次之後，把 UID 填進 terraform.tfvars 的 admin_uids，"
  warn "apply 完再重跑這支腳本就會同步過來。"
fi
