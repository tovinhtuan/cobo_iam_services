#!/bin/sh
# =============================================================================
# deploy-dev.sh — Đẩy thay đổi lên môi trường dev server để kiểm thử
# =============================================================================
#
# THAM KHẢO:
#   Makefile          (deploy-be, deploy-fe, deploy-all, push-migration, dev-*)
#   deploy-artifacts/push-migration.sh
#   docker-compose.artifacts.yml
#   migrations/run_dev_migrations.sh
#
# CÁCH DÙNG:
#   sh deploy-dev.sh              # deploy BE + FE + migrations mới (default)
#   sh deploy-dev.sh be           # chỉ deploy BE (Go binaries)
#   sh deploy-dev.sh fe           # chỉ deploy FE (React build)
#   sh deploy-dev.sh migrate      # chỉ push + apply migrations mới
#   sh deploy-dev.sh all          # BE + FE + migrations
#   sh deploy-dev.sh verify       # chỉ kiểm tra trạng thái server (không deploy)
#
# FLAGS (truyền sau mode):
#   --skip-tests    bỏ qua go test / npm lint (nhanh hơn, ít an toàn hơn)
#
# VÍ DỤ:
#   sh deploy-dev.sh                        # deploy đầy đủ
#   sh deploy-dev.sh be --skip-tests        # chỉ BE, bỏ qua test
#   sh deploy-dev.sh migrate                # chỉ apply migrations mới
#   sh deploy-dev.sh fe                     # chỉ build + deploy FE
#   sh deploy-dev.sh verify                 # xem trạng thái không deploy gì
#
# YÊU CẦU:
#   - Có thể SSH tới root@88.216.208.0 (port 21239) bằng SSH key hoặc password
#   - Go toolchain (cho BE cross-compile)
#   - Node.js + npm (cho FE build)
# =============================================================================

set -eu

# ── Cấu hình dev server (bám Makefile) ───────────────────────────────────────
DEV_HOST="88.216.208.0"
DEV_PORT="21239"
DEV_USER="root"
DEV_PATH="/root/cobo_project"

# ── Đường dẫn nội bộ (tự resolve, không hardcode) ────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MAKEFILE_DIR="$SCRIPT_DIR"                         # Makefile nằm cùng thư mục với script này
ARTIFACTS_DIR="$SCRIPT_DIR/deploy-artifacts"
MIGRATIONS_DIR="$SCRIPT_DIR/migrations"
PUSH_MIGRATION_SCRIPT="$ARTIFACTS_DIR/push-migration.sh"

# ── Parse arguments ───────────────────────────────────────────────────────────
MODE="${1:-all}"
SKIP_TESTS=0
for arg in "$@"; do
  case "$arg" in
    --skip-tests) SKIP_TESTS=1 ;;
  esac
done

# ── Helper: màu sắc terminal (tắt nếu không có tput) ─────────────────────────
if command -v tput >/dev/null 2>&1 && tput setaf 1 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)";  YELLOW="$(tput setaf 3)"
  RED="$(tput setaf 1)";    CYAN="$(tput setaf 6)"
  BOLD="$(tput bold)";      RESET="$(tput sgr0)"
else
  GREEN=""; YELLOW=""; RED=""; CYAN=""; BOLD=""; RESET=""
fi

log_step()  { printf "\n${BOLD}${CYAN}==> %s${RESET}\n" "$1"; }
log_ok()    { printf "${GREEN}    ✓ %s${RESET}\n" "$1"; }
log_warn()  { printf "${YELLOW}    ⚠ %s${RESET}\n" "$1"; }
log_error() { printf "${RED}    ✗ %s${RESET}\n" "$1"; }
log_info()  { printf "    %s\n" "$1"; }

# ── SSH helper (SSHPASS + sshpass khi không dùng key) ─────────────────────────
ssh_cmd() {
  if [ -n "${SSHPASS:-}" ] && command -v sshpass >/dev/null 2>&1; then
    sshpass -e ssh -p "$DEV_PORT" -o StrictHostKeyChecking=accept-new "${DEV_USER}@${DEV_HOST}" "$@"
  else
    ssh -p "$DEV_PORT" "${DEV_USER}@${DEV_HOST}" "$@"
  fi
}

scp_cmd() {
  if [ -n "${SSHPASS:-}" ] && command -v sshpass >/dev/null 2>&1; then
    sshpass -e scp -P "$DEV_PORT" -o StrictHostKeyChecking=accept-new "$@"
  else
    scp -P "$DEV_PORT" "$@"
  fi
}

ssh_exec() {
  ssh_cmd "$@"
}

sync_migrations_dir() {
  log_step "Migrations: đồng bộ thư mục migrations lên dev server"
  ssh_exec "mkdir -p ${DEV_PATH} && rm -rf ${DEV_PATH}/migrations"
  scp_cmd -r "$MIGRATIONS_DIR" "${DEV_USER}@${DEV_HOST}:${DEV_PATH}/"
  log_ok "Đã đồng bộ migrations/"
}

read_applied_migrations() {
  ssh_exec \
    "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse 'SELECT file_name FROM schema_migrations'"
}

# =============================================================================
# BƯỚC 1: Pre-flight — kiểm tra SSH
# =============================================================================
preflight_check() {
  log_step "Pre-flight: kiểm tra kết nối SSH tới dev server"
  log_info "SSH có thể yêu cầu nhập password nếu SSH key chưa sẵn sàng"

  if ! ssh_cmd -o ConnectTimeout=8 "echo ok" >/dev/null 2>&1; then
    log_error "Không kết nối được SSH tới ${DEV_USER}@${DEV_HOST}:${DEV_PORT}"
    printf "    Kiểm tra:\n"
    printf "      1. Password hoặc SSH key có đúng không?\n"
    printf "      2. Dev server có đang chạy không?\n"
    printf "      3. Thử thủ công:  ssh -p %s %s@%s\n" \
           "$DEV_PORT" "$DEV_USER" "$DEV_HOST"
    exit 1
  fi
  log_ok "SSH OK — ${DEV_USER}@${DEV_HOST}:${DEV_PORT}"
}

# =============================================================================
# BƯỚC 2: Pre-deploy checks (lint / build test) — bỏ qua nếu --skip-tests
# =============================================================================
precheck_be() {
  if [ "$SKIP_TESTS" -eq 1 ]; then
    log_warn "Bỏ qua BE checks (--skip-tests)"
    return
  fi
  log_step "BE pre-check: go build ./..."
  cd "$MAKEFILE_DIR"
  if ! go build ./...; then
    log_error "go build thất bại — không deploy BE"
    exit 1
  fi
  log_ok "go build OK"

  # go test là optional vì chậm — uncomment nếu muốn bắt buộc
  # log_info "Chạy go test ./... (dùng --skip-tests để bỏ qua)"
  # go test ./... || { log_error "go test thất bại"; exit 1; }
  # log_ok "go test OK"
}

precheck_fe() {
  FE_DIR="$MAKEFILE_DIR/../cobo_web_design"
  if [ "$SKIP_TESTS" -eq 1 ]; then
    log_warn "Bỏ qua FE checks (--skip-tests)"
    return
  fi
  log_step "FE pre-check: npm run lint (tsc --noEmit)"
  cd "$FE_DIR"
  if ! npm run lint; then
    log_error "FE lint thất bại — không deploy FE"
    exit 1
  fi
  log_ok "npm run lint OK"
  cd "$MAKEFILE_DIR"
}

# =============================================================================
# BƯỚC 3: Phát hiện và push migrations mới
# =============================================================================
deploy_migrations() {
  log_step "Migrations: so sánh local vs server"
  sync_migrations_dir
  bootstrap_recovered=0

  # Lấy danh sách migrations đã apply trên server
  if ! applied="$(read_applied_migrations 2>&1)"; then
    log_error "Không lấy được schema_migrations từ server."
    case "$applied" in
      *"Permission denied"*|*"Host key verification failed"*)
        log_error "SSH authentication thất bại khi truy vấn dev server."
        ;;
      *"No such container: cobo-iam-mysql"*)
        log_error "Container MySQL 'cobo-iam-mysql' không tồn tại trên dev server."
        ;;
      *"is not running"*|*"Container "*)
        log_error "Container MySQL trên dev server không chạy ổn định."
        ;;
      *"Access denied for user"*)
        log_error "MySQL từ chối credentials hiện tại."
        ;;
      *"Unknown database 'cobo_iam'"*)
        log_error "Database 'cobo_iam' chưa tồn tại trên dev server."
        ;;
      *"Table 'cobo_iam.schema_migrations' doesn't exist"*)
        log_warn "Bảng 'schema_migrations' chưa tồn tại trong database 'cobo_iam'."
        log_info "Đang chạy bootstrap migration runner trên dev server..."
        if ! ssh_exec "cd ${DEV_PATH} && docker compose -f docker-compose.artifacts.yml run --rm migrate"; then
          log_error "Bootstrap migration runner thất bại."
          exit 1
        fi
        if ! applied="$(read_applied_migrations 2>&1)"; then
          log_error "Bootstrap đã chạy nhưng vẫn không đọc được schema_migrations."
          log_error "Chi tiết lỗi: ${applied:-không có output}"
          exit 1
        fi
        bootstrap_recovered=1
        ;;
      *"Can't connect to local MySQL server"*|*"Can't connect to MySQL server"*)
        log_error "MySQL không chấp nhận kết nối từ bên trong container dev."
        ;;
      *)
        log_error "Chi tiết lỗi: ${applied:-không có output}"
        ;;
    esac
    if [ "$bootstrap_recovered" -ne 1 ]; then
      log_error "Dừng deploy vì không xác định được trạng thái migration hiện tại trên dev server."
      log_info "Thử thủ công: ssh -p ${DEV_PORT} ${DEV_USER}@${DEV_HOST}"
      log_info "Sau đó chạy: docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse 'SELECT file_name FROM schema_migrations'"
      exit 1
    fi
  fi

  # Lấy danh sách migrations local từ run_dev_migrations.sh
  # Lấy các dòng trong block MIGRATIONS="..." — chỉ dòng chứa .sql, loại comment
  local_list="$(awk '/^MIGRATIONS="/{p=1;next} /^"/{if(p)p=0} p{gsub(/[[:space:]]/,""); if(/\.sql$/) print}' \
    "$MIGRATIONS_DIR/run_dev_migrations.sh")"

  new_count=0
  new_list=""
  for f in $local_list; do
    if ! echo "$applied" | grep -qF "$f"; then
      new_list="$new_list $f"
      new_count=$((new_count + 1))
    fi
  done

  if [ "$new_count" -eq 0 ]; then
    log_ok "Không có migration mới — server đã up to date"
    return
  fi

  log_info "Phát hiện ${new_count} migration chưa apply:"
  for f in $new_list; do
    log_info "  - $f"
  done

  log_step "Đang push + apply ${new_count} migration(s)..."
  for f in $new_list; do
    log_info "Applying: $f"
    if sh "$PUSH_MIGRATION_SCRIPT" "$f"; then
      log_ok "$f applied"
    else
      log_error "$f FAILED — dừng migration. Kiểm tra log trên server."
      exit 1
    fi
  done
  log_ok "Tất cả migrations đã apply thành công"
}

# =============================================================================
# BƯỚC 4: Deploy BE (gọi make deploy-be như Makefile định nghĩa)
# =============================================================================
deploy_be() {
  log_step "Deploy BE: build Linux binary + SCP + restart api/worker"
  cd "$MAKEFILE_DIR"
  if ! make deploy-be; then
    log_error "make deploy-be thất bại"
    exit 1
  fi
  log_ok "BE deployed"
}

# =============================================================================
# BƯỚC 5: Deploy FE (gọi make deploy-fe như Makefile định nghĩa)
# =============================================================================
deploy_fe() {
  log_step "Deploy FE: npm run build + SCP dist + restart web"
  cd "$MAKEFILE_DIR"
  if ! make deploy-fe; then
    log_error "make deploy-fe thất bại"
    exit 1
  fi
  log_ok "FE deployed"
}

# =============================================================================
# BƯỚC 6: Verify sau deploy
# =============================================================================
verify() {
  log_step "Verify: trạng thái containers trên dev server"
  cd "$MAKEFILE_DIR"
  make dev-ps || log_warn "dev-ps thất bại (SSH issue?)"

  log_step "Verify: health check API"
  # Gọi healthz qua SSH để tránh cần mở port ra ngoài
  status="$(ssh_exec "curl -sf http://localhost:8080/healthz 2>/dev/null || echo FAIL")"
  if [ "$status" = '{"status":"ok"}' ] || echo "$status" | grep -q '"status":"ok"'; then
    log_ok "/healthz → ok"
  else
    log_warn "/healthz trả về: ${status:-không phản hồi}"
    log_warn "API có thể đang khởi động — kiểm tra lại sau 10-15s"
    log_info "  make dev-logs  (xem log realtime)"
  fi

  readyz="$(ssh_exec "curl -sf http://localhost:8080/readyz 2>/dev/null || echo FAIL")"
  if echo "$readyz" | grep -q '"status":"ready"'; then
    log_ok "/readyz → ready"
  else
    log_warn "/readyz: ${readyz:-không phản hồi}"
  fi

  log_step "Verify: nginx proxy /api (port 3000)"
  code300="$(ssh_exec "curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3000/api/v1/auth/login-password-key 2>/dev/null || echo 000")"
  if [ "$code300" = "200" ] || [ "$code300" = "204" ]; then
    log_ok "/api/v1/auth/login-password-key via :3000 → HTTP $code300"
  else
    log_warn "login-password-key via :3000 → HTTP ${code300:-000} (502 thường do API down hoặc nginx chưa proxy được)"
    log_info "Chạy trên server: sh ${DEV_PATH}/fix-dev-web-perms.sh"
    log_info "Hoặc local: make dev-fix-web-perms"
  fi
}

# =============================================================================
# MAIN — điều hướng theo mode
# =============================================================================
printf "\n${BOLD}deploy-dev.sh — Cobo IAM Dev Deploy${RESET}\n"
printf "Mode: ${YELLOW}%s${RESET}   Skip-tests: %s\n" \
  "$MODE" "$([ "$SKIP_TESTS" -eq 1 ] && echo yes || echo no)"
printf "Target: ${CYAN}%s@%s:%s → %s${RESET}\n\n" \
  "$DEV_USER" "$DEV_HOST" "$DEV_PORT" "$DEV_PATH"

case "$MODE" in
  be)
    preflight_check
    precheck_be
    deploy_be
    verify
    ;;

  fe)
    preflight_check
    precheck_fe
    deploy_fe
    verify
    ;;

  migrate)
    preflight_check
    deploy_migrations
    verify
    ;;

  all|"")
    preflight_check
    precheck_be
    precheck_fe
    deploy_migrations
    deploy_be
    deploy_fe
    verify
    ;;

  verify)
    preflight_check
    verify
    ;;

  *)
    printf "${RED}Mode không hợp lệ: '%s'${RESET}\n" "$MODE"
    printf "Dùng: be | fe | all | migrate | verify\n"
    exit 1
    ;;
esac

printf "\n${BOLD}${GREEN}==> Deploy hoàn tất.${RESET}\n"
printf "    Xem log realtime:  ${CYAN}make dev-logs${RESET}  (từ thư mục cobo_iam_services/)\n"
printf "    SSH vào server:    ${CYAN}make dev-ssh${RESET}\n\n"
