#!/bin/bash

set -e

# ============================================
# AuraLogic 一键编译 & All-in-One Docker 镜像构建脚本
# 包含: Nginx + Frontend + Backend
# 功能: 交互式配置生成、数据库初始化、管理员创建
# ============================================

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK_DIR="$PROJECT_ROOT"
IMAGE_NAME="auralogic-allinone"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
IMAGE_TAG="${TIMESTAMP}"
LAST_CONFIG_FILE="$PROJECT_ROOT/.build_last_config"

# 颜色输出
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

# ---------------------------
# 加载上次构建配置
# ---------------------------
load_last_config() {
  if [ -f "$LAST_CONFIG_FILE" ]; then
    source "$LAST_CONFIG_FILE"
    info "已加载上次构建配置 ($LAST_CONFIG_FILE)"
  fi
}

# ---------------------------
# 保存本次构建配置 (不保存密码)
# ---------------------------
save_config() {
  local f="$LAST_CONFIG_FILE"
  : > "$f"
  _sv() { printf '%s=%q\n' "$1" "$2" >> "$f"; }

  # 基本配置
  _sv LAST_APP_NAME "$APP_NAME"
  _sv LAST_APP_URL "$APP_URL"
  _sv LAST_EXPOSE_PORT "$EXPOSE_PORT"
  _sv LAST_APP_ENV "$APP_ENV"
  _sv LAST_APP_DEBUG "$APP_DEBUG"

  # 数据库 (不保存密码)
  _sv LAST_DB_DRIVER "$DB_DRIVER"
  _sv LAST_DB_HOST "$DB_HOST"
  _sv LAST_DB_PORT "$DB_PORT"
  _sv LAST_DB_NAME "$DB_NAME"
  _sv LAST_DB_USER "$DB_USER"
  _sv LAST_DB_SSL_MODE "$DB_SSL_MODE"

  # JWT (不保存 secret)
  _sv LAST_JWT_EXPIRE "$JWT_EXPIRE"
  _sv LAST_JWT_REFRESH_EXPIRE "$JWT_REFRESH_EXPIRE"

  # OAuth (不保存 secret)
  _sv LAST_GITHUB_ENABLED "$GITHUB_ENABLED"
  _sv LAST_GITHUB_CLIENT_ID "$GITHUB_CLIENT_ID"
  _sv LAST_GOOGLE_ENABLED "$GOOGLE_ENABLED"
  _sv LAST_GOOGLE_CLIENT_ID "$GOOGLE_CLIENT_ID"

  # SMTP (不保存密码)
  _sv LAST_SMTP_ENABLED "$SMTP_ENABLED"
  _sv LAST_SMTP_HOST "$SMTP_HOST"
  _sv LAST_SMTP_PORT "$SMTP_PORT"
  _sv LAST_SMTP_USER "$SMTP_USER"
  _sv LAST_SMTP_FROM_EMAIL "$SMTP_FROM_EMAIL"
  _sv LAST_SMTP_FROM_NAME "$SMTP_FROM_NAME"

  # SMS (不保存密钥)
  _sv LAST_SMS_ENABLED "$SMS_ENABLED"
  _sv LAST_SMS_PROVIDER "$SMS_PROVIDER"

  # 管理员 (不保存密码)
  _sv LAST_ADMIN_EMAIL "$ADMIN_EMAIL"
  _sv LAST_ADMIN_NAME "$ADMIN_NAME"

  unset -f _sv
  ok "构建配置已保存到 $LAST_CONFIG_FILE"
}

# ---------------------------
# 检测包管理器
# ---------------------------
detect_pkg_manager() {
  if command -v apt-get &>/dev/null; then
    PKG_MANAGER="apt"
  elif command -v dnf &>/dev/null; then
    PKG_MANAGER="dnf"
  elif command -v yum &>/dev/null; then
    PKG_MANAGER="yum"
  elif command -v pacman &>/dev/null; then
    PKG_MANAGER="pacman"
  elif command -v apk &>/dev/null; then
    PKG_MANAGER="apk"
  elif command -v brew &>/dev/null; then
    PKG_MANAGER="brew"
  elif command -v zypper &>/dev/null; then
    PKG_MANAGER="zypper"
  else
    PKG_MANAGER=""
  fi
}

# ---------------------------
# 安装单个依赖
# ---------------------------
install_pkg() {
  local cmd="$1"
  local pkg="$2"

  if [ -z "$PKG_MANAGER" ]; then
    err "未检测到包管理器，请手动安装: $cmd"
    return 1
  fi

  info "正在安装 $cmd ..."

  case "$PKG_MANAGER" in
    apt)
      sudo apt-get update -qq && sudo apt-get install -y -qq "$pkg"
      ;;
    dnf)
      sudo dnf install -y -q "$pkg"
      ;;
    yum)
      sudo yum install -y -q "$pkg"
      ;;
    pacman)
      sudo pacman -S --noconfirm "$pkg"
      ;;
    apk)
      sudo apk add --no-cache "$pkg"
      ;;
    brew)
      brew install "$pkg"
      ;;
    zypper)
      sudo zypper install -y "$pkg"
      ;;
  esac
}

# ---------------------------
# 安装 Docker
# ---------------------------
install_docker() {
  info "正在安装 Docker ..."

  if [ "$PKG_MANAGER" = "brew" ]; then
    brew install --cask docker
    warn "请启动 Docker Desktop 后重新运行此脚本"
    exit 1
  fi

  # 使用官方脚本安装
  if command -v curl &>/dev/null; then
    curl -fsSL https://get.docker.com | sudo sh
  elif command -v wget &>/dev/null; then
    wget -qO- https://get.docker.com | sudo sh
  else
    err "需要 curl 或 wget 来安装 Docker，请手动安装"
    return 1
  fi

  # 将当前用户加入 docker 组
  if [ "$(id -u)" -ne 0 ]; then
    sudo usermod -aG docker "$USER"
    warn "已将 $USER 加入 docker 组，可能需要重新登录后生效"
  fi

  # 启动 Docker 服务
  if command -v systemctl &>/dev/null; then
    sudo systemctl start docker
    sudo systemctl enable docker
  fi
}

# ---------------------------
# 检查并安装依赖
# ---------------------------
check_deps() {
  step "检查依赖工具"

  detect_pkg_manager

  # 依赖列表: 命令名 -> 包名
  local need_install=false

  # 检查 docker
  if ! command -v docker &>/dev/null; then
    warn "未检测到 docker"
    read -rp "$(echo -e "${CYAN}是否自动安装 Docker?${NC} [Y/n]: ")" install_confirm
    if [[ "$install_confirm" =~ ^[Nn]$ ]]; then
      err "Docker 是必须依赖，无法继续"
      exit 1
    fi
    install_docker
    # 再次检查
    if ! command -v docker &>/dev/null; then
      err "Docker 安装失败，请手动安装后重试"
      exit 1
    fi
    ok "Docker 安装成功"
  else
    ok "Docker 已安装"
  fi

  # 检查 openssl (用于生成 JWT Secret)
  if ! command -v openssl &>/dev/null; then
    warn "未检测到 openssl"
    install_pkg "openssl" "openssl" || {
      warn "openssl 安装失败，将使用备用方式生成密钥"
    }
  fi

  # 检查 Docker 是否正在运行
  if ! docker info &>/dev/null 2>&1; then
    warn "Docker 未运行"
    if command -v systemctl &>/dev/null; then
      info "尝试启动 Docker 服务..."
      sudo systemctl start docker
      sleep 2
    fi
    if ! docker info &>/dev/null 2>&1; then
      err "Docker 未运行，请先启动 Docker 服务"
      exit 1
    fi
  fi
  ok "Docker 运行中"

  # 检查项目目录结构
  if [ ! -d "$PROJECT_ROOT/backend" ] || [ ! -d "$PROJECT_ROOT/frontend" ]; then
    err "未找到 backend/ 或 frontend/ 目录，请在项目根目录下运行此脚本"
    exit 1
  fi

  ok "依赖检查全部通过"
}

# ---------------------------
# 生成随机密钥
# ---------------------------
generate_secret() {
  if command -v openssl &>/dev/null; then
    openssl rand -base64 32 | tr -d "=+/" | cut -c1-32
  elif [ -r /dev/urandom ]; then
    head -c 32 /dev/urandom | base64 | tr -d "=+/" | cut -c1-32
  else
    # 最终备用: 用时间戳 + 随机数拼接
    echo "$(date +%s%N)${RANDOM}${RANDOM}" | sha256sum | cut -c1-32
  fi
}

# ---------------------------
# 用户输入 - 基本配置
# ---------------------------
read_basic_config() {
  step "基本配置"

  local def_name="${LAST_APP_NAME:-AuraLogic}"
  local def_url="${LAST_APP_URL:-https://auralogic.example.com}"
  local def_port="${LAST_EXPOSE_PORT:-80}"
  local def_env="${LAST_APP_ENV:-production}"
  local def_debug="${LAST_APP_DEBUG:-false}"

  # 应用配置
  read -rp "$(echo -e "${CYAN}应用名称${NC} [${def_name}]: ")" APP_NAME
  APP_NAME="${APP_NAME:-$def_name}"

  read -rp "$(echo -e "${CYAN}应用域名/URL${NC} [${def_url}]: ")" APP_URL
  APP_URL="${APP_URL:-$def_url}"

  # 容器内后端固定端口 8080（nginx 反向代理到此）
  APP_PORT=8080

  read -rp "$(echo -e "${CYAN}Docker 对外映射端口${NC} [${def_port}]: ")" EXPOSE_PORT
  EXPOSE_PORT="${EXPOSE_PORT:-$def_port}"

  read -rp "$(echo -e "${CYAN}环境${NC} [${def_env}]: ")" APP_ENV
  APP_ENV="${APP_ENV:-$def_env}"

  if [ "$def_debug" = "true" ]; then
    read -rp "$(echo -e "${CYAN}启用调试模式?${NC} [Y/n]: ")" debug_input
    if [[ "$debug_input" =~ ^[Nn]$ ]]; then
      APP_DEBUG="false"
    else
      APP_DEBUG="true"
    fi
  else
    read -rp "$(echo -e "${CYAN}启用调试模式?${NC} [y/N]: ")" debug_input
    if [[ "$debug_input" =~ ^[Yy]$ ]]; then
      APP_DEBUG="true"
    else
      APP_DEBUG="false"
    fi
  fi

  # Docker 镜像 tag
  read -rp "$(echo -e "${CYAN}Docker 镜像 Tag${NC} [${IMAGE_TAG}]: ")" input_tag
  IMAGE_TAG="${input_tag:-$IMAGE_TAG}"
}

# ---------------------------
# 用户输入 - 数据库配置
# ---------------------------
read_database_config() {
  step "数据库配置"

  # 根据上次选择设置默认值
  local def_choice="1"
  case "${LAST_DB_DRIVER}" in
    postgres) def_choice="2" ;;
    mysql)    def_choice="3" ;;
  esac

  echo "选择数据库类型:"
  echo "  1) SQLite (默认, 适合小型部署)"
  echo "  2) PostgreSQL (推荐生产环境)"
  echo "  3) MySQL"
  read -rp "请选择 [${def_choice}]: " db_choice
  db_choice="${db_choice:-$def_choice}"

  case "$db_choice" in
    1)
      DB_DRIVER="sqlite"
      DB_NAME="data/auralogic.db?_busy_timeout=5000&_journal_mode=WAL"
      DB_HOST=""
      DB_PORT=0
      DB_USER=""
      DB_PASSWORD=""
      DB_SSL_MODE=""
      ;;
    2)
      DB_DRIVER="postgres"
      local def_host="${LAST_DB_HOST:-localhost}"
      local def_port="${LAST_DB_PORT:-5432}"
      local def_name="${LAST_DB_NAME:-auralogic}"
      local def_user="${LAST_DB_USER:-postgres}"
      local def_ssl="${LAST_DB_SSL_MODE:-disable}"
      # 如果上次不是 postgres，用原始默认值
      if [ "${LAST_DB_DRIVER}" != "postgres" ]; then
        def_host="localhost"; def_port="5432"; def_name="auralogic"; def_user="postgres"; def_ssl="disable"
      fi
      read -rp "$(echo -e "${CYAN}PostgreSQL 主机${NC} [${def_host}]: ")" DB_HOST
      DB_HOST="${DB_HOST:-$def_host}"
      read -rp "$(echo -e "${CYAN}PostgreSQL 端口${NC} [${def_port}]: ")" DB_PORT
      DB_PORT="${DB_PORT:-$def_port}"
      read -rp "$(echo -e "${CYAN}数据库名${NC} [${def_name}]: ")" DB_NAME
      DB_NAME="${DB_NAME:-$def_name}"
      read -rp "$(echo -e "${CYAN}用户名${NC} [${def_user}]: ")" DB_USER
      DB_USER="${DB_USER:-$def_user}"
      read -rsp "$(echo -e "${CYAN}密码${NC}: ")" DB_PASSWORD
      echo ""
      read -rp "$(echo -e "${CYAN}SSL 模式${NC} [${def_ssl}]: ")" DB_SSL_MODE
      DB_SSL_MODE="${DB_SSL_MODE:-$def_ssl}"
      ;;
    3)
      DB_DRIVER="mysql"
      local def_host="${LAST_DB_HOST:-localhost}"
      local def_port="${LAST_DB_PORT:-3306}"
      local def_name="${LAST_DB_NAME:-auralogic}"
      local def_user="${LAST_DB_USER:-root}"
      if [ "${LAST_DB_DRIVER}" != "mysql" ]; then
        def_host="localhost"; def_port="3306"; def_name="auralogic"; def_user="root"
      fi
      read -rp "$(echo -e "${CYAN}MySQL 主机${NC} [${def_host}]: ")" DB_HOST
      DB_HOST="${DB_HOST:-$def_host}"
      read -rp "$(echo -e "${CYAN}MySQL 端口${NC} [${def_port}]: ")" DB_PORT
      DB_PORT="${DB_PORT:-$def_port}"
      read -rp "$(echo -e "${CYAN}数据库名${NC} [${def_name}]: ")" DB_NAME
      DB_NAME="${DB_NAME:-$def_name}"
      read -rp "$(echo -e "${CYAN}用户名${NC} [${def_user}]: ")" DB_USER
      DB_USER="${DB_USER:-$def_user}"
      read -rsp "$(echo -e "${CYAN}密码${NC}: ")" DB_PASSWORD
      echo ""
      DB_SSL_MODE=""
      ;;
    *)
      err "无效选择"
      exit 1
      ;;
  esac

  ok "数据库配置完成: $DB_DRIVER"
}

# ---------------------------
# 用户输入 - Redis 配置
# ---------------------------
read_redis_config() {
  # All-in-One 镜像内置 Redis，固定配置
  REDIS_HOST="127.0.0.1"
  REDIS_PORT=6379
  REDIS_PASSWORD=""
  REDIS_DB=0
}

# ---------------------------
# 用户输入 - JWT 配置
# ---------------------------
read_jwt_config() {
  step "JWT 配置"

  local def_expire="${LAST_JWT_EXPIRE:-24}"
  local def_refresh="${LAST_JWT_REFRESH_EXPIRE:-168}"

  JWT_SECRET=$(generate_secret)
  info "已自动生成 JWT Secret: ${JWT_SECRET:0:10}..."

  read -rp "$(echo -e "${CYAN}JWT 过期时间 (小时)${NC} [${def_expire}]: ")" JWT_EXPIRE
  JWT_EXPIRE="${JWT_EXPIRE:-$def_expire}"

  read -rp "$(echo -e "${CYAN}JWT 刷新令牌过期时间 (小时)${NC} [${def_refresh}]: ")" JWT_REFRESH_EXPIRE
  JWT_REFRESH_EXPIRE="${JWT_REFRESH_EXPIRE:-$def_refresh}"

  ok "JWT 配置完成"
}

# ---------------------------
# 用户输入 - OAuth 配置 (可选)
# ---------------------------
read_oauth_config() {
  step "OAuth 配置 (可选)"

  # GitHub OAuth
  local gh_hint="y/N"
  [ "${LAST_GITHUB_ENABLED}" = "true" ] && gh_hint="Y/n"
  read -rp "$(echo -e "${CYAN}是否配置 GitHub OAuth?${NC} [${gh_hint}]: ")" enable_github

  if [ -z "$enable_github" ]; then
    # 回车 = 沿用上次
    [ "${LAST_GITHUB_ENABLED}" = "true" ] && enable_github="Y" || enable_github="N"
  fi

  if [[ "$enable_github" =~ ^[Yy]$ ]]; then
    GITHUB_ENABLED="true"
    local def_gh_id="${LAST_GITHUB_CLIENT_ID}"
    if [ -n "$def_gh_id" ]; then
      read -rp "$(echo -e "${CYAN}GitHub Client ID${NC} [${def_gh_id}]: ")" GITHUB_CLIENT_ID
      GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID:-$def_gh_id}"
    else
      read -rp "$(echo -e "${CYAN}GitHub Client ID${NC}: ")" GITHUB_CLIENT_ID
    fi
    read -rp "$(echo -e "${CYAN}GitHub Client Secret${NC}: ")" GITHUB_CLIENT_SECRET
    GITHUB_REDIRECT_URL="${APP_URL}/oauth/github/callback"
  else
    GITHUB_ENABLED="false"
    GITHUB_CLIENT_ID=""
    GITHUB_CLIENT_SECRET=""
    GITHUB_REDIRECT_URL=""
  fi

  # Google OAuth
  local gg_hint="y/N"
  [ "${LAST_GOOGLE_ENABLED}" = "true" ] && gg_hint="Y/n"
  read -rp "$(echo -e "${CYAN}是否配置 Google OAuth?${NC} [${gg_hint}]: ")" enable_google

  if [ -z "$enable_google" ]; then
    [ "${LAST_GOOGLE_ENABLED}" = "true" ] && enable_google="Y" || enable_google="N"
  fi

  if [[ "$enable_google" =~ ^[Yy]$ ]]; then
    GOOGLE_ENABLED="true"
    local def_gg_id="${LAST_GOOGLE_CLIENT_ID}"
    if [ -n "$def_gg_id" ]; then
      read -rp "$(echo -e "${CYAN}Google Client ID${NC} [${def_gg_id}]: ")" GOOGLE_CLIENT_ID
      GOOGLE_CLIENT_ID="${GOOGLE_CLIENT_ID:-$def_gg_id}"
    else
      read -rp "$(echo -e "${CYAN}Google Client ID${NC}: ")" GOOGLE_CLIENT_ID
    fi
    read -rp "$(echo -e "${CYAN}Google Client Secret${NC}: ")" GOOGLE_CLIENT_SECRET
    GOOGLE_REDIRECT_URL="${APP_URL}/oauth/google/callback"
  else
    GOOGLE_ENABLED="false"
    GOOGLE_CLIENT_ID=""
    GOOGLE_CLIENT_SECRET=""
    GOOGLE_REDIRECT_URL=""
  fi

  ok "OAuth 配置完成"
}

# ---------------------------
# 用户输入 - SMTP 配置 (可选)
# ---------------------------
read_smtp_config() {
  step "SMTP 邮件配置 (可选)"

  local smtp_hint="y/N"
  [ "${LAST_SMTP_ENABLED}" = "true" ] && smtp_hint="Y/n"
  read -rp "$(echo -e "${CYAN}是否启用 SMTP 邮件服务?${NC} [${smtp_hint}]: ")" enable_smtp

  if [ -z "$enable_smtp" ]; then
    [ "${LAST_SMTP_ENABLED}" = "true" ] && enable_smtp="Y" || enable_smtp="N"
  fi

  if [[ "$enable_smtp" =~ ^[Yy]$ ]]; then
    SMTP_ENABLED="true"
    local def_host="${LAST_SMTP_HOST:-smtp.gmail.com}"
    local def_port="${LAST_SMTP_PORT:-587}"
    local def_user="${LAST_SMTP_USER}"
    local def_from_email="${LAST_SMTP_FROM_EMAIL:-noreply@${APP_URL#https://}}"
    local def_from_name="${LAST_SMTP_FROM_NAME:-$APP_NAME}"
    # 如果上次未启用 SMTP，用原始默认值
    if [ "${LAST_SMTP_ENABLED}" != "true" ]; then
      def_host="smtp.gmail.com"; def_port="587"; def_user=""
      def_from_email="noreply@${APP_URL#https://}"; def_from_name="$APP_NAME"
    fi

    read -rp "$(echo -e "${CYAN}SMTP 主机${NC} [${def_host}]: ")" SMTP_HOST
    SMTP_HOST="${SMTP_HOST:-$def_host}"
    read -rp "$(echo -e "${CYAN}SMTP 端口${NC} [${def_port}]: ")" SMTP_PORT
    SMTP_PORT="${SMTP_PORT:-$def_port}"
    if [ -n "$def_user" ]; then
      read -rp "$(echo -e "${CYAN}SMTP 用户名${NC} [${def_user}]: ")" SMTP_USER
      SMTP_USER="${SMTP_USER:-$def_user}"
    else
      read -rp "$(echo -e "${CYAN}SMTP 用户名${NC}: ")" SMTP_USER
    fi
    read -rsp "$(echo -e "${CYAN}SMTP 密码${NC}: ")" SMTP_PASSWORD
    echo ""
    read -rp "$(echo -e "${CYAN}发件人邮箱${NC} [${def_from_email}]: ")" SMTP_FROM_EMAIL
    SMTP_FROM_EMAIL="${SMTP_FROM_EMAIL:-$def_from_email}"
    read -rp "$(echo -e "${CYAN}发件人名称${NC} [${def_from_name}]: ")" SMTP_FROM_NAME
    SMTP_FROM_NAME="${SMTP_FROM_NAME:-$def_from_name}"
  else
    SMTP_ENABLED="false"
    SMTP_HOST=""
    SMTP_PORT=587
    SMTP_USER=""
    SMTP_PASSWORD=""
    SMTP_FROM_EMAIL=""
    SMTP_FROM_NAME=""
  fi

  ok "SMTP 配置完成"
}

# ---------------------------
# 用户输入 - SMS 配置 (可选)
# ---------------------------
read_sms_config() {
  step "SMS 短信配置 (可选)"

  local sms_hint="y/N"
  [ "${LAST_SMS_ENABLED}" = "true" ] && sms_hint="Y/n"
  read -rp "$(echo -e "${CYAN}是否启用 SMS 短信服务?${NC} [${sms_hint}]: ")" enable_sms

  if [ -z "$enable_sms" ]; then
    [ "${LAST_SMS_ENABLED}" = "true" ] && enable_sms="Y" || enable_sms="N"
  fi

  if [[ "$enable_sms" =~ ^[Yy]$ ]]; then
    SMS_ENABLED="true"

    local def_provider="${LAST_SMS_PROVIDER:-aliyun}"
    echo "选择 SMS 服务商:"
    echo "  1) 阿里云 (aliyun)"
    echo "  2) Twilio"
    echo "  3) 自定义 HTTP (custom)"
    local def_sms_choice="1"
    case "$def_provider" in
      twilio) def_sms_choice="2" ;;
      custom) def_sms_choice="3" ;;
    esac
    read -rp "请选择 [${def_sms_choice}]: " sms_choice
    sms_choice="${sms_choice:-$def_sms_choice}"

    case "$sms_choice" in
      1)
        SMS_PROVIDER="aliyun"
        read -rp "$(echo -e "${CYAN}阿里云 AccessKey ID${NC}: ")" SMS_ALIYUN_AK_ID
        read -rsp "$(echo -e "${CYAN}阿里云 AccessKey Secret${NC}: ")" SMS_ALIYUN_AK_SECRET
        echo ""
        read -rp "$(echo -e "${CYAN}短信签名${NC}: ")" SMS_ALIYUN_SIGN
        read -rp "$(echo -e "${CYAN}模板 Code${NC}: ")" SMS_ALIYUN_TPL
        ;;
      2)
        SMS_PROVIDER="twilio"
        read -rp "$(echo -e "${CYAN}Twilio Account SID${NC}: ")" SMS_TWILIO_SID
        read -rsp "$(echo -e "${CYAN}Twilio Auth Token${NC}: ")" SMS_TWILIO_TOKEN
        echo ""
        read -rp "$(echo -e "${CYAN}Twilio From Number${NC}: ")" SMS_TWILIO_FROM
        ;;
      3)
        SMS_PROVIDER="custom"
        read -rp "$(echo -e "${CYAN}自定义 API URL${NC}: ")" SMS_CUSTOM_URL
        read -rp "$(echo -e "${CYAN}HTTP Method${NC} [POST]: ")" SMS_CUSTOM_METHOD
        SMS_CUSTOM_METHOD="${SMS_CUSTOM_METHOD:-POST}"
        ;;
      *)
        err "无效选择"; exit 1 ;;
    esac
  else
    SMS_ENABLED="false"
    SMS_PROVIDER="aliyun"
  fi

  ok "SMS 配置完成"
}

# ---------------------------
# 用户输入 - 管理员账号
# ---------------------------
read_admin_config() {
  step "超级管理员账号配置"

  local def_email="${LAST_ADMIN_EMAIL:-admin@${APP_URL#https://}}"
  local def_name="${LAST_ADMIN_NAME:-超级管理员}"

  read -rp "$(echo -e "${CYAN}管理员邮箱${NC} [${def_email}]: ")" ADMIN_EMAIL
  ADMIN_EMAIL="${ADMIN_EMAIL:-$def_email}"

  while true; do
    read -rsp "$(echo -e "${CYAN}管理员密码${NC} (至少8位，包含大小写字母和数字): ")" ADMIN_PASSWORD
    echo ""
    if [ ${#ADMIN_PASSWORD} -lt 8 ]; then
      warn "密码长度至少8位"
      continue
    fi
    read -rsp "$(echo -e "${CYAN}确认密码${NC}: ")" ADMIN_PASSWORD_CONFIRM
    echo ""
    if [ "$ADMIN_PASSWORD" != "$ADMIN_PASSWORD_CONFIRM" ]; then
      warn "两次密码不一致"
      continue
    fi
    break
  done

  read -rp "$(echo -e "${CYAN}管理员名称${NC} [${def_name}]: ")" ADMIN_NAME
  ADMIN_NAME="${ADMIN_NAME:-$def_name}"

  ok "管理员账号配置完成"
}

# ---------------------------
# 配置确认
# ---------------------------
confirm_config() {
  step "配置确认"

  echo "=========================================="
  echo "应用配置:"
  echo "  名称:     $APP_NAME"
  echo "  URL:      $APP_URL"
  echo "  映射端口: $EXPOSE_PORT (宿主机) -> 80 (容器)"
  echo "  环境:     $APP_ENV"
  echo "  调试:     $APP_DEBUG"
  echo ""
  echo "数据库:"
  echo "  类型:     $DB_DRIVER"
  if [ "$DB_DRIVER" != "sqlite" ]; then
    echo "  主机:     $DB_HOST:$DB_PORT"
    echo "  数据库:   $DB_NAME"
    echo "  用户:     $DB_USER"
  else
    echo "  文件:     $DB_NAME"
  fi
  echo ""
  echo "OAuth:"
  echo "  GitHub:   $GITHUB_ENABLED"
  echo "  Google:   $GOOGLE_ENABLED"
  echo ""
  echo "SMTP:"
  echo "  启用:     $SMTP_ENABLED"
  if [ "$SMTP_ENABLED" = "true" ]; then
    echo "  主机:     $SMTP_HOST:$SMTP_PORT"
    echo "  发件人:   $SMTP_FROM_EMAIL"
  fi
  echo ""
  echo "SMS:"
  echo "  启用:     $SMS_ENABLED"
  if [ "$SMS_ENABLED" = "true" ]; then
    echo "  服务商:   $SMS_PROVIDER"
  fi
  echo ""
  echo "管理员:"
  echo "  邮箱:     $ADMIN_EMAIL"
  echo "  名称:     $ADMIN_NAME"
  echo ""
  echo "Docker:"
  echo "  镜像:     ${IMAGE_NAME}:${IMAGE_TAG}"
  echo "  项目:     $PROJECT_ROOT"
  echo "=========================================="
  echo ""

  read -rp "$(echo -e "${YELLOW}确认以上配置开始构建?${NC} [Y/n]: ")" confirm
  if [[ "$confirm" =~ ^[Nn]$ ]]; then
    info "已取消"
    exit 0
  fi
}

# ---------------------------
# 从 URL 提取域名
# ---------------------------
extract_domain() {
  local url="$1"
  # 移除协议
  local domain="${url#http://}"
  domain="${domain#https://}"
  # 移除端口和路径
  domain="${domain%%:*}"
  domain="${domain%%/*}"
  echo "$domain"
}

# ---------------------------
# 生成配置文件
# ---------------------------
generate_configs() {
  step "生成配置文件"

  mkdir -p "$WORK_DIR/docker-build"

  # 提取域名用于 CORS 和图片域名配置
  APP_DOMAIN=$(extract_domain "$APP_URL")

  # DB pool defaults (sqlite uses a single connection to avoid lock contention)
  DB_MAX_OPEN=100
  DB_MAX_IDLE=10
  DB_CONN_MAX_LIFETIME=3600
  if [ "$DB_DRIVER" = "sqlite" ]; then
    DB_MAX_OPEN=1
    DB_MAX_IDLE=1
    DB_CONN_MAX_LIFETIME=0
  fi

  # 生成 config.json
  cat > "$WORK_DIR/docker-build/config.json" <<EOF
{
  "app": {
    "name": "$APP_NAME",
    "port": $APP_PORT,
    "env": "$APP_ENV",
    "debug": $APP_DEBUG,
    "url": "$APP_URL",
    "default_theme": "system"
  },
  "database": {
    "driver": "$DB_DRIVER",
    "host": "$DB_HOST",
    "port": $DB_PORT,
    "user": "$DB_USER",
    "password": "$DB_PASSWORD",
    "name": "$DB_NAME",
    "ssl_mode": "$DB_SSL_MODE",
    "max_open_conns": $DB_MAX_OPEN,
    "max_idle_conns": $DB_MAX_IDLE,
    "conn_max_lifetime": $DB_CONN_MAX_LIFETIME
  },
  "redis": {
    "host": "$REDIS_HOST",
    "port": $REDIS_PORT,
    "password": "$REDIS_PASSWORD",
    "db": $REDIS_DB,
    "pool_size": 10
  },
  "jwt": {
    "secret": "$JWT_SECRET",
    "expire_hours": $JWT_EXPIRE,
    "refresh_expire_hours": $JWT_REFRESH_EXPIRE
  },
  "oauth": {
    "github": {
      "enabled": $GITHUB_ENABLED,
      "client_id": "$GITHUB_CLIENT_ID",
      "client_secret": "$GITHUB_CLIENT_SECRET",
      "redirect_url": "$GITHUB_REDIRECT_URL"
    },
    "google": {
      "enabled": $GOOGLE_ENABLED,
      "client_id": "$GOOGLE_CLIENT_ID",
      "client_secret": "$GOOGLE_CLIENT_SECRET",
      "redirect_url": "$GOOGLE_REDIRECT_URL"
    }
  },
  "smtp": {
    "enabled": $SMTP_ENABLED,
    "host": "$SMTP_HOST",
    "port": $SMTP_PORT,
    "user": "$SMTP_USER",
    "password": "$SMTP_PASSWORD",
    "from_email": "$SMTP_FROM_EMAIL",
    "from_name": "$SMTP_FROM_NAME"
  },
  "sms": {
    "enabled": $SMS_ENABLED,
    "provider": "$SMS_PROVIDER",
    "aliyun_access_key_id": "${SMS_ALIYUN_AK_ID:-}",
    "aliyun_access_secret": "${SMS_ALIYUN_AK_SECRET:-}",
    "aliyun_sign_name": "${SMS_ALIYUN_SIGN:-}",
    "aliyun_template_code": "${SMS_ALIYUN_TPL:-}",
    "templates": {"login": "", "register": "", "reset_password": "", "bind_phone": ""},
    "dypns_code_length": 6,
    "twilio_account_sid": "${SMS_TWILIO_SID:-}",
    "twilio_auth_token": "${SMS_TWILIO_TOKEN:-}",
    "twilio_from_number": "${SMS_TWILIO_FROM:-}",
    "custom_url": "${SMS_CUSTOM_URL:-}",
    "custom_method": "${SMS_CUSTOM_METHOD:-POST}",
    "custom_headers": {},
    "custom_body_template": ""
  },
  "security": {
    "ip_header": "X-Real-IP",
    "trusted_proxies": ["127.0.0.1/32"],
    "captcha": {
      "provider": "none",
      "site_key": "",
      "secret_key": "",
      "enable_for_login": false,
      "enable_for_register": false,
      "enable_for_serial_verify": false,
      "enable_for_bind": false
    },
    "cors": {
      "allowed_origins": ["$APP_URL", "http://localhost:3000"],
      "allowed_methods": ["GET", "POST", "PUT", "PATCH", "DELETE"],
      "allowed_headers": ["Origin", "Content-Type", "Authorization", "X-API-Key", "X-API-Secret", "X-Real-IP", "X-Forwarded-For"],
      "max_age": 86400
    },
    "login": {
      "allow_password_login": true,
      "allow_registration": true,
      "require_email_verification": false,
      "allow_email_login": false,
      "allow_password_reset": false,
      "allow_phone_login": false,
      "allow_phone_register": false,
      "allow_phone_password_reset": false
    },
    "password_policy": {
      "min_length": 8,
      "require_uppercase": true,
      "require_lowercase": true,
      "require_number": true,
      "require_special": false
    }
  },
  "serial": {
    "enabled": true
  },
  "rate_limit": {
    "enabled": true,
    "api": 10000,
    "user_request": 600,
    "user_login": 100,
    "admin_request": 2000
  },
  "email_rate_limit": {
    "hourly": 0,
    "daily": 0,
    "exceed_action": "cancel"
  },
  "sms_rate_limit": {
    "hourly": 0,
    "daily": 0,
    "exceed_action": "cancel"
  },
  "order": {
    "auto_cancel_hours": 72,
    "no_prefix": "ORD",
    "currency": "CNY",
    "stock_display": {
      "mode": "exact",
      "low_stock_threshold": 10,
      "high_stock_threshold": 50
    },
    "virtual_delivery_order": "random"
  },
  "upload": {
    "dir": "uploads",
    "max_size": 5242880,
    "allowed_types": [".jpg", ".jpeg", ".png", ".gif", ".webp"]
  },
  "log": {
    "level": "info",
    "format": "json",
    "output": "file",
    "file_path": "logs/app.log"
  },
  "magic_link": {
    "expire_minutes": 15,
    "max_uses": 1
  },
  "form": {
    "expire_hours": 24
  },
  "ticket": {
    "enabled": true,
    "categories": [],
    "template": "",
    "max_content_length": 0,
    "auto_close_hours": 0,
    "attachment": {
      "enable_image": true,
      "enable_voice": true,
      "max_image_size": 5242880,
      "max_voice_size": 10485760,
      "max_voice_duration": 60,
      "allowed_image_types": [".jpg", ".jpeg", ".png", ".gif", ".webp"],
      "retention_days": 0
    }
  },
  "customization": {
    "primary_color": "",
    "logo_url": "",
    "favicon_url": "",
    "page_rules": [],
    "auth_branding": {
      "mode": "default",
      "title": "",
      "title_en": "",
      "subtitle": "",
      "subtitle_en": "",
      "custom_html": ""
    }
  },
  "email_notifications": {
    "user_register": false,
    "order_created": false,
    "order_paid": false,
    "order_shipped": false,
    "order_completed": false,
    "order_cancelled": false,
    "order_resubmit": false,
    "ticket_created": false,
    "ticket_admin_reply": false,
    "ticket_user_reply": false,
    "ticket_resolved": false
  },
  "analytics": {
    "enabled": false
  }
}
EOF

  # 生成 admin.json
  cat > "$WORK_DIR/docker-build/admin.json" <<EOF
{
  "super_admin": {
    "email": "$ADMIN_EMAIL",
    "password": "$ADMIN_PASSWORD",
    "name": "$ADMIN_NAME"
  }
}
EOF

  # ---------------------------
  # 前端配置: .env.production
  # ---------------------------
  cat > "$WORK_DIR/docker-build/frontend.env.production" <<EOF
NEXT_PUBLIC_API_URL=$APP_URL
NEXT_PUBLIC_GIT_COMMIT=$(cd "$WORK_DIR" && git rev-parse --short HEAD 2>/dev/null || echo "dev")
EOF

  # ---------------------------
  # 前端配置: 补丁 next.config.js (替换图片域名)
  # ---------------------------
  cat > "$WORK_DIR/docker-build/next.config.js" <<NEXTEOF
/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    images: {
        domains: [
            'localhost',
            '$APP_DOMAIN',
        ],
        formats: ['image/avif', 'image/webp'],
    },
    reactStrictMode: true,
    env: {
        NEXT_PUBLIC_GIT_COMMIT: process.env.NEXT_PUBLIC_GIT_COMMIT || 'dev',
        NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || '$APP_URL',
    },
    async redirects() {
        return [
            {
                source: '/admin',
                destination: '/admin/dashboard',
                permanent: true,
            },
        ]
    },
    async headers() {
        return [
            {
                source: '/:path*',
                headers: [
                    { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
                    { key: 'X-Content-Type-Options', value: 'nosniff' },
                    { key: 'X-XSS-Protection', value: '1; mode=block' },
                    { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
                    { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
                    { key: 'Strict-Transport-Security', value: 'max-age=31536000; includeSubDomains' },
                ],
            },
        ]
    },
    webpack: (config, { isServer }) => {
        if (!isServer) {
            config.resolve.fallback = { ...config.resolve.fallback, fs: false }
        }
        return config
    },
}

module.exports = nextConfig
NEXTEOF

  ok "前后端配置文件已生成"
  info "后端: docker-build/config.json"
  info "前端: docker-build/next.config.js, docker-build/frontend.env.production"
}

# ---------------------------
# 复制 Docker 构建文件
# ---------------------------
copy_docker_files() {
  step "准备 Docker 构建文件"

  # 复制 Dockerfile, nginx.conf, supervisord.conf, entrypoint.sh
  cp "$(dirname "$0")/docker/Dockerfile.all-in-one" "$WORK_DIR/docker-build/Dockerfile"
  cp "$(dirname "$0")/docker/nginx.conf" "$WORK_DIR/docker-build/nginx.conf"
  cp "$(dirname "$0")/docker/supervisord.conf" "$WORK_DIR/docker-build/supervisord.conf"
  cp "$(dirname "$0")/docker/entrypoint.sh" "$WORK_DIR/docker-build/entrypoint.sh"

  # 复制后端邮件模板文件到构建目录 (用户可在此目录自定义模板)
  if [ -d "$PROJECT_ROOT/backend/templates" ]; then
    cp -r "$PROJECT_ROOT/backend/templates" "$WORK_DIR/docker-build/templates"
    ok "邮件模板已复制到 docker-build/templates/"
  fi

  ok "Docker 构建文件已准备"
}

# ---------------------------
# 构建 Docker 镜像
# ---------------------------
build_docker_image() {
  step "构建 All-in-One Docker 镜像"

  cd "$WORK_DIR"

  # 构建镜像
  GIT_COMMIT=$(cd "$WORK_DIR" && git rev-parse --short HEAD 2>/dev/null || echo "dev")
  docker build \
    --build-arg NEXT_PUBLIC_API_URL="$APP_URL" \
    --build-arg BUILD_VERSION="$GIT_COMMIT" \
    -f docker-build/Dockerfile \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -t "${IMAGE_NAME}:latest" \
    .

  ok "Docker 镜像构建完成: ${IMAGE_NAME}:${IMAGE_TAG}"
}

# ---------------------------
# 生成 docker-compose.yml
# ---------------------------
generate_compose() {
  step "生成 docker-compose.yml"

  # 根据数据库类型生成不同的 compose 文件
  case "$DB_DRIVER" in
    sqlite)
      cat > "$WORK_DIR/docker-compose.yml" <<EOF
services:
  auralogic:
    image: ${IMAGE_NAME}:${IMAGE_TAG}
    container_name: auralogic
    ports:
      - "${EXPOSE_PORT}:80"
    volumes:
      - ./docker-build/config.json:/app/backend/config/config.json
      - ./docker-build/templates:/app/backend/templates
      - auralogic_data:/app/backend/data
      - auralogic_logs:/app/backend/logs
      - auralogic_uploads:/app/backend/uploads
      - redis_data:/var/lib/redis
    restart: unless-stopped

volumes:
  auralogic_data:
  auralogic_logs:
  auralogic_uploads:
  redis_data:
EOF
      ;;
    postgres)
      cat > "$WORK_DIR/docker-compose.yml" <<EOF
services:
  auralogic:
    image: ${IMAGE_NAME}:${IMAGE_TAG}
    container_name: auralogic
    ports:
      - "${EXPOSE_PORT}:80"
    volumes:
      - ./docker-build/config.json:/app/backend/config/config.json
      - ./docker-build/templates:/app/backend/templates
      - auralogic_logs:/app/backend/logs
      - auralogic_uploads:/app/backend/uploads
      - redis_data:/var/lib/redis
    depends_on:
      - postgres
    restart: unless-stopped
    networks:
      - auralogic-net

  postgres:
    image: postgres:15-alpine
    container_name: auralogic-postgres
    environment:
      POSTGRES_DB: ${DB_NAME}
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped
    networks:
      - auralogic-net

volumes:
  auralogic_logs:
  auralogic_uploads:
  redis_data:
  postgres_data:

networks:
  auralogic-net:
    driver: bridge
EOF
      ;;
    mysql)
      cat > "$WORK_DIR/docker-compose.yml" <<EOF
services:
  auralogic:
    image: ${IMAGE_NAME}:${IMAGE_TAG}
    container_name: auralogic
    ports:
      - "${EXPOSE_PORT}:80"
    volumes:
      - ./docker-build/config.json:/app/backend/config/config.json
      - ./docker-build/templates:/app/backend/templates
      - auralogic_logs:/app/backend/logs
      - auralogic_uploads:/app/backend/uploads
      - redis_data:/var/lib/redis
    depends_on:
      - mysql
    restart: unless-stopped
    networks:
      - auralogic-net

  mysql:
    image: mysql:8.0
    container_name: auralogic-mysql
    environment:
      MYSQL_DATABASE: ${DB_NAME}
      MYSQL_USER: ${DB_USER}
      MYSQL_PASSWORD: ${DB_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    restart: unless-stopped
    networks:
      - auralogic-net

volumes:
  auralogic_logs:
  auralogic_uploads:
  redis_data:
  mysql_data:

networks:
  auralogic-net:
    driver: bridge
EOF
      ;;
  esac

  ok "docker-compose.yml 已生成"
}

# ---------------------------
# 更新本地容器 (跳过交互配置，复用上次构建)
# ---------------------------
update_container() {
  step "更新本地容器"

  # 检查构建文件是否存在
  if [ ! -f "$WORK_DIR/docker-build/Dockerfile" ] || [ ! -f "$WORK_DIR/docker-build/config.json" ]; then
    err "未找到上次构建文件 (docker-build/)，请先执行完整构建"
    exit 1
  fi

  load_last_config

  APP_URL="${LAST_APP_URL:-}"
  EXPOSE_PORT="${LAST_EXPOSE_PORT:-80}"
  DB_DRIVER="${LAST_DB_DRIVER:-sqlite}"

  if [ -z "$APP_URL" ]; then
    err "未找到上次构建配置，请先执行完整构建"
    exit 1
  fi

  info "使用上次构建配置:"
  info "  应用 URL:  $APP_URL"
  info "  映射端口:  $EXPOSE_PORT"
  info "  数据库:    $DB_DRIVER"
  echo ""

  read -rp "$(echo -e "${YELLOW}确认重新编译并更新容器?${NC} [Y/n]: ")" confirm
  if [[ "$confirm" =~ ^[Nn]$ ]]; then
    info "已取消"
    exit 0
  fi

  # 可选: 覆盖邮件模板文件
  if [ -d "$PROJECT_ROOT/backend/templates" ]; then
    echo ""
    if [ -d "$WORK_DIR/docker-build/templates" ]; then
      read -rp "$(echo -e "${CYAN}是否用最新源码覆盖邮件模板文件?${NC} (如果您自定义过模板请选 N) [y/N]: ")" overwrite_templates
      if [[ "$overwrite_templates" =~ ^[Yy]$ ]]; then
        rm -rf "$WORK_DIR/docker-build/templates"
        cp -r "$PROJECT_ROOT/backend/templates" "$WORK_DIR/docker-build/templates"
        ok "邮件模板已更新为最新版本"
      else
        info "保留现有邮件模板"
      fi
    else
      info "未发现已有模板目录，自动复制源码模板..."
      cp -r "$PROJECT_ROOT/backend/templates" "$WORK_DIR/docker-build/templates"
      ok "邮件模板已复制到 docker-build/templates/"
    fi
  fi

  # 更新构建文件（Dockerfile + 前端配置）
  step "更新构建文件"
  cp "$(dirname "$0")/docker/Dockerfile.all-in-one" "$WORK_DIR/docker-build/Dockerfile"
  cp "$(dirname "$0")/docker/nginx.conf" "$WORK_DIR/docker-build/nginx.conf"
  cp "$(dirname "$0")/docker/supervisord.conf" "$WORK_DIR/docker-build/supervisord.conf"
  cp "$(dirname "$0")/docker/entrypoint.sh" "$WORK_DIR/docker-build/entrypoint.sh"

  cat > "$WORK_DIR/docker-build/frontend.env.production" <<EOF
NEXT_PUBLIC_API_URL=$APP_URL
NEXT_PUBLIC_GIT_COMMIT=$(cd "$WORK_DIR" && git rev-parse --short HEAD 2>/dev/null || echo "dev")
EOF

  cat > "$WORK_DIR/docker-build/next.config.js" <<NEXTEOF
/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    images: {
        domains: [
            'localhost',
            '$(extract_domain "$APP_URL")',
        ],
        formats: ['image/avif', 'image/webp'],
    },
    reactStrictMode: true,
    env: {
        NEXT_PUBLIC_GIT_COMMIT: process.env.NEXT_PUBLIC_GIT_COMMIT || 'dev',
        NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || '$APP_URL',
    },
    async redirects() {
        return [
            {
                source: '/admin',
                destination: '/admin/dashboard',
                permanent: true,
            },
        ]
    },
    async headers() {
        return [
            {
                source: '/:path*',
                headers: [
                    { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
                    { key: 'X-Content-Type-Options', value: 'nosniff' },
                    { key: 'X-XSS-Protection', value: '1; mode=block' },
                    { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
                    { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
                    { key: 'Strict-Transport-Security', value: 'max-age=31536000; includeSubDomains' },
                ],
            },
        ]
    },
    webpack: (config, { isServer }) => {
        if (!isServer) {
            config.resolve.fallback = { ...config.resolve.fallback, fs: false }
        }
        return config
    },
}

module.exports = nextConfig
NEXTEOF

  ok "构建文件已更新"

  # 重新构建镜像
  step "重新构建 Docker 镜像"
  cd "$WORK_DIR"
  GIT_COMMIT=$(cd "$WORK_DIR" && git rev-parse --short HEAD 2>/dev/null || echo "dev")
  docker build \
    --build-arg NEXT_PUBLIC_API_URL="$APP_URL" \
    --build-arg BUILD_VERSION="$GIT_COMMIT" \
    -f docker-build/Dockerfile \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -t "${IMAGE_NAME}:latest" \
    .

  ok "镜像构建完成: ${IMAGE_NAME}:${IMAGE_TAG}"

  # 更新并重启容器
  step "更新容器"

  # 先停止并移除旧容器（避免容器名冲突）
  if docker ps -aq -f name='^auralogic$' | grep -q .; then
    info "停止并移除旧容器..."
    docker stop auralogic 2>/dev/null || true
    docker rm auralogic 2>/dev/null || true
  fi

  if [ -f "$WORK_DIR/docker-compose.yml" ]; then
    # 更新 compose 文件中的镜像 tag
    sed -i "s|image: ${IMAGE_NAME}:.*|image: ${IMAGE_NAME}:${IMAGE_TAG}|" "$WORK_DIR/docker-compose.yml"

    # 移除旧版 version 字段（如有）
    sed -i '/^version:/d' "$WORK_DIR/docker-compose.yml"

    # 移除 config.json 挂载的 :ro 标志（如有）
    sed -i 's|:/app/backend/config/config.json:ro|:/app/backend/config/config.json|g' "$WORK_DIR/docker-compose.yml"

    # 如果 compose 文件中没有模板挂载，自动添加
    if ! grep -q 'docker-build/templates:/app/backend/templates' "$WORK_DIR/docker-compose.yml"; then
      sed -i '/docker-build\/config.json:\/app\/backend\/config\/config.json/a\      - ./docker-build/templates:/app/backend/templates' "$WORK_DIR/docker-compose.yml"
      info "已为 docker-compose.yml 添加模板挂载"
    fi

    cd "$WORK_DIR"
    docker compose up -d --force-recreate
    ok "容器已通过 docker compose 更新"
  else
    # 无 compose 文件，直接操作容器
    local container_id
    container_id=$(docker ps -aq -f name='^auralogic$')

    if [ -n "$container_id" ]; then
      info "停止并移除旧容器..."
      docker stop auralogic 2>/dev/null || true
      docker rm auralogic 2>/dev/null || true
    fi

    info "启动新容器..."
    docker run -d \
      -p "${EXPOSE_PORT}:80" \
      --name auralogic \
      -v "$WORK_DIR/docker-build/config.json:/app/backend/config/config.json" \
      -v "$WORK_DIR/docker-build/templates:/app/backend/templates" \
      -v auralogic_data:/app/backend/data \
      -v auralogic_logs:/app/backend/logs \
      -v auralogic_uploads:/app/backend/uploads \
      -v redis_data:/var/lib/redis \
      "${IMAGE_NAME}:${IMAGE_TAG}"
    ok "容器已启动"
  fi

  # 清理旧镜像
  echo ""
  read -rp "$(echo -e "${CYAN}是否清理旧版本镜像?${NC} [y/N]: ")" clean_confirm
  if [[ "$clean_confirm" =~ ^[Yy]$ ]]; then
    docker images "${IMAGE_NAME}" --format "{{.Tag}}" | grep -v "^${IMAGE_TAG}$" | grep -v "^latest$" | while read -r old_tag; do
      docker rmi "${IMAGE_NAME}:${old_tag}" 2>/dev/null && info "已删除: ${IMAGE_NAME}:${old_tag}"
    done
    ok "旧镜像清理完成"
  fi

  echo ""
  echo "=========================================="
  echo -e "${GREEN}✓ 容器更新完成!${NC}"
  echo "=========================================="
  echo ""
  echo "Docker 镜像: ${IMAGE_NAME}:${IMAGE_TAG}"
  echo "访问应用:    ${APP_URL}"
  echo "=========================================="
}

# ---------------------------
# 汇总
# ---------------------------
summary() {
  step "构建完成!"

  echo "=========================================="
  echo -e "${GREEN}✓ All-in-One Docker 镜像构建成功!${NC}"
  echo "=========================================="
  echo ""
  echo "Docker 镜像:"
  echo "  ${IMAGE_NAME}:${IMAGE_TAG}"
  echo "  ${IMAGE_NAME}:latest"
  echo ""
  echo "构建目录:"
  echo "  $PROJECT_ROOT"
  echo ""
  echo "部署文件:"
  echo "  $PROJECT_ROOT/docker-compose.yml"
  echo ""
  echo "启动服务:"
  echo "  cd $WORK_DIR"
  if [ "$DB_DRIVER" = "sqlite" ]; then
    echo "  docker run -d -p ${EXPOSE_PORT}:80 --name auralogic ${IMAGE_NAME}:${IMAGE_TAG}"
  else
    echo "  docker compose up -d"
  fi
  echo ""
  echo "访问应用:"
  echo "  $APP_URL"
  echo ""
  echo "管理员登录:"
  echo "  邮箱: $ADMIN_EMAIL"
  echo "  密码: (您设置的密码)"
  echo ""
  echo "=========================================="
  echo -e "${YELLOW}⚠️  重要提示:${NC}"
  echo "  1. 首次启动会自动初始化数据库和管理员账号"
  echo "  2. 请妥善保管配置文件和管理员密码"
  echo "  3. 生产环境建议配置 HTTPS 反向代理"
  if [ "$DB_DRIVER" != "sqlite" ]; then
    echo "  4. 确保 PostgreSQL/MySQL 服务正常运行"
  fi
  echo "=========================================="
}

# ---------------------------
# 完整构建流程
# ---------------------------
full_build() {
  load_last_config
  check_deps
  read_basic_config
  read_database_config
  read_redis_config
  read_jwt_config
  read_oauth_config
  read_smtp_config
  read_sms_config
  read_admin_config
  confirm_config
  save_config
  generate_configs
  copy_docker_files
  build_docker_image
  generate_compose
  summary
}

# ---------------------------
# 主流程
# ---------------------------
main() {
  echo ""
  echo "=========================================="
  echo "  AuraLogic All-in-One Docker 构建脚本"
  echo "=========================================="
  echo ""

  # 支持命令行参数: ./build_docker.sh update
  local action="$1"

  if [ -z "$action" ]; then
    echo "请选择操作:"
    echo "  1) 完整构建 (首次部署/重新配置)"
    echo "  2) 更新容器 (重新编译并更新现有容器)"
    read -rp "请选择 [1]: " action_choice
    action_choice="${action_choice:-1}"

    case "$action_choice" in
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
      check_deps
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
