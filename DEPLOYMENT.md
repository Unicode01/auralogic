# 生产环境部署指南

## 🌐 域名配置

**生产域名**: `https://auralogic.un1c0de.com`

## 📋 配置清单

### 1. 后端配置

#### 使用生产配置文件

```bash
cd backend
# 将生产配置复制为正式配置
cp config/config.prod.json config/config.json
```

或者在启动时指定配置文件：

```bash
CONFIG_FILE=config/config.prod.json go run cmd/api/main.go
```

#### 关键配置项

**config/config.prod.json**:
- ✅ `app.url`: `https://auralogic.un1c0de.com`
- ✅ `app.env`: `production`
- ✅ `app.debug`: `false`
- ✅ `oauth.google.redirect_url`: 设置为生产环境回调地址
- ✅ `security.cors.allowed_origins`: `["https://auralogic.un1c0de.com"]`
- ✅ `rate_limit.enabled`: `true`
- ✅ `log.level`: `info`
- ⚠️  `jwt.secret`: **必须修改为强密码！**

### 2. 前端配置

#### Next.js 环境变量

**方法一：通过 .env.local（本地覆盖）**

创建 `frontend/.env.local` 文件：
```bash
NEXT_PUBLIC_API_URL=https://auralogic.un1c0de.com
```

**方法二：通过系统环境变量**

```bash
export NEXT_PUBLIC_API_URL=https://auralogic.un1c0de.com
```

**方法三：通过 next.config.js（已配置）**

默认值已设置为生产域名，无需额外配置。

#### 本地开发切换

如果需要在本地开发，创建 `.env.local`:
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### 3. OAuth 配置

如果启用了 OAuth 登录（Google/Github），需要在对应平台更新回调 URL 为生产环境地址。

## 🚀 部署步骤

### 后端部署

```bash
cd backend

# 1. 构建
go build -o auralogic cmd/api/main.go

# 2. 运行（使用生产配置）
CONFIG_FILE=config/config.prod.json ./auralogic

# 或者使用 systemd 服务（推荐）
sudo systemctl start auralogic
```

### 前端部署

```bash
cd frontend

# 1. 设置环境变量
export NEXT_PUBLIC_API_URL=https://auralogic.un1c0de.com

# 2. 构建
npm run build

# 3. 启动
npm start

# 或者使用 PM2（推荐）
pm2 start npm --name "auralogic-frontend" -- start
```

## 🔒 安全检查清单

- [ ] 修改 JWT secret 为强随机密码（至少32字符）
- [ ] 更新 OAuth 回调 URL 为生产环境地址
- [ ] 配置 HTTPS 证书（Let's Encrypt）
- [ ] 启用速率限制（rate_limit.enabled: true）
- [ ] 配置 SMTP 邮件服务
- [ ] 检查 CORS 配置（只允许生产域名）
- [ ] 关闭调试模式（app.debug: false）
- [ ] 配置数据库备份

## 📝 已更新的文件

### 后端
- ✅ `backend/config/config.json` - 更新为生产URL
- ✅ `backend/config/config.prod.json` - 新建生产配置模板

### 前端
- ✅ `frontend/next.config.js` - 添加域名到 images.domains
- ✅ `frontend/next.config.js` - 设置默认 API URL
- ✅ `frontend/lib/api.ts` - 更新默认 API URL
- ✅ `frontend/components/forms/shipping-form.tsx` - 更新 API URL
- ✅ `frontend/app/(admin)/admin/orders/page.tsx` - 更新导出/导入 URL
- ✅ `frontend/app/(admin)/admin/orders/[id]/page.tsx` - 更新 API URL

## 🔄 回滚到本地开发

如果需要切回本地开发：

**后端**:
```bash
cd backend
git checkout config/config.json
# 或者手动改回 localhost
```

**前端**:
```bash
cd frontend
# 创建 .env.local
echo "NEXT_PUBLIC_API_URL=http://localhost:8080" > .env.local
```

## 🧪 测试

部署后测试以下功能：

1. ✅ 用户登录/注册
2. ✅ OAuth 登录（回调 URL 是否正确）
3. ✅ 商品浏览和购买
4. ✅ 订单创建和管理
5. ✅ 管理后台访问
6. ✅ 图片上传（URL 是否正确）

## 📞 技术支持

如有问题，请检查：
- 后端日志：`backend/logs/app.log`
- 前端日志：浏览器控制台
- Nginx/反向代理配置
- 防火墙规则

