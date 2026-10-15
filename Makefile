# 常用指令。直接打 make 會列出所有指令。
#
# 額外的 go test 參數用 ARGS 傳，例如：
#   make test-store ARGS="-run Slug -v"

ARGS ?=

.DEFAULT_GOAL := help
.PHONY: help test test-handler test-slug test-store

help: ## 列出所有指令
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-14s %s\n", $$1, $$2}'

test: test-handler test-store ## 全部跑一次（handler + store；store 已包含 slug）

test-handler: ## handler 與 middleware 的測試（用假的 store，不需要資料庫）
	go test ./internal/handler -count=1 $(ARGS)

test-slug: ## slug 與 ID 格式的單元測試（不需要資料庫）
	FIRESTORE_EMULATOR_HOST= go test ./internal/store -count=1 -run '^Test(MakeSlug|ValidSlug|WithSuffix|ValidID)' $(ARGS)

test-store: ## store 的測試：啟動 Firestore 模擬器 → 跑測試 → 關掉模擬器
	./scripts/test-store.sh $(ARGS)
