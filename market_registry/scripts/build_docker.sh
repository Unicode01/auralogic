#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REGISTRY_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK_DIR="$REGISTRY_ROOT"
DOCKER_TEMPLATE_DIR="$SCRIPT_DIR/docker"
DOCKER_BUILD_DIR="$WORK_DIR/docker-build"
IMAGE_NAME="auralogic-market-registry"
CONTAINER_NAME="auralogic-market-registry"
CONTAINER_PORT="18080"
DATA_ROOT="/app/data"
DATA_DIR="${DATA_ROOT}/data"
KEY_DIR="${DATA_ROOT}/keys"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
IMAGE_TAG="${TIMESTAMP}"
LAST_CONFIG_FILE="$WORK_DIR/.docker_last_config"
ENV_FILE="$DOCKER_BUILD_DIR/registry.env"
COMPOSE_FILE="$WORK_DIR/docker-compose.yml"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $1"; }
ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()   { echo -e "${RED}[ERROR]${NC} $1"; }
step()  { echo -e "\n${BLUE}==>${NC} ${BLUE}$1${NC}\n"; }

load_last_config() {
  if [ -f "$LAST_CONFIG_FILE" ]; then
    # shellcheck disable=SC1090
    source "$LAST_CONFIG_FILE"
    info "已加载上次构建配置 ($LAST_CONFIG_FILE)"
  fi
}

save_config() {
  local file="$LAST_CONFIG_FILE"
  : > "$file"
  _sv() { printf '%s=%q\n' "$1" "$2" >> "$file"; }

  _sv LAST_IMAGE_NAME "$IMAGE_NAME"
  _sv LAST_CONTAINER_NAME "$CONTAINER_NAME"
  _sv LAST_BASE_URL "$BASE_URL"
  _sv LAST_EXPOSE_PORT "$EXPOSE_PORT"
  _sv LAST_SOURCE_ID "$SOURCE_ID"
  _sv LAST_SOURCE_NAME "$SOURCE_NAME"
  _sv LAST_KEY_ID "$KEY_ID"
  _sv LAST_CHANNEL "$CHANNEL"
  _sv LAST_STORAGE_TYPE "$STORAGE_TYPE"
  _sv LAST_STORAGE_S3_ENDPOINT "$STORAGE_S3_ENDPOINT"
  _sv LAST_STORAGE_S3_REGION "$STORAGE_S3_REGION"
  _sv LAST_STORAGE_S3_BUCKET "$STORAGE_S3_BUCKET"
  _sv LAST_STORAGE_S3_PREFIX "$STORAGE_S3_PREFIX"
  _sv LAST_STORAGE_S3_USE_PATH_STYLE "$STORAGE_S3_USE_PATH_STYLE"
  _sv LAST_STORAGE_S3_ACCESS_KEY_ID "$STORAGE_S3_ACCESS_KEY_ID"
  _sv LAST_ADMIN_USERNAME "$ADMIN_USERNAME"
  _sv LAST_AUTH_TOKEN_TTL "$AUTH_TOKEN_TTL"

  unset -f _sv
  ok "构建配置已保存到 $LAST_CONFIG_FILE"
}

docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
    return
  fi
  err "未检测到 docker compose / docker-compose"
  exit 1
}

check_deps() {
  step "检查依赖工具"

  if ! command -v docker >/dev/null 2>&1; then
    err "未检测到 Docker，请先安装 Docker"
    exit 1
  fi

  if ! docker info >/dev/null 2>&1; then
    err "Docker 未运行，请先启动 Docker 服务"
    exit 1
  fi

  if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
    err "未检测到 docker compose / docker-compose"
    exit 1
  fi

  if [ ! -d "$WORK_DIR/admin" ] || [ ! -d "$WORK_DIR/cmd" ] || [ ! -f "$WORK_DIR/go.mod" ]; then
    err "未检测到完整的 market_registry 目录结构，请在 market_registry 下使用此脚本"
    exit 1
  fi

  ok "依赖检查通过"
}

write_env_line() {
  local key="$1"
  local value="${2-}"
  if [ -z "${value}" ]; then
    printf '%s=\n' "$key" >> "$ENV_FILE"
    return
  fi

  if [[ "$value" =~ ^[A-Za-z0-9._:/@%+=,-]+$ ]]; then
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
    return
  fi

  local escaped
  escaped=$(printf "%s" "$value" | sed "s/'/'\"'\"'/g")
  printf "%s='%s'\n" "$key" "$escaped" >> "$ENV_FILE"
}

load_env_file() {
  if [ ! -f "$ENV_FILE" ]; then
    err "未找到 $ENV_FILE，请先执行完整构建"
    exit 1
  fi

  # shellcheck disable=SC1090
  set -a && . "$ENV_FILE" && set +a
}

read_basic_config() {
  step "基础配置"

  local def_base_url="${LAST_BASE_URL:-http://localhost:18080}"
  local def_port="${LAST_EXPOSE_PORT:-18080}"
  local def_source_id="${LAST_SOURCE_ID:-official}"
  local def_source_name="${LAST_SOURCE_NAME:-AuraLogic Official Source}"
  local def_key_id="${LAST_KEY_ID:-official-2026-01}"
  local def_channel="${LAST_CHANNEL:-stable}"
  local def_admin_username="${LAST_ADMIN_USERNAME:-admin}"
  local def_auth_token_ttl="${LAST_AUTH_TOKEN_TTL:-12h}"

  read -rp "$(echo -e "${CYAN}Registry 对外访问 URL${NC} [${def_base_url}]: ")" BASE_URL
  BASE_URL="${BASE_URL:-$def_base_url}"

  read -rp "$(echo -e "${CYAN}Docker 对外映射端口${NC} [${def_port}]: ")" EXPOSE_PORT
  EXPOSE_PORT="${EXPOSE_PORT:-$def_port}"

  read -rp "$(echo -e "${CYAN}市场源 ID${NC} [${def_source_id}]: ")" SOURCE_ID
  SOURCE_ID="${SOURCE_ID:-$def_source_id}"

  read -rp "$(echo -e "${CYAN}市场源名称${NC} [${def_source_name}]: ")" SOURCE_NAME
  SOURCE_NAME="${SOURCE_NAME:-$def_source_name}"

  read -rp "$(echo -e "${CYAN}签名 Key ID${NC} [${def_key_id}]: ")" KEY_ID
  KEY_ID="${KEY_ID:-$def_key_id}"

  read -rp "$(echo -e "${CYAN}默认发布渠道${NC} [${def_channel}]: ")" CHANNEL
  CHANNEL="${CHANNEL:-$def_channel}"

  read -rp "$(echo -e "${CYAN}管理员用户名${NC} [${def_admin_username}]: ")" ADMIN_USERNAME
  ADMIN_USERNAME="${ADMIN_USERNAME:-$def_admin_username}"

  read -rp "$(echo -e "${CYAN}管理员 Token TTL${NC} [${def_auth_token_ttl}]: ")" AUTH_TOKEN_TTL
  AUTH_TOKEN_TTL="${AUTH_TOKEN_TTL:-$def_auth_token_ttl}"

  read -rsp "$(echo -e "${CYAN}管理员密码${NC}: ")" ADMIN_PASSWORD
  echo ""
  if [ -z "$ADMIN_PASSWORD" ]; then
    err "管理员密码不能为空"
    exit 1
  fi

  read -rsp "$(echo -e "${CYAN}确认管理员密码${NC}: ")" ADMIN_PASSWORD_CONFIRM
  echo ""
  if [ "$ADMIN_PASSWORD" != "$ADMIN_PASSWORD_CONFIRM" ]; then
    err "两次输入的管理员密码不一致"
    exit 1
  fi

  read -rp "$(echo -e "${CYAN}Docker 镜像 Tag${NC} [${IMAGE_TAG}]: ")" INPUT_TAG
  IMAGE_TAG="${INPUT_TAG:-$IMAGE_TAG}"
}

read_storage_config() {
  step "存储配置"

  local def_storage_type="${LAST_STORAGE_TYPE:-local}"
  local def_storage_choice="1"
  if [ "$def_storage_type" = "s3" ]; then
    def_storage_choice="2"
  fi

  echo "选择 canonical storage backend:"
  echo "  1) local (默认，本地卷持久化)"
  echo "  2) s3 / r2"
  read -rp "请选择 [${def_storage_choice}]: " STORAGE_CHOICE
  STORAGE_CHOICE="${STORAGE_CHOICE:-$def_storage_choice}"

  case "$STORAGE_CHOICE" in
    1)
      STORAGE_TYPE="local"
      STORAGE_S3_ENDPOINT=""
      STORAGE_S3_REGION=""
      STORAGE_S3_BUCKET=""
      STORAGE_S3_PREFIX=""
      STORAGE_S3_ACCESS_KEY_ID=""
      STORAGE_S3_SECRET_ACCESS_KEY=""
      STORAGE_S3_SESSION_TOKEN=""
      STORAGE_S3_USE_PATH_STYLE="false"
      ;;
    2)
      STORAGE_TYPE="s3"
      read -rp "$(echo -e "${CYAN}S3 / R2 Endpoint${NC} [${LAST_STORAGE_S3_ENDPOINT:-https://example.r2.cloudflarestorage.com}]: ")" STORAGE_S3_ENDPOINT
      STORAGE_S3_ENDPOINT="${STORAGE_S3_ENDPOINT:-${LAST_STORAGE_S3_ENDPOINT:-https://example.r2.cloudflarestorage.com}}"

      read -rp "$(echo -e "${CYAN}S3 Region${NC} [${LAST_STORAGE_S3_REGION:-auto}]: ")" STORAGE_S3_REGION
      STORAGE_S3_REGION="${STORAGE_S3_REGION:-${LAST_STORAGE_S3_REGION:-auto}}"

      read -rp "$(echo -e "${CYAN}S3 Bucket${NC} [${LAST_STORAGE_S3_BUCKET:-market-artifacts}]: ")" STORAGE_S3_BUCKET
      STORAGE_S3_BUCKET="${STORAGE_S3_BUCKET:-${LAST_STORAGE_S3_BUCKET:-market-artifacts}}"

      read -rp "$(echo -e "${CYAN}S3 Prefix${NC} [${LAST_STORAGE_S3_PREFIX:-registry/prod}]: ")" STORAGE_S3_PREFIX
      STORAGE_S3_PREFIX="${STORAGE_S3_PREFIX:-${LAST_STORAGE_S3_PREFIX:-registry/prod}}"

      read -rp "$(echo -e "${CYAN}S3 Access Key ID${NC} [${LAST_STORAGE_S3_ACCESS_KEY_ID:-}]: ")" STORAGE_S3_ACCESS_KEY_ID
      STORAGE_S3_ACCESS_KEY_ID="${STORAGE_S3_ACCESS_KEY_ID:-${LAST_STORAGE_S3_ACCESS_KEY_ID:-}}"

      read -rsp "$(echo -e "${CYAN}S3 Secret Access Key${NC}: ")" STORAGE_S3_SECRET_ACCESS_KEY
      echo ""
      if [ -z "$STORAGE_S3_SECRET_ACCESS_KEY" ]; then
        err "S3 Secret Access Key 不能为空"
        exit 1
      fi

      read -rp "$(echo -e "${CYAN}S3 Session Token${NC} [留空表示不使用]: ")" STORAGE_S3_SESSION_TOKEN
      STORAGE_S3_SESSION_TOKEN="${STORAGE_S3_SESSION_TOKEN:-}"

      local def_use_path_style="${LAST_STORAGE_S3_USE_PATH_STYLE:-false}"
      local path_style_hint="y/N"
      [ "$def_use_path_style" = "true" ] && path_style_hint="Y/n"
      read -rp "$(echo -e "${CYAN}启用 S3 Path Style?${NC} [${path_style_hint}]: ")" STORAGE_PATH_STYLE_INPUT
      if [ -z "$STORAGE_PATH_STYLE_INPUT" ]; then
        STORAGE_S3_USE_PATH_STYLE="$def_use_path_style"
      elif [[ "$STORAGE_PATH_STYLE_INPUT" =~ ^[Yy]$ ]]; then
        STORAGE_S3_USE_PATH_STYLE="true"
      else
        STORAGE_S3_USE_PATH_STYLE="false"
      fi
      ;;
    *)
      err "无效选择"
      exit 1
      ;;
  esac
}

confirm_config() {
  step "确认配置"

  echo "镜像名称:         ${IMAGE_NAME}:${IMAGE_TAG}"
  echo "容器名称:         ${CONTAINER_NAME}"
  echo "映射端口:         ${EXPOSE_PORT}:${CONTAINER_PORT}"
  echo "对外 URL:         ${BASE_URL}"
  echo "源 ID / 名称:     ${SOURCE_ID} / ${SOURCE_NAME}"
  echo "签名 Key ID:      ${KEY_ID}"
  echo "默认渠道:         ${CHANNEL}"
  echo "存储后端:         ${STORAGE_TYPE}"
  if [ "$STORAGE_TYPE" = "s3" ]; then
    echo "S3 Endpoint:      ${STORAGE_S3_ENDPOINT}"
    echo "S3 Region:        ${STORAGE_S3_REGION}"
    echo "S3 Bucket:        ${STORAGE_S3_BUCKET}"
    echo "S3 Prefix:        ${STORAGE_S3_PREFIX}"
    echo "S3 Path Style:    ${STORAGE_S3_USE_PATH_STYLE}"
    echo "S3 Access Key ID: ${STORAGE_S3_ACCESS_KEY_ID}"
  else
    echo "本地数据卷:       market_registry_runtime"
  fi
  echo "管理员用户名:     ${ADMIN_USERNAME}"
  echo "Token TTL:        ${AUTH_TOKEN_TTL}"
  echo ""

  read -rp "$(echo -e "${YELLOW}确认继续?${NC} [Y/n]: ")" CONFIRM
  if [[ "$CONFIRM" =~ ^[Nn]$ ]]; then
    info "已取消"
    exit 0
  fi
}

write_env_file() {
  step "生成运行时环境文件"

  mkdir -p "$DOCKER_BUILD_DIR"
  : > "$ENV_FILE"

  write_env_line "MARKET_REGISTRY_ADDR" ":${CONTAINER_PORT}"
  write_env_line "MARKET_REGISTRY_BASE_URL" "$BASE_URL"
  write_env_line "MARKET_REGISTRY_DATA_DIR" "$DATA_DIR"
  write_env_line "MARKET_REGISTRY_KEY_DIR" "$KEY_DIR"
  write_env_line "MARKET_REGISTRY_KEY_ID" "$KEY_ID"
  write_env_line "MARKET_REGISTRY_ID" "$SOURCE_ID"
  write_env_line "MARKET_REGISTRY_NAME" "$SOURCE_NAME"
  write_env_line "MARKET_REGISTRY_CHANNEL" "$CHANNEL"
  write_env_line "MARKET_REGISTRY_STORAGE_TYPE" "$STORAGE_TYPE"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_ENDPOINT" "$STORAGE_S3_ENDPOINT"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_REGION" "$STORAGE_S3_REGION"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_BUCKET" "$STORAGE_S3_BUCKET"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_PREFIX" "$STORAGE_S3_PREFIX"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID" "$STORAGE_S3_ACCESS_KEY_ID"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY" "$STORAGE_S3_SECRET_ACCESS_KEY"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN" "$STORAGE_S3_SESSION_TOKEN"
  write_env_line "MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE" "$STORAGE_S3_USE_PATH_STYLE"
  write_env_line "MARKET_REGISTRY_ADMIN_USERNAME" "$ADMIN_USERNAME"
  write_env_line "MARKET_REGISTRY_ADMIN_PASSWORD" "$ADMIN_PASSWORD"
  write_env_line "MARKET_REGISTRY_AUTH_TOKEN_TTL" "$AUTH_TOKEN_TTL"

  chmod 600 "$ENV_FILE"
  ok "运行时环境文件已生成: $ENV_FILE"
}

copy_docker_files() {
  step "准备 Docker 构建文件"

  mkdir -p "$DOCKER_BUILD_DIR"
  cp "$DOCKER_TEMPLATE_DIR/Dockerfile" "$DOCKER_BUILD_DIR/Dockerfile"
  cp "$DOCKER_TEMPLATE_DIR/entrypoint.sh" "$DOCKER_BUILD_DIR/entrypoint.sh"
  chmod +x "$DOCKER_BUILD_DIR/entrypoint.sh"

  ok "Docker 构建文件已准备"
}

build_image() {
  step "构建 Docker 镜像"

  local git_commit
  git_commit="$(git -C "$WORK_DIR" rev-parse --short HEAD 2>/dev/null || echo "dev")"

  docker build \
    --build-arg BUILD_VERSION="$git_commit" \
    -f "$DOCKER_BUILD_DIR/Dockerfile" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -t "${IMAGE_NAME}:latest" \
    "$WORK_DIR"

  ok "Docker 镜像构建完成: ${IMAGE_NAME}:${IMAGE_TAG}"
}

generate_compose() {
  step "生成 docker-compose.yml"

  cat > "$COMPOSE_FILE" <<EOF
services:
  market-registry:
    image: ${IMAGE_NAME}:${IMAGE_TAG}
    container_name: ${CONTAINER_NAME}
    ports:
      - "${EXPOSE_PORT}:${CONTAINER_PORT}"
    env_file:
      - ./docker-build/registry.env
    volumes:
      - market_registry_runtime:/app/data
    restart: unless-stopped

volumes:
  market_registry_runtime:
EOF

  ok "docker-compose.yml 已生成"
}

start_stack() {
  step "启动 / 更新容器"
  cd "$WORK_DIR"
  docker_compose up -d --force-recreate
  ok "容器已启动 / 更新"
}

cleanup_old_images() {
  echo ""
  read -rp "$(echo -e "${CYAN}是否清理旧版本镜像?${NC} [y/N]: ")" CLEAN_CONFIRM
  if [[ ! "$CLEAN_CONFIRM" =~ ^[Yy]$ ]]; then
    return
  fi

  docker images "${IMAGE_NAME}" --format "{{.Tag}}" | grep -v "^${IMAGE_TAG}$" | grep -v "^latest$" | while read -r old_tag; do
    [ -z "$old_tag" ] && continue
    docker rmi "${IMAGE_NAME}:${old_tag}" >/dev/null 2>&1 || true
    info "已删除: ${IMAGE_NAME}:${old_tag}"
  done
  ok "旧镜像清理完成"
}

summary() {
  step "完成"

  echo "=========================================="
  echo -e "${GREEN}✓ Market Registry Docker 部署已完成!${NC}"
  echo "=========================================="
  echo "镜像:            ${IMAGE_NAME}:${IMAGE_TAG}"
  echo "管理后台:        ${BASE_URL%/}/admin/ui/"
  echo "市场 API:        ${BASE_URL%/}/v1/source.json"
  echo "compose 文件:    ${COMPOSE_FILE}"
  echo "运行时 env:      ${ENV_FILE}"
  echo "数据卷:          market_registry_runtime"
  echo "签名 Key ID:     ${KEY_ID}"
  echo "=========================================="
}

prepare_update_context() {
  load_last_config

  if [ ! -f "$ENV_FILE" ] || [ ! -f "$COMPOSE_FILE" ]; then
    err "未找到现有 docker-build/registry.env 或 docker-compose.yml，请先执行完整构建"
    exit 1
  fi

  load_env_file

  BASE_URL="${MARKET_REGISTRY_BASE_URL:-${LAST_BASE_URL:-http://localhost:18080}}"
  EXPOSE_PORT="${LAST_EXPOSE_PORT:-18080}"
  SOURCE_ID="${MARKET_REGISTRY_ID:-${LAST_SOURCE_ID:-official}}"
  SOURCE_NAME="${MARKET_REGISTRY_NAME:-${LAST_SOURCE_NAME:-AuraLogic Official Source}}"
  KEY_ID="${MARKET_REGISTRY_KEY_ID:-${LAST_KEY_ID:-official-2026-01}}"
  CHANNEL="${MARKET_REGISTRY_CHANNEL:-${LAST_CHANNEL:-stable}}"
  STORAGE_TYPE="${MARKET_REGISTRY_STORAGE_TYPE:-${LAST_STORAGE_TYPE:-local}}"
  STORAGE_S3_ENDPOINT="${MARKET_REGISTRY_STORAGE_S3_ENDPOINT:-${LAST_STORAGE_S3_ENDPOINT:-}}"
  STORAGE_S3_REGION="${MARKET_REGISTRY_STORAGE_S3_REGION:-${LAST_STORAGE_S3_REGION:-}}"
  STORAGE_S3_BUCKET="${MARKET_REGISTRY_STORAGE_S3_BUCKET:-${LAST_STORAGE_S3_BUCKET:-}}"
  STORAGE_S3_PREFIX="${MARKET_REGISTRY_STORAGE_S3_PREFIX:-${LAST_STORAGE_S3_PREFIX:-}}"
  STORAGE_S3_ACCESS_KEY_ID="${MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID:-${LAST_STORAGE_S3_ACCESS_KEY_ID:-}}"
  STORAGE_S3_SECRET_ACCESS_KEY="${MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY:-}"
  STORAGE_S3_SESSION_TOKEN="${MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN:-}"
  STORAGE_S3_USE_PATH_STYLE="${MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE:-${LAST_STORAGE_S3_USE_PATH_STYLE:-false}}"
  ADMIN_USERNAME="${MARKET_REGISTRY_ADMIN_USERNAME:-${LAST_ADMIN_USERNAME:-admin}}"
  ADMIN_PASSWORD="${MARKET_REGISTRY_ADMIN_PASSWORD:-}"
  AUTH_TOKEN_TTL="${MARKET_REGISTRY_AUTH_TOKEN_TTL:-${LAST_AUTH_TOKEN_TTL:-12h}}"

  if [ -z "$ADMIN_PASSWORD" ]; then
    err "现有环境文件中缺少 MARKET_REGISTRY_ADMIN_PASSWORD，无法执行 update"
    exit 1
  fi

  info "使用现有部署配置:"
  info "  对外 URL:  $BASE_URL"
  info "  映射端口:  $EXPOSE_PORT"
  info "  存储后端:  $STORAGE_TYPE"
  info "  管理后台:  ${BASE_URL%/}/admin/ui/"
  echo ""

  read -rp "$(echo -e "${YELLOW}确认重新编译并更新容器?${NC} [Y/n]: ")" CONFIRM
  if [[ "$CONFIRM" =~ ^[Nn]$ ]]; then
    info "已取消"
    exit 0
  fi
}

full_build() {
  load_last_config
  check_deps
  read_basic_config
  read_storage_config
  confirm_config
  write_env_file
  copy_docker_files
  build_image
  generate_compose
  save_config
  start_stack
  cleanup_old_images
  summary
}

update_container() {
  check_deps
  prepare_update_context
  IMAGE_TAG="$TIMESTAMP"
  copy_docker_files
  write_env_file
  build_image
  generate_compose
  save_config
  start_stack
  cleanup_old_images
  summary
}

main() {
  echo ""
  echo "=========================================="
  echo "  AuraLogic Market Registry Docker 构建脚本"
  echo "=========================================="
  echo ""

  local action="${1:-}"
  if [ -z "$action" ]; then
    echo "请选择操作:"
    echo "  1) 完整构建 (首次部署/重新配置)"
    echo "  2) 更新容器 (重新编译并更新现有容器)"
    read -rp "请选择 [1]: " ACTION_CHOICE
    ACTION_CHOICE="${ACTION_CHOICE:-1}"
    case "$ACTION_CHOICE" in
      1) action="build" ;;
      2) action="update" ;;
      *) err "无效选择"; exit 1 ;;
    esac
  fi

  case "$action" in
    build)
      full_build
      ;;
    update)
      update_container
      ;;
    *)
      err "未知操作: $action"
      echo "用法: $0 [build|update]"
      exit 1
      ;;
  esac
}

main "$@"
