# ─────────────────────────────────────────────────────────────────────
# Cobo IAM Services — Makefile
#
# LOCAL (docker-compose.dev.yml):
#   go run ./cmd/api + npm run dev + volume mount source code
#   Targets: dc-*
#
# DEV SERVER (docker-compose.artifacts.yml via SSH):
#   pre-compiled binaries (./bin/) + built FE (./web/dist/) + nginx
#   Targets: deploy-* / dev-*
#
# BE : cobo_iam_services/  (thư mục này)
# FE : ../cobo_web_design/
# Dev: 88.216.208.0:21239  user: root  path: /root/cobo_project
# ─────────────────────────────────────────────────────────────────────

DEV_HOST  := 88.216.208.0
DEV_PORT  := 21239
DEV_USER  := root
DEV_PATH  := /root/cobo_project

FE_DIR    := ../cobo_web_design
ARTIFACTS := ./deploy-artifacts

SSH := ssh -p $(DEV_PORT) $(DEV_USER)@$(DEV_HOST)
SCP := scp -P $(DEV_PORT)

ensure_fe_env = if [ "$$(id -u)" -eq 0 ]; then \
	echo "FE targets must not run with sudo/root."; \
	echo "Reason: root is using a different Node/npm runtime than your user shell, which breaks Vite startup."; \
	echo "Run without sudo, for example: make fe-build"; \
	exit 1; \
fi; \
node -e "const major = Number(process.versions.node.split('.')[0]); const ok = (major >= 18 && major < 19) || (major >= 20 && major < 21) || major >= 22; if (!ok) { console.error('FE targets require Node ^18 || ^20 || >=22. Current: ' + process.version); process.exit(1); }"

.DEFAULT_GOAL := help
.PHONY: help \
    be-build be-build-linux be-run be-run-worker be-test \
    fe-install fe-dev fe-build fe-test fe-clean \
    dc-up dc-down dc-build dc-rebuild dc-logs dc-ps dc-restart \
    deploy-init deploy-be deploy-fe deploy-all push-migration \
    deploy-dev deploy-dev-be deploy-dev-fe deploy-dev-migrate \
    dev-up dev-down dev-ps dev-logs dev-restart dev-ssh

# ─────────────────────────────────────────────────────────────────────
help:  ## Liệt kê tất cả targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
	    awk 'BEGIN{FS=":.*?## "}{printf "  %-24s %s\n",$$1,$$2}'

# ─────────────────────────────────────────────────────────────────────
# Backend (Go)
# ─────────────────────────────────────────────────────────────────────
be-build: ## Build binary cho OS hiện tại → bin/api, bin/worker
	go build -o bin/api    ./cmd/api
	go build -o bin/worker ./cmd/worker

be-build-linux: ## Cross-compile Linux x86-64 → deploy-artifacts/backend/bin/
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(ARTIFACTS)/backend/bin/api    ./cmd/api
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(ARTIFACTS)/backend/bin/worker ./cmd/worker

be-run: ## Chạy API local (in-memory nếu không có MYSQL_DSN)
	go run ./cmd/api

be-run-worker: ## Chạy Worker local
	go run ./cmd/worker

be-test: ## Chạy toàn bộ Go test
	go test ./...

# ─────────────────────────────────────────────────────────────────────
# Frontend (React/Vite — ../cobo_web_design)
# ─────────────────────────────────────────────────────────────────────
fe-install: ## npm install
	@$(ensure_fe_env)
	cd $(FE_DIR) && npm install

fe-dev: ## Khởi động Vite dev server (port 3000)
	@$(ensure_fe_env)
	cd $(FE_DIR) && npm run dev

fe-build: ## Production build → cobo_web_design/dist/
	@$(ensure_fe_env)
	cd $(FE_DIR) && npm run build

fe-test: ## Chạy FE tests
	@$(ensure_fe_env)
	cd $(FE_DIR) && npm test

fe-clean: ## Xóa dist/
	@$(ensure_fe_env)
	cd $(FE_DIR) && npm run clean

# ─────────────────────────────────────────────────────────────────────
# Docker Compose — LOCAL  (docker-compose.dev.yml)
# go run + npm run dev + volume mount source code
# ─────────────────────────────────────────────────────────────────────
dc-up: ## [local] Khởi động toàn bộ stack (detached)
	docker compose -f docker-compose.dev.yml up -d

dc-down: ## [local] Dừng và xóa containers
	docker compose -f docker-compose.dev.yml down

dc-build: ## [local] Build Docker images
	docker compose -f docker-compose.dev.yml build

dc-rebuild: ## [local] Full rebuild: down → build --no-cache → up
	docker compose -f docker-compose.dev.yml down
	docker compose -f docker-compose.dev.yml build --no-cache
	docker compose -f docker-compose.dev.yml up -d

dc-logs: ## [local] Xem log realtime (Ctrl-C để thoát)
	docker compose -f docker-compose.dev.yml logs -f

dc-ps: ## [local] Xem trạng thái containers
	docker compose -f docker-compose.dev.yml ps

dc-restart: ## [local] Restart toàn bộ containers
	docker compose -f docker-compose.dev.yml restart

# ─────────────────────────────────────────────────────────────────────
# Deploy → Dev server  (docker-compose.artifacts.yml)
# pre-compiled binaries + built FE + nginx
# ─────────────────────────────────────────────────────────────────────
deploy-init: ## [dev] Lần đầu: SCP docker-compose.artifacts.yml + tạo thư mục trên server
	$(SCP) docker-compose.artifacts.yml \
	    $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/docker-compose.artifacts.yml
	$(SSH) "mkdir -p $(DEV_PATH)/bin $(DEV_PATH)/configs $(DEV_PATH)/web $(DEV_PATH)/migrations"
	$(SCP) -r migrations $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/
	@echo "==> Xong. Tiếp theo: tạo $(DEV_PATH)/.env từ configs/config.example.env trên server"

deploy-be: be-build-linux ## [dev] Build Linux binary, SCP bin/ + configs/, restart api + worker
	$(SSH) "mkdir -p $(DEV_PATH)/bin $(DEV_PATH)/configs && rm -rf $(DEV_PATH)/bin/api $(DEV_PATH)/bin/worker $(DEV_PATH)/migrations"
	$(SCP) $(ARTIFACTS)/backend/bin/api    $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/bin/.api.tmp
	$(SCP) $(ARTIFACTS)/backend/bin/worker $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/bin/.worker.tmp
	$(SCP) -r $(ARTIFACTS)/backend/configs $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/
	$(SCP) -r migrations $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/
	$(SSH) "mv $(DEV_PATH)/bin/.api.tmp $(DEV_PATH)/bin/api && \
	    mv $(DEV_PATH)/bin/.worker.tmp $(DEV_PATH)/bin/worker && \
	    chmod 755 $(DEV_PATH)/bin/api $(DEV_PATH)/bin/worker && \
	    cd $(DEV_PATH) && \
	    docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker"

deploy-fe: fe-build ## [dev] Build FE, copy dist + nginx.conf, SCP, restart web
	rm -rf $(ARTIFACTS)/web/dist
	cp -r $(FE_DIR)/dist $(ARTIFACTS)/web/dist
	$(SSH) "mkdir -p $(DEV_PATH)/web && rm -rf $(DEV_PATH)/web/dist && mkdir -p $(DEV_PATH)/web/dist"
	$(SCP) -r $(ARTIFACTS)/web/dist/*   $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/web/dist/
	$(SCP)    $(ARTIFACTS)/web/nginx.conf $(DEV_USER)@$(DEV_HOST):$(DEV_PATH)/web/nginx.conf
	$(SSH) "cd $(DEV_PATH) && \
	    docker compose -f docker-compose.artifacts.yml restart web"

deploy-all: deploy-be deploy-fe ## [dev] Deploy cả BE + FE lên dev

deploy-dev: ## [dev] Deploy dev (auto-detect BE/FE/migrations): make deploy-dev [MODE=be|fe|all|migrate|verify]
	sh deploy-dev.sh $(MODE)

deploy-dev-be: ## [dev] Chỉ deploy BE lên dev
	sh deploy-dev.sh be

deploy-dev-fe: ## [dev] Chỉ deploy FE lên dev
	sh deploy-dev.sh fe

deploy-dev-migrate: ## [dev] Chỉ push + apply migrations mới lên dev
	sh deploy-dev.sh migrate

push-migration: ## [dev] Push + apply một migration: make push-migration FILE=0007_foo.up.sql
	@test -n "$(FILE)" || { echo "Usage: make push-migration FILE=0007_foo.up.sql"; exit 1; }
	sh $(ARTIFACTS)/push-migration.sh $(FILE)

# ─────────────────────────────────────────────────────────────────────
# Quản lý stack trên dev server (SSH remote)
# ─────────────────────────────────────────────────────────────────────
dev-up: ## [dev] Khởi động artifacts stack trên dev server
	$(SSH) "cd $(DEV_PATH) && docker compose -f docker-compose.artifacts.yml up -d"

dev-down: ## [dev] Dừng artifacts stack trên dev server
	$(SSH) "cd $(DEV_PATH) && docker compose -f docker-compose.artifacts.yml down"

dev-ps: ## [dev] Xem trạng thái containers trên dev server
	$(SSH) "cd $(DEV_PATH) && docker compose -f docker-compose.artifacts.yml ps"

dev-logs: ## [dev] Tail logs trên dev server (Ctrl-C để thoát)
	$(SSH) "cd $(DEV_PATH) && docker compose -f docker-compose.artifacts.yml logs -f"

dev-restart: ## [dev] Restart toàn bộ artifacts stack trên dev server
	$(SSH) "cd $(DEV_PATH) && docker compose -f docker-compose.artifacts.yml restart"

dev-ssh: ## [dev] Mở SSH shell vào dev server
	ssh -p $(DEV_PORT) $(DEV_USER)@$(DEV_HOST)
