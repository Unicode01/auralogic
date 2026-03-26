# AuraLogic Market Registry

`market_registry/` 是 AuraLogic 市场源的唯一实现目录，也是市场相关的唯一文档入口。

## 它负责什么

- 对外提供市场索引与下载入口，例如 `source.json`、catalog、artifact download
- 托管嵌入式管理后台，前端 build 后由 Go 后端 `embed` 并统一分发
- 维护制品版本、签名、channel、元数据与发布记录
- 支持从本地 ZIP、GitHub Release 导入制品
- 支持把 S3 / R2 作为底层对象存储，市场侧自己负责索引与治理

当前推荐架构：

- `market_registry` 负责索引、签名、版本治理、下载前门
- `GitHub Release` 可作为同步来源
- `S3/R2` 仅作为 canonical storage backend

## 快速开始

### 本地运行

```bash
cd market_registry
go run ./cmd/market-registry-cli keygen --key-id auralogic

cd admin
npm install
npm run build

cd ..
go run ./cmd/market-registry-api
```

默认地址：

- 市场 API: `http://localhost:18080/v1/source.json`
- 管理后台: `http://localhost:18080/admin/ui/`

### 管理前端开发模式

```bash
cd market_registry/admin
npm install
npm start
```

说明：

- 开发态前端默认请求 `http://localhost:18080`
- 生产态前端构建产物输出到 `internal/adminui/dist/`
- 后端编译时会直接嵌入这些静态资源

### Docker 构建与更新

```bash
cd market_registry
bash ./scripts/build_docker.sh build
bash ./scripts/build_docker.sh update
```

脚本会自动生成镜像、环境文件、`docker-compose.yml`，并在首次启动时补齐签名 key。

说明：

- Docker 只打包 `market_registry` 自身运行所需文件

## 常用工作流

### 发布本地 ZIP

```bash
go run ./cmd/market-registry-cli publish \
  --kind plugin_package \
  --name hello-market \
  --version 1.0.0 \
  --artifact plugin.zip \
  --metadata metadata.json
```

### 重建索引

```bash
go run ./cmd/market-registry-cli reindex
```

兼容别名 `repair` 仍可用。

### 从 GitHub Release 同步

```bash
go run ./cmd/market-registry-cli sync github-release \
  --owner auralogic \
  --repo market-packages \
  --tag v1.0.0 \
  --asset hello-market-1.0.0.zip \
  --metadata metadata.json
```

说明：

- 会下载 release asset 并写入市场自身存储
- 若未显式提供 `--metadata`，会尽量从 ZIP 内 `manifest.json` 与 release 元数据补齐字段
- 若当前目录存在 `manifest.json`，CLI 会自动补空 `kind/name/version/title/summary/description`
- 私有仓库可通过 `MARKET_REGISTRY_GITHUB_TOKEN` 或 `--token-env` 提供 token

### 审计与查看配置

```bash
go run ./cmd/market-registry-cli audit
go run ./cmd/market-registry-cli audit --json
go run ./cmd/market-registry-cli audit --strict
go run ./cmd/market-registry-cli config
go run ./cmd/market-registry-cli config --api --json
go run ./cmd/market-registry-cli config --shared
```

## 管理后台

管理后台支持：

- 发布 ZIP 制品
- 查看 catalog / 版本 / channel
- 查看配置与运行状态

生产态访问路径固定为 `/admin/ui/`。

## 关键环境变量

推荐使用以下命名：

- `MARKET_REGISTRY_ADDR`
- `MARKET_REGISTRY_BASE_URL`
- `MARKET_REGISTRY_DATA_DIR`
- `MARKET_REGISTRY_KEY_DIR`
- `MARKET_REGISTRY_KEY_ID`
- `MARKET_REGISTRY_ID`
- `MARKET_REGISTRY_NAME`
- `MARKET_REGISTRY_CHANNEL`
- `MARKET_REGISTRY_ADMIN_USERNAME`
- `MARKET_REGISTRY_ADMIN_PASSWORD`
- `MARKET_REGISTRY_ADMIN_PASSWORD_HASH`
- `MARKET_REGISTRY_AUTH_TOKEN_TTL`
- `MARKET_REGISTRY_STORAGE_TYPE`
- `MARKET_REGISTRY_STORAGE_S3_ENDPOINT`
- `MARKET_REGISTRY_STORAGE_S3_REGION`
- `MARKET_REGISTRY_STORAGE_S3_BUCKET`
- `MARKET_REGISTRY_STORAGE_S3_PREFIX`
- `MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID`
- `MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY`
- `MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN`
- `MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE`

兼容别名仍保留：

- `SOURCE_API_ADDR`
- `SOURCE_API_BASE_URL`
- `SOURCE_ADMIN_USERNAME`
- `SOURCE_ADMIN_PASSWORD`
- `SOURCE_ADMIN_PASSWORD_HASH`
- `SOURCE_AUTH_TOKEN_TTL`
- `SOURCE_ID`
- `SOURCE_NAME`
- `DATA_DIR`
- `KEY_DIR`
- `KEY_ID`
- `CHANNEL`
- `BASE_URL`

## 说明

- 统一使用 `go run ./cmd/market-registry-api` 和 `go run ./cmd/market-registry-cli`
- 新增或修改市场能力时，以本文件为准，不再拆分额外 market 文档
