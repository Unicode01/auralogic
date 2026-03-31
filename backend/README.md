# AuraLogic Backend - 订单和发货管理系统

基于Go + Gin + GORM的订单和发货管理系统后端API服务。

## ✨ 核心特性

- ✅ 完整的订单管理流程
- ✅ 灵活的权限系统（RBAC）
- ✅ 邮件通知功能
- ✅ OAuth通用支持（可扩展第三方平台）
- ✅ SQLite本地调试支持
- ✅ API密钥管理
- ✅ 隐私保护功能
- ✅ Docker部署支持

## 技术栈

- **语言**: Go 1.24+
- **Web框架**: Gin
- **ORM**: GORM
- **数据库**: PostgreSQL / MySQL / SQLite
- **缓存**: Redis
- **认证**: JWT + OAuth2.0
- **密码加密**: bcrypt
- **邮件**: SMTP

## 项目结构

```
backend/
├── cmd/
│   └── api/              # API服务入口
│       └── main.go
├── internal/
│   ├── config/           # 配置管理
│   ├── models/           # 数据模型
│   ├── repository/       # 数据访问层
│   ├── service/          # 业务逻辑层
│   ├── handler/          # HTTP处理器
│   │   ├── api/          # 外部API（第三方平台）
│   │   ├── admin/        # 管理员接口
│   │   ├── user/         # 用户接口
│   │   └── form/         # 表单接口
│   ├── middleware/       # 中间件
│   ├── pkg/              # 工具包
│   ├── database/         # 数据库初始化
│   └── router/           # 路由配置
├── scripts/              # 脚本
│   └── init_admin.go     # 初始化超级管理员
├── config/               # 配置文件
│   ├── config.json       # 主配置（需创建）
│   └── admin.json        # 管理员配置（需创建）
├── go.mod
├── go.sum
└── README.md
```

## 快速开始

### 最简单的方式（SQLite）⚡

```bash
cd backend
go mod download
cp config/sqlite.example.json config/config.json
# 编辑 config/admin.json 设置管理员密码
go run scripts/init_admin.go
go run cmd/api/main.go
```

✅ 完成！服务已运行在 http://localhost:8080

### 使用PostgreSQL/MySQL

1. **准备数据库**
```bash
createdb auralogic  # PostgreSQL
# 或
mysql -e "CREATE DATABASE auralogic"  # MySQL
```

2. **准备Redis**
```bash
redis-server
```

3. **配置文件**
```bash
cp config/config.example.json config/config.json
# 编辑配置文件
```

4. **初始化并启动**
```bash
go run scripts/init_admin.go
go run cmd/api/main.go
```

### Docker一键启动

```bash
docker-compose up -d
```

## 📚 文档

- [API 总览](../docs/API.md) - 手工维护的接口概览，最新实现请以代码与测试为准 📖
- [付款方式 JS API](../docs/PAYMENT_JS_API.md) - 自定义付款方式脚本开发 💳
- [部署指南](../DEPLOYMENT.md) - 生产环境部署配置 🚀

### 主要API端点

完整API文档请查看 [API.md](../docs/API.md)

#### 外部API（第三方平台）- API Key认证
- 订单管理（创建、查询、更新）
- 物流单号分配
- 要求重填信息

#### 用户端API - JWT认证
- 用户登录和认证
- 订单查询和管理
- 确认订单完成

#### 管理员API - JWT + 权限认证
- 订单管理（查看、编辑、分配物流）
- 用户管理（查看、编辑、权限分配）
- API密钥管理
- 权限管理

#### 表单API - 安全令牌 / 登录用户
- 获取发货信息表单
- 提交发货信息（自动创建用户）

## 配置说明

### 主配置文件 (config/config.json)

```json
{
  "app": {
    "name": "AuraLogic",
    "env": "development",
    "port": 8080,
    "url": "http://localhost:3000"
  },
  "database": {
    "driver": "postgres",
    "host": "localhost",
    "port": 5432,
    "name": "auralogic",
    "user": "postgres",
    "password": "your_password"
  },
  "redis": {
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0
  },
  "jwt": {
    "secret": "your-jwt-secret-min-32-characters",
    "expire_hours": 24
  },
  "security": {
    "login": {
      "allow_password_login": true
    }
  }
}
```

### 管理员配置文件 (config/admin.json)

```json
{
  "super_admin": {
    "email": "admin@yourdomain.com",
    "password": "ChangeMe123!",
    "name": "超级管理员"
  }
}
```

## 开发指南

### 添加新的API端点

1. 在 `internal/handler/` 目录下创建Handler
2. 在 `internal/router/router.go` 中注册路由
3. 如需数据库操作，在 `internal/repository/` 中添加Repository方法
4. 如需业务逻辑，在 `internal/service/` 中添加Service方法

### 数据库迁移

GORM会自动执行数据库迁移。如需手动迁移：

```go
database.AutoMigrate()
```

### 日志记录

系统使用标准的Go log包。在生产环境建议使用结构化日志工具（如logrus、zap）。

## 部署

### Docker部署

```bash
# 构建镜像
docker build -t auralogic-backend .

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  --name auralogic-api \
  auralogic-backend
```

### 二进制部署

```bash
# 编译
go build -o bin/api cmd/api/main.go

# 运行
./bin/api
```

## 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/service/...

# 运行测试并显示覆盖率
go test -cover ./...
```

## 安全注意事项

1. **密码加密**: 所有密码使用bcrypt加密存储
2. **JWT密钥**: 确保JWT密钥至少32个字符，生产环境使用强随机密钥
3. **API密钥**: 妥善保管API密钥，定期轮换
4. **HTTPS**: 生产环境必须使用HTTPS
5. **CORS**: 根据实际需求配置CORS白名单
6. **限流**: 启用限流功能防止滥用

## 故障排查

### 数据库连接失败

检查数据库配置是否正确，确保数据库服务正在运行。

### Redis连接失败

检查Redis配置是否正确，确保Redis服务正在运行。

### JWT验证失败

检查JWT密钥配置是否正确，确保客户端使用正确的token格式。

## 许可证

MIT License

## 相关文档

- [API接口文档](../docs/API.md)
- [付款方式JS API](../docs/PAYMENT_JS_API.md)
- [部署指南](../DEPLOYMENT.md)

