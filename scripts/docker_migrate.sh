#!/bin/bash

set -e

# ============================================
# AuraLogic Docker 迁移脚本
# 功能: 打包迁移 / 解包释放
# 包含: Docker镜像 + 数据卷 + 配置文件
# ============================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_NAME="auralogic-allinone"
CONTAINER_NAME="auralogic"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DEFAULT_PACK_NAME="auralogic-migrate-${TIMESTAMP}.tar.gz"

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

# 需要迁移的挂载点 (容器内路径)
MOUNT_POINTS=(/app/backend/data /app/backend/logs /app/backend/uploads /var/lib/redis)

# ---------------------------
# 从容器获取 volume 名 (按挂载点)
# 返回格式: 挂载点=volume名，每行一条
# ---------------------------
detect_volumes_from_container() {
  docker inspect "$CONTAINER_NAME" \
    --format='{{range .Mounts}}{{if eq .Type "volume"}}{{.Destination}}={{.Name}}{{"\n"}}{{end}}{{end}}' \
    2>/dev/null | grep -v '^$'
}

# 根据挂载点查找 volume 名
find_volume_for_mount() {
  local mount="$1"
  local mapping="$2"
  echo "$mapping" | grep "^${mount}=" | head -1 | cut -d= -f2
}

# ---------------------------
# 检查 Docker 环境
# ---------------------------
check_docker() {
  if ! command -v docker &>/dev/null; then
    err "未检测到 Docker，请先安装 Docker"
    exit 1
  fi
  if ! docker info &>/dev/null 2>&1; then
    err "Docker 未运行，请先启动 Docker 服务"
    exit 1
  fi
  ok "Docker 环境正常"
}

# ---------------------------
# 获取当前使用的镜像 tag
# ---------------------------
get_current_image_tag() {
  local tag
  # 优先从运行中的容器获取
  tag=$(docker inspect --format='{{.Config.Image}}' "$CONTAINER_NAME" 2>/dev/null || true)
  if [ -n "$tag" ]; then
    echo "$tag"
    return
  fi
  # 其次从 docker-compose.yml 获取
  if [ -f "$PROJECT_ROOT/docker-compose.yml" ]; then
    tag=$(grep -oP "image:\s*\K${IMAGE_NAME}:\S+" "$PROJECT_ROOT/docker-compose.yml" 2>/dev/null | head -1)
    if [ -n "$tag" ]; then
      echo "$tag"
      return
    fi
  fi
  # 兜底用 latest
  echo "${IMAGE_NAME}:latest"
}

# ---------------------------
# 打包迁移 (export)
# ---------------------------
do_pack() {
  step "打包迁移 - 导出 Docker 镜像 + 数据卷 + 配置"

  # 输出文件名
  read -rp "$(echo -e "${CYAN}打包文件名${NC} [${DEFAULT_PACK_NAME}]: ")" PACK_NAME
  PACK_NAME="${PACK_NAME:-$DEFAULT_PACK_NAME}"

  # 输出目录
  local def_out="$PROJECT_ROOT"
  read -rp "$(echo -e "${CYAN}输出目录${NC} [${def_out}]: ")" OUT_DIR
  OUT_DIR="${OUT_DIR:-$def_out}"
  mkdir -p "$OUT_DIR"

  local PACK_PATH="${OUT_DIR}/${PACK_NAME}"
  local TMPDIR
  TMPDIR=$(mktemp -d)
  trap "rm -rf '$TMPDIR'" EXIT

  info "临时工作目录: $TMPDIR"

  # ---- 1. 导出 Docker 镜像 ----
  step "1/4 导出 Docker 镜像"

  local IMAGE_TAG
  IMAGE_TAG=$(get_current_image_tag)
  info "当前镜像: $IMAGE_TAG"

  if ! docker image inspect "$IMAGE_TAG" &>/dev/null; then
    err "镜像 $IMAGE_TAG 不存在，请先构建镜像"
    exit 1
  fi

  info "正在导出镜像 (可能需要几分钟)..."
  docker save "$IMAGE_TAG" -o "$TMPDIR/image.tar"
  ok "镜像已导出: $(du -sh "$TMPDIR/image.tar" | cut -f1)"

  # 记录镜像名
  echo "$IMAGE_TAG" > "$TMPDIR/image_tag.txt"

  # ---- 2. 导出数据卷 ----
  step "2/4 导出数据卷"

  # 停止容器以保证数据一致性
  local was_running=false
  if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    was_running=true
    warn "检测到容器正在运行，将暂停容器以保证数据一致性"
    read -rp "$(echo -e "${YELLOW}是否暂停容器进行导出?${NC} [Y/n]: ")" stop_confirm
    if [[ "$stop_confirm" =~ ^[Nn]$ ]]; then
      warn "跳过暂停，数据可能不一致"
    else
      docker stop "$CONTAINER_NAME"
      info "容器已暂停"
    fi
  fi

  # 从容器检测实际 volume 名称
  local vol_mapping
  vol_mapping=$(detect_volumes_from_container)

  if [ -z "$vol_mapping" ]; then
    warn "无法从容器 $CONTAINER_NAME 检测到 volume 挂载"
    warn "请确认容器名称是否正确，或容器是否存在"
  fi

  mkdir -p "$TMPDIR/volumes"
  : > "$TMPDIR/volumes.map"

  for mount in "${MOUNT_POINTS[@]}"; do
    local vol_name
    vol_name=$(find_volume_for_mount "$mount" "$vol_mapping")

    if [ -z "$vol_name" ]; then
      warn "挂载点 $mount 未找到对应 volume，跳过"
      continue
    fi

    # 用挂载点生成安全文件名作为 key (如 /app/backend/data -> app_backend_data)
    local safe_name
    safe_name=$(echo "$mount" | sed 's|^/||;s|/|_|g')

    info "导出卷: $vol_name ($mount)"
    docker run --rm \
      -v "${vol_name}:/source:ro" \
      -v "$TMPDIR/volumes:/backup" \
      alpine tar cf "/backup/${safe_name}.tar" -C /source .
    ok "$vol_name -> $(du -sh "$TMPDIR/volumes/${safe_name}.tar" | cut -f1)"

    # 记录映射: 挂载点=安全文件名
    echo "${mount}=${safe_name}" >> "$TMPDIR/volumes.map"
  done

  # 重启容器
  if [ "$was_running" = true ] && ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    info "重新启动容器..."
    docker start "$CONTAINER_NAME"
    ok "容器已恢复运行"
  fi

  # ---- 3. 导出配置文件 ----
  step "3/4 导出配置文件"

  mkdir -p "$TMPDIR/config"
  local config_dir="$PROJECT_ROOT/docker-build"

  if [ -d "$config_dir" ]; then
    cp -r "$config_dir"/* "$TMPDIR/config/" 2>/dev/null || true
    ok "docker-build/ 配置已导出"
  else
    warn "未找到 docker-build/ 目录"
  fi

  # 导出 docker-compose.yml
  if [ -f "$PROJECT_ROOT/docker-compose.yml" ]; then
    cp "$PROJECT_ROOT/docker-compose.yml" "$TMPDIR/docker-compose.yml"
    ok "docker-compose.yml 已导出"
  fi

  # 导出构建配置记录
  if [ -f "$PROJECT_ROOT/.build_last_config" ]; then
    cp "$PROJECT_ROOT/.build_last_config" "$TMPDIR/.build_last_config"
    ok ".build_last_config 已导出"
  fi

  # ---- 4. 打包 ----
  step "4/4 创建迁移包"

  # 写入元信息
  cat > "$TMPDIR/manifest.txt" <<EOF
AuraLogic Migration Pack
========================
Created: $(date '+%Y-%m-%d %H:%M:%S')
Image:   $IMAGE_TAG
Host:    $(hostname)
Volumes: ${MOUNT_POINTS[*]}
EOF

  info "正在压缩打包..."
  tar czf "$PACK_PATH" -C "$TMPDIR" .
  ok "迁移包已创建: $PACK_PATH"
  info "文件大小: $(du -sh "$PACK_PATH" | cut -f1)"

  echo ""
  echo "=========================================="
  echo -e "${GREEN}打包完成!${NC}"
  echo "=========================================="
  echo "迁移包: $PACK_PATH"
  echo ""
  echo "将此文件传输到目标服务器后执行:"
  echo "  bash docker_migrate.sh unpack $PACK_NAME"
  echo "=========================================="
}

# ---------------------------
# 解包释放 (import)
# ---------------------------
do_unpack() {
  local PACK_FILE="$1"

  step "解包释放 - 导入 Docker 镜像 + 数据卷 + 配置"

  # 获取迁移包路径
  if [ -z "$PACK_FILE" ]; then
    read -rp "$(echo -e "${CYAN}迁移包文件路径${NC}: ")" PACK_FILE
  fi

  # 支持相对路径
  if [[ "$PACK_FILE" != /* ]]; then
    PACK_FILE="$(pwd)/$PACK_FILE"
  fi

  if [ ! -f "$PACK_FILE" ]; then
    err "文件不存在: $PACK_FILE"
    exit 1
  fi

  info "迁移包: $PACK_FILE ($(du -sh "$PACK_FILE" | cut -f1))"

  local TMPDIR
  TMPDIR=$(mktemp -d)
  trap "rm -rf '$TMPDIR'" EXIT

  # ---- 解压 ----
  step "1/5 解压迁移包"
  tar xzf "$PACK_FILE" -C "$TMPDIR"
  ok "解压完成"

  # 显示元信息
  if [ -f "$TMPDIR/manifest.txt" ]; then
    echo ""
    cat "$TMPDIR/manifest.txt"
    echo ""
  fi

  read -rp "$(echo -e "${YELLOW}确认开始导入?${NC} [Y/n]: ")" confirm
  if [[ "$confirm" =~ ^[Nn]$ ]]; then
    info "已取消"
    exit 0
  fi

  # ---- 2. 导入 Docker 镜像 ----
  step "2/5 导入 Docker 镜像"

  if [ -f "$TMPDIR/image.tar" ]; then
    info "正在加载镜像 (可能需要几分钟)..."
    docker load -i "$TMPDIR/image.tar"
    ok "镜像导入完成"
  else
    err "迁移包中未找到 image.tar"
    exit 1
  fi

  local IMAGE_TAG
  IMAGE_TAG=$(cat "$TMPDIR/image_tag.txt" 2>/dev/null || echo "${IMAGE_NAME}:latest")
  info "镜像: $IMAGE_TAG"

  # ---- 3. 停止现有容器 ----
  step "3/5 停止现有容器"

  if docker ps -aq -f name="^${CONTAINER_NAME}$" | grep -q .; then
    warn "检测到已有 auralogic 容器"
    read -rp "$(echo -e "${YELLOW}是否停止并替换现有容器?${NC} [Y/n]: ")" replace_confirm
    if [[ "$replace_confirm" =~ ^[Nn]$ ]]; then
      err "已取消，请手动处理现有容器后重试"
      exit 1
    fi
    docker stop "$CONTAINER_NAME" 2>/dev/null || true
    docker rm "$CONTAINER_NAME" 2>/dev/null || true
    ok "旧容器已移除"
  else
    info "未检测到现有容器"
  fi

  # ---- 4. 恢复配置文件 ----
  step "4/6 恢复配置文件"

  # 选择部署目录
  local def_deploy="$PROJECT_ROOT"
  read -rp "$(echo -e "${CYAN}部署目录${NC} [${def_deploy}]: ")" DEPLOY_DIR
  DEPLOY_DIR="${DEPLOY_DIR:-$def_deploy}"
  mkdir -p "$DEPLOY_DIR"

  # 恢复 docker-build 配置
  if [ -d "$TMPDIR/config" ] && [ "$(ls -A "$TMPDIR/config" 2>/dev/null)" ]; then
    mkdir -p "$DEPLOY_DIR/docker-build"
    cp -r "$TMPDIR/config"/* "$DEPLOY_DIR/docker-build/"
    ok "配置文件已恢复到 $DEPLOY_DIR/docker-build/"
  fi

  # 恢复 .build_last_config
  if [ -f "$TMPDIR/.build_last_config" ]; then
    cp "$TMPDIR/.build_last_config" "$DEPLOY_DIR/.build_last_config"
    ok ".build_last_config 已恢复"
  fi

  # 恢复或生成 docker-compose.yml
  if [ -f "$TMPDIR/docker-compose.yml" ]; then
    cp "$TMPDIR/docker-compose.yml" "$DEPLOY_DIR/docker-compose.yml"
    # 更新镜像 tag
    sed -i "s|image: ${IMAGE_NAME}:.*|image: ${IMAGE_TAG}|" "$DEPLOY_DIR/docker-compose.yml"
    ok "docker-compose.yml 已恢复"
  else
    warn "迁移包中未找到 docker-compose.yml，将生成默认配置"
    cat > "$DEPLOY_DIR/docker-compose.yml" <<COMPEOF
services:
  auralogic:
    image: ${IMAGE_TAG}
    container_name: auralogic
    ports:
      - "80:80"
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
COMPEOF
    ok "默认 docker-compose.yml 已生成"
  fi

  # ---- 5. 导入数据卷 ----
  step "5/6 导入数据卷"

  if [ -d "$TMPDIR/volumes" ] && [ -f "$TMPDIR/volumes.map" ]; then
    # 先用 docker compose create 创建容器和 volume (不启动)
    info "创建容器以初始化 volume..."
    cd "$DEPLOY_DIR"
    docker compose create --no-start 2>/dev/null || docker compose up --no-start 2>/dev/null || true

    # 从新创建的容器检测实际 volume 名
    local new_mapping
    new_mapping=$(detect_volumes_from_container)

    if [ -z "$new_mapping" ]; then
      warn "无法从容器检测 volume，尝试直接匹配..."
    fi

    while IFS='=' read -r mount safe_name; do
      [ -z "$mount" ] && continue
      local tar_file="$TMPDIR/volumes/${safe_name}.tar"
      [ -f "$tar_file" ] || { warn "未找到 ${safe_name}.tar，跳过"; continue; }

      # 从新容器的挂载信息中找到对应 volume 名
      local target_vol
      target_vol=$(find_volume_for_mount "$mount" "$new_mapping")

      if [ -z "$target_vol" ]; then
        warn "挂载点 $mount 未找到目标 volume，跳过"
        continue
      fi

      info "恢复卷: $target_vol ($mount)"
      docker run --rm \
        -v "${target_vol}:/dest" \
        -v "$TMPDIR/volumes:/backup:ro" \
        alpine sh -c "rm -rf /dest/* /dest/..?* /dest/.[!.]* 2>/dev/null; tar xf /backup/${safe_name}.tar -C /dest"
      ok "卷已恢复: $target_vol"
    done < "$TMPDIR/volumes.map"
  else
    warn "迁移包中未找到数据卷或映射文件"
  fi

  # ---- 6. 启动容器 ----
  step "6/6 启动容器"

  read -rp "$(echo -e "${CYAN}是否立即启动容器?${NC} [Y/n]: ")" start_confirm
  if [[ ! "$start_confirm" =~ ^[Nn]$ ]]; then
    cd "$DEPLOY_DIR"
    docker compose up -d
    ok "容器已启动"
  fi

  echo ""
  echo "=========================================="
  echo -e "${GREEN}解包完成!${NC}"
  echo "=========================================="
  echo "部署目录:       $DEPLOY_DIR"
  echo "Docker 镜像:    $IMAGE_TAG"
  echo "docker-compose: $DEPLOY_DIR/docker-compose.yml"
  echo ""
  echo "管理命令:"
  echo "  cd $DEPLOY_DIR"
  echo "  docker compose up -d      # 启动"
  echo "  docker compose down       # 停止"
  echo "  docker compose logs -f    # 查看日志"
  echo "=========================================="
}

# ---------------------------
# 查看迁移包信息
# ---------------------------
do_info() {
  local PACK_FILE="$1"

  if [ -z "$PACK_FILE" ]; then
    read -rp "$(echo -e "${CYAN}迁移包文件路径${NC}: ")" PACK_FILE
  fi

  if [[ "$PACK_FILE" != /* ]]; then
    PACK_FILE="$(pwd)/$PACK_FILE"
  fi

  if [ ! -f "$PACK_FILE" ]; then
    err "文件不存在: $PACK_FILE"
    exit 1
  fi

  info "文件: $PACK_FILE"
  info "大小: $(du -sh "$PACK_FILE" | cut -f1)"
  echo ""

  # 提取并显示 manifest
  tar xzf "$PACK_FILE" --to-stdout ./manifest.txt 2>/dev/null || warn "未找到 manifest 信息"

  echo ""
  info "包含的数据卷:"
  local map_content
  map_content=$(tar xzf "$PACK_FILE" --to-stdout ./volumes.map 2>/dev/null || true)
  if [ -n "$map_content" ]; then
    echo "$map_content" | while IFS='=' read -r mount safe_name; do
      [ -n "$mount" ] && echo "  - $mount ($safe_name.tar)"
    done
  else
    tar tzf "$PACK_FILE" 2>/dev/null | grep '^./volumes/' | sed 's|./volumes/||;s|\.tar$||' | while read -r v; do
      [ -n "$v" ] && echo "  - $v"
    done
  fi

  echo ""
  info "包含的配置文件:"
  tar tzf "$PACK_FILE" 2>/dev/null | grep '^./config/' | sed 's|./config/||' | while read -r f; do
    [ -n "$f" ] && echo "  - $f"
  done
}

# ---------------------------
# 主流程
# ---------------------------
main() {
  echo ""
  echo "=========================================="
  echo "  AuraLogic Docker 迁移工具"
  echo "=========================================="
  echo ""

  check_docker

  local action="$1"
  local arg="$2"

  if [ -z "$action" ]; then
    echo "请选择操作:"
    echo "  1) 打包迁移 (导出镜像+数据+配置)"
    echo "  2) 解包释放 (导入镜像+数据+配置)"
    echo "  3) 查看迁移包信息"
    read -rp "请选择 [1]: " action_choice
    action_choice="${action_choice:-1}"

    case "$action_choice" in
      1) action="pack" ;;
      2) action="unpack" ;;
      3) action="info" ;;
      *) err "无效选择"; exit 1 ;;
    esac
  fi

  case "$action" in
    pack|export)
      do_pack
      ;;
    unpack|import)
      do_unpack "$arg"
      ;;
    info)
      do_info "$arg"
      ;;
    *)
      err "未知操作: $action"
      echo ""
      echo "用法: $0 [pack|unpack|info] [file]"
      echo ""
      echo "  pack            打包当前环境为迁移包"
      echo "  unpack <file>   从迁移包恢复环境"
      echo "  info <file>     查看迁移包信息"
      exit 1
      ;;
  esac
}

main "$@"
