# AuraLogic

<div align="center">

**自托管的全功能电商平台，支持实体与虚拟商品销售**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Next.js](https://img.shields.io/badge/Next.js-14+-000000?style=flat&logo=next.js)](https://nextjs.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)]()

</div>

---

## 系统概述

AuraLogic 是一个自托管的电商平台，支持实体商品和虚拟商品（激活码、序列号等）的销售与管理。系统提供商品展示、购物车、多种支付方式、库存管理、订单处理、客服工单、知识库等完整电商功能，同时支持通过 API 与第三方平台对接。

---

## 核心功能

### 商品与库存

- **实体商品** - 支持多属性（颜色、尺寸等），需发货的传统商品
- **虚拟商品** - 激活码、序列号等数字商品，支持自动发货
- **库存管理** - 独立库存池，支持多商品共享库存、安全库存预警
- **盲盒模式** - 随机分配商品属性，支持按优先级分配
- **虚拟库存** - 批量导入虚拟库存项，支持预留/释放机制
- **商品序列号** - 防伪序列号生成与公开验证

### 购物与订单

- **购物车** - 按属性区分的购物车系统
- **订单管理** - 用户下单、管理员代下单、第三方 API 创建草稿订单
- **发货表单** - 通过安全链接填写收货信息，支持隐私保护模式
- **物流跟踪** - 物流单号分配与展示
- **优惠码** - 百分比/固定金额折扣，支持商品范围限制和有效期

### 支付系统

- **多种支付方式** - 内置 USDT (TRC20/BEP20) 自动确认
- **可扩展支付** - 通过 JavaScript 脚本自定义支付方式
- **支付轮询** - 自动检测支付状态

### 用户系统

- **多种登录方式** - 邮箱密码、OAuth（Google/GitHub）、快速登录链接
- **自动注册** - 填写发货表单时自动创建账号
- **隐私保护** - 用户可选择隐藏收货信息，仅发货管理员可见

### 客服与内容

- **工单系统** - 用户提交工单、消息对话、文件上传、订单共享
- **知识库** - 分类树结构的帮助文档
- **公告系统** - 系统公告发布，支持强制阅读确认

### 管理后台

- **RBAC 权限** - 细粒度权限控制（订单、商品、用户、隐私查看等）
- **数据分析** - 用户、订单、收入、设备、页面访问统计
- **API 密钥** - 第三方平台 API 接入管理，支持速率限制
- **系统设置** - SMTP 邮件、登录策略、验证码、落地页编辑等
- **操作日志** - 管理员操作记录、邮件发送日志

### API 接入

第三方平台通过 API 密钥（`X-API-Key` + `X-API-Secret`）访问管理端接口（`/api/admin/*`），按分配的权限范围（scopes）操作订单、商品、用户等资源。管理员在后台创建 API 密钥时指定允许的权限和速率限制。

---

## 技术栈

**后端**: Go + Gin + GORM，支持 PostgreSQL / MySQL / SQLite

**前端**: Next.js 14 (App Router) + TypeScript + Tailwind CSS + shadcn/ui

**基础设施**: Redis 缓存 + JWT 认证 + OAuth2

---

## 快速开始

### 一键 Docker 部署（推荐）

```bash
git clone https://github.com/Unicode01/auralogic
cd auralogic
bash scripts/build_docker.sh
```

脚本会交互式引导完成配置（数据库类型、JWT 密钥、OAuth、SMTP、管理员账号等），自动构建 Docker 镜像并生成 `docker-compose.yml`。

```bash
# 启动
docker compose up -d

# 更新（代码更新后，无需重新配置）
bash scripts/build_docker.sh update
```

容器内集成 Nginx + Backend + Frontend + Redis，由 Supervisor 管理进程。

### 手动部署

#### 后端

```bash
cd backend
cp config/config.example.json config/config.json
# 编辑 config/config.json 和 config/admin.json
go mod download
go run scripts/init_admin.go
go run cmd/api/main.go --config=config/config.json
```

#### 前端

```bash
cd frontend
pnpm install
# 编辑 .env.local 设置 API 地址
pnpm dev
```

- 用户端: http://localhost:3000
- 管理后台: http://localhost:3000/admin
- 默认管理员: 见 `backend/config/admin.json`

---

## 项目结构

```
.
├── backend/                    # Go 后端
│   ├── cmd/api/               # 应用入口
│   ├── internal/
│   │   ├── config/            # 配置定义
│   │   ├── models/            # 数据模型
│   │   ├── repository/        # 数据访问层
│   │   ├── service/           # 业务逻辑层
│   │   ├── handler/           # HTTP 处理器
│   │   │   ├── admin/         # 管理端接口
│   │   │   ├── user/          # 用户端接口
│   │   │   ├── api/           # 第三方 API 接口
│   │   │   └── form/          # 表单接口
│   │   ├── middleware/        # 中间件
│   │   ├── database/          # 数据库初始化
│   │   ├── router/            # 路由配置
│   │   └── pkg/               # 工具包
│   ├── scripts/               # 初始化脚本
│   └── config/                # 配置文件
│
├── frontend/                   # Next.js 前端
│   ├── app/
│   │   ├── (auth)/            # 登录/注册
│   │   ├── (user)/            # 用户端（商品、购物车、订单、工单、知识库、公告）
│   │   ├── (admin)/           # 管理后台
│   │   ├── form/              # 发货表单
│   │   └── serial-verify/     # 序列号验证
│   ├── components/            # React 组件
│   ├── lib/                   # API 客户端、工具函数
│   ├── hooks/                 # 自定义 Hooks
│   └── types/                 # TypeScript 类型
│
├── scripts/                    # 脚本工具
│   ├── build_docker.sh        # 一键构建脚本
│   ├── migrate_dujiaoka.py    # 独角数卡数据迁移脚本
│   └── docker/                # Dockerfile、Nginx、Supervisor 配置
│
├── docs/                       # 文档
│   ├── API.md                 # API 接口文档
│   └── PAYMENT_JS_API.md      # 自定义支付方式开发文档
│
└── DEPLOYMENT.md              # 部署指南
```

---

## 配置说明

### 后端配置 (config/config.json)

```json
{
  "app": { "env": "production", "port": 8080, "url": "https://yourdomain.com" },
  "database": { "driver": "sqlite", "name": "auralogic.db" },
  "redis": { "host": "localhost", "port": 6379 },
  "jwt": { "secret": "至少32字符的密钥", "expire_hours": 24 },
  "security": {
    "login": { "allow_password_login": false },
    "captcha": { "provider": "none" }
  },
  "smtp": { "enabled": true, "host": "smtp.gmail.com", "port": 587 },
  "oauth": {
    "google": { "enabled": false, "client_id": "", "client_secret": "" },
    "github": { "enabled": false, "client_id": "", "client_secret": "" }
  }
}
```

**登录策略** (`allow_password_login`):
- `false` - 普通用户仅限快速登录/OAuth（推荐生产环境）
- `true` - 所有用户可密码登录
- 超级管理员始终可密码登录

### 初始管理员 (config/admin.json)

```json
{
  "super_admin": {
    "email": "admin@yourdomain.com",
    "password": "ChangeMe123!",
    "name": "超级管理员"
  }
}
```

---

## 从独角数卡迁移

提供 Python 脚本将 [独角数卡 (Dujiaoka)](https://github.com/assimon/dujiaoka) 的数据迁移到 AuraLogic。

**迁移内容**:
- 商品 → AuraLogic 商品（保留名称、价格、图片、分类、购买限制等）
- 卡密 → 虚拟库存（支持普通卡密批量导入和循环卡密单条导入并预留）
- 优惠码 → 促销码（保留折扣金额、关联商品、使用次数）

**安装依赖**:

```bash
pip install pymysql requests rich
```

**使用示例**:

```bash
# 预览模式（不写入数据，推荐先执行）
python scripts/migrate_dujiaoka.py --dry-run \
  --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \
  --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx

# 正式迁移
python scripts/migrate_dujiaoka.py \
  --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \
  --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx

# 仅迁移商品（跳过卡密和优惠码）
python scripts/migrate_dujiaoka.py --no-carmis --no-coupons \
  --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \
  --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx
```

**常用参数**:

| 参数 | 说明 |
|------|------|
| `--dry-run` | 预览模式，不实际写入数据 |
| `--no-products` | 跳过商品迁移 |
| `--no-carmis` | 跳过卡密迁移 |
| `--no-coupons` | 跳过优惠码迁移 |
| `--skip-disabled` | 跳过已禁用的商品和分类 |
| `--batch-size N` | 卡密批量导入大小（默认 50） |
| `--product-status` | 导入商品的初始状态：`draft`（默认）/ `active` / `inactive` |

迁移完成后会生成 `migration_mapping.json` 文件，记录独角数卡 ID 与 AuraLogic ID 的映射关系。

---

## 文档

| 文档 | 说明 |
|------|------|
| [API 接口文档](docs/API.md) | 完整的 API 端点参考 |
| [自定义支付方式](docs/PAYMENT_JS_API.md) | 支付方式 JS 脚本开发 |
| [部署指南](DEPLOYMENT.md) | 生产环境部署配置 |
| [后端文档](backend/README.md) | 后端开发说明 |
| [前端文档](frontend/README.md) | 前端开发说明 |

---

## 许可证

MIT License
