#!/bin/sh
set -e

echo "=========================================="
echo "  AuraLogic All-in-One Container Starting"
echo "=========================================="

# ---------------------------
# 设置环境变量
# ---------------------------
export CONFIG_PATH="/app/backend/config/config.json"

# ---------------------------
# 等待数据库就绪 (PostgreSQL/MySQL)
# ---------------------------
wait_for_db() {
  # 从 config.json 读取数据库类型
  DB_DRIVER=$(cat /app/backend/config/config.json | sed -n 's/.*"driver"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)

  if [ "$DB_DRIVER" = "sqlite" ]; then
    echo "[INIT] 使用 SQLite，跳过数据库等待"
    return 0
  fi

  DB_HOST=$(cat /app/backend/config/config.json | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
  DB_PORT=$(cat /app/backend/config/config.json | sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -1)

  if [ -z "$DB_HOST" ] || [ -z "$DB_PORT" ] || [ "$DB_PORT" = "0" ]; then
    echo "[INIT] 未检测到外部数据库配置，跳过等待"
    return 0
  fi

  echo "[INIT] 等待数据库就绪 ${DB_DRIVER}://${DB_HOST}:${DB_PORT} ..."

  MAX_RETRIES=30
  RETRY=0
  while [ $RETRY -lt $MAX_RETRIES ]; do
    if nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; then
      echo "[INIT] 数据库已就绪"
      return 0
    fi
    RETRY=$((RETRY + 1))
    echo "[INIT] 等待数据库... (${RETRY}/${MAX_RETRIES})"
    sleep 2
  done

  echo "[ERROR] 数据库连接超时，请检查数据库服务是否正常运行"
  exit 1
}

# ---------------------------
# 等待 Redis 就绪 (仅外部 Redis)
# ---------------------------
wait_for_redis() {
  REDIS_HOST=$(cat /app/backend/config/config.json | sed -n '/"redis"/,/}/{' -e 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' -e '}' | head -1)
  REDIS_PORT=$(cat /app/backend/config/config.json | sed -n '/"redis"/,/}/{ s/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p; }' | head -1)
  REDIS_PORT="${REDIS_PORT:-6379}"

  # 内置 Redis 由 supervisord 管理，无需在此等待
  if [ -z "$REDIS_HOST" ] || [ "$REDIS_HOST" = "127.0.0.1" ] || [ "$REDIS_HOST" = "localhost" ]; then
    echo "[INIT] 使用内置 Redis，跳过等待 (由 supervisord 管理)"
    return 0
  fi

  echo "[INIT] 等待外部 Redis 就绪 ${REDIS_HOST}:${REDIS_PORT} ..."

  MAX_RETRIES=15
  RETRY=0
  while [ $RETRY -lt $MAX_RETRIES ]; do
    if nc -z "$REDIS_HOST" "$REDIS_PORT" 2>/dev/null; then
      echo "[INIT] Redis 已就绪"
      return 0
    fi
    RETRY=$((RETRY + 1))
    echo "[INIT] 等待 Redis... (${RETRY}/${MAX_RETRIES})"
    sleep 2
  done

  echo "[WARN] Redis 连接超时，服务可能受影响"
}

# ---------------------------
# 数据库迁移 & 初始化管理员
# ---------------------------
init_database() {
  cd /app/backend

  # 检查配置文件
  if [ ! -f config/config.json ]; then
    echo "[ERROR] 未找到配置文件: /app/backend/config/config.json"
    exit 1
  fi

  # admin.json 不存在说明已经初始化过，跳过
  if [ ! -f config/admin.json ]; then
    echo "[INIT] admin.json 不存在，已完成过初始化，跳过"
    return 0
  fi

  # 检查 init_admin 二进制
  if [ ! -x ./init_admin ]; then
    echo "[ERROR] 未找到 init_admin 可执行文件"
    exit 1
  fi

  echo "[INIT] 首次启动，执行数据库迁移 & 初始化超级管理员..."

  OUTPUT=$(./init_admin 2>&1) || {
    echo "[ERROR] 管理员初始化失败:"
    echo "$OUTPUT"
    exit 1
  }

  echo "$OUTPUT"

  # 安全清理: 删除包含明文密码的 admin.json
  echo "[SECURITY] 删除 admin.json 防止密码泄露..."
  rm -f config/admin.json

  echo "[INIT] 数据库初始化完成"
}

# ---------------------------
# 主流程
# ---------------------------
wait_for_db
wait_for_redis
init_database

echo "[START] 启动所有服务 (backend + frontend + nginx)..."
exec /usr/bin/supervisord -c /etc/supervisord.conf
