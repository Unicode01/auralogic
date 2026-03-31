# 部署指南

本文档只说明主分支当前实际包含的部署方式与必改配置。

## 部署范围

主分支包含：

- `backend` 后端 API 与插件宿主
- `frontend` 前端应用
- `scripts/build_docker.sh` 一键构建脚本
- `plugins/js_market` 与 `plugins/sdk`

主分支不包含市场注册表、官方模板包、支付包与插件示例目录；这些内容位于派生分支。

## 推荐方式：一键脚本

在仓库根目录执行：

```bash
bash scripts/build_docker.sh
```

脚本会引导生成运行配置并构建 Docker 镜像。后续启动：

```bash
docker compose up -d
```

若代码更新后需要重新构建：

```bash
bash scripts/build_docker.sh update
```

## 手动部署

### 1. 后端

先基于示例配置准备生产配置：

```bash
cd backend
cp config/config.prod.example.json config/config.prod.json
```

建议显式指定配置文件启动：

```bash
CONFIG_FILE=config/config.prod.json ./auralogic
```

典型构建方式：

```bash
cd backend
go build -o auralogic cmd/api/main.go
CONFIG_FILE=config/config.prod.json ./auralogic
```

首次部署前记得初始化管理员账号：

```bash
cd backend
go run scripts/init_admin.go
```

### 2. 前端

生产环境不要依赖仓库中的默认回退值，务必显式设置环境变量：

```bash
cd frontend
export NEXT_PUBLIC_API_URL=https://api.yourdomain.com
export NEXT_PUBLIC_APP_URL=https://yourdomain.com
npm install
npm run build
npm run start
```

如果前后端同域反代，也至少应显式设置 `NEXT_PUBLIC_API_URL`。

## 必改配置

部署前至少检查以下配置：

- `backend/config/config.prod.json`
  - `app.url`
  - `database`
  - `redis`
  - `jwt.secret`
  - `security.cors.allowed_origins`
  - `smtp`
  - `oauth.google.redirect_url`
  - `oauth.github.redirect_url`
- `backend/config/admin.json`
  - 初始管理员邮箱、密码、名称
- 前端环境变量
  - `NEXT_PUBLIC_API_URL`
  - `NEXT_PUBLIC_APP_URL`

## 安全检查

- `jwt.secret` 使用至少 32 位随机强密钥
- `security.cors.allowed_origins` 只保留实际生产域名
- `app.debug` 关闭
- 数据库、Redis、SMTP 使用生产凭据
- OAuth 回调地址与生产域名一致
- 站点启用 HTTPS
- 日志与数据库做好备份

## 验证清单

部署完成后建议至少验证：

1. 用户登录 / 注册 / 找回密码
2. 订单创建、支付、订单详情
3. 管理后台登录与关键页面加载
4. 文件上传与图片访问
5. SMTP / SMS / OAuth 等已启用能力
6. 反向代理后真实 IP 获取是否正常

## 补充说明

- `backend/config/config.prod.json` 可以作为本地样板，但不应直接照搬仓库中的域名、邮箱和凭据。
- 若使用 Docker / Nginx / Supervisor 组合，根目录 `scripts/docker/` 下提供了对应资源。
- 若你需要连同市场注册表、官方模板包或插件示例一起部署，请切换到对应派生分支查看说明。
