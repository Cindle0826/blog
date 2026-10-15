#!/usr/bin/env bash
#
# 啟動 Firestore 模擬器、跑 internal/store 的測試，跑完自動關掉模擬器。
#
#   ./scripts/test-store.sh            # 跑全部
#   ./scripts/test-store.sh -run Slug  # 其他參數會原樣傳給 go test
#
# 模擬器是 gcloud 的元件（cloud-firestore-emulator），需要 Java 21 以上；
# gcloud 577 已經在警告 21 會停止支援，所以用 SDKMAN 裝的 Java 25。
# 預設的 Java 不受影響：這裡只在這支腳本裡把 Java 25 放到 PATH 最前面。
#
# 測試只會連到模擬器（專案 ID 是 demo- 開頭），不會碰正式的 Firestore。

set -euo pipefail

PORT="${FIRESTORE_TEST_PORT:-8090}"
HOST="127.0.0.1:${PORT}"

# 找 SDKMAN 裡 21 以上的 Java，挑版本最新的那個
JAVA_HOME_FOR_EMULATOR=""
for dir in "$HOME"/.sdkman/candidates/java/*/; do
  name="$(basename "$dir")"
  major="${name%%.*}"
  if [[ "$major" =~ ^[0-9]+$ ]] && (( major >= 21 )); then
    JAVA_HOME_FOR_EMULATOR="${dir%/}"
  fi
done
if [[ -z "$JAVA_HOME_FOR_EMULATOR" ]]; then
  echo "找不到 Java 21 以上。安裝方式：sdk install java 25.0.4.1+1-tem（問是否設為預設時回答 n）" >&2
  exit 1
fi

if ! gcloud components list --filter="id:cloud-firestore-emulator" --format="value(state.name)" 2>/dev/null | grep -q Installed; then
  echo "還沒安裝模擬器：gcloud components install cloud-firestore-emulator" >&2
  exit 1
fi

LOG="$(mktemp "${TMPDIR:-/tmp}/firestore-emulator.XXXXXX")"
echo "啟動模擬器（Java：$(basename "$JAVA_HOME_FOR_EMULATOR")，port $PORT，log：$LOG）"
PATH="$JAVA_HOME_FOR_EMULATOR/bin:$PATH" JAVA_HOME="$JAVA_HOME_FOR_EMULATOR" \
  gcloud emulators firestore start --host-port="$HOST" >"$LOG" 2>&1 &
EMULATOR_PID=$!
disown "$EMULATOR_PID" # 結束時是我們自己關的，不要讓 bash 印出 "Terminated"

cleanup() {
  # gcloud 會再開一個 java 子行程，兩個都要關
  pkill -P "$EMULATOR_PID" 2>/dev/null || true
  kill "$EMULATOR_PID" 2>/dev/null || true
  pkill -f "cloud-firestore-emulator.*--port=${PORT}" 2>/dev/null || true
  # java 收到訊號後要一兩秒才真的結束。等它放掉 port，連續執行兩次才不會撞在一起
  for _ in $(seq 1 20); do
    pgrep -f "cloud-firestore-emulator.*--port=${PORT}" >/dev/null || return 0
    sleep 0.5
  done
}
trap cleanup EXIT

for _ in $(seq 1 60); do
  if curl -s -o /dev/null "http://$HOST/"; then
    break
  fi
  if ! kill -0 "$EMULATOR_PID" 2>/dev/null; then
    echo "模擬器啟動失敗：" >&2
    cat "$LOG" >&2
    exit 1
  fi
  sleep 1
done

cd "$(dirname "$0")/.."
FIRESTORE_EMULATOR_HOST="$HOST" go test ./internal/store -count=1 "$@"
