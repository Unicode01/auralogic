# AuraLogic - 前端

基于 Next.js 15 + TypeScript + Tailwind CSS + shadcn/ui 构建的现代化订单管理系统前端应用。

## 📚 技术栈

- **框架**: Next.js 15 (App Router)
- **语言**: TypeScript 5
- **样式**: Tailwind CSS 3
- **UI组件**: shadcn/ui + Radix UI
- **状态管理**: @tanstack/react-query
- **表单管理**: React Hook Form + Zod
- **HTTP客户端**: Axios
- **图标**: Lucide React

## 🚀 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 配置环境变量

编辑 `.env.local` 配置API地址：

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_APP_URL=http://localhost:3000
```

### 3. 启动开发服务器

```bash
npm run dev
```

访问 http://localhost:3000

### 4. 构建生产版本

```bash
npm run build
npm run start
```

## 📁 项目结构

```
frontend/
├── app/                    # Next.js App Router
│   ├── (auth)/            # 认证路由组
│   ├── (user)/            # 用户路由组
│   ├── (admin)/           # 管理员路由组
│   └── form/              # 表单路由
├── components/            # React 组件
│   ├── ui/               # shadcn 基础组件
│   ├── layout/           # 布局组件
│   ├── orders/           # 订单组件
│   ├── forms/            # 表单组件
│   └── admin/            # 管理组件
├── lib/                  # 工具函数
│   ├── api.ts           # API 客户端
│   ├── auth.ts          # 认证工具
│   ├── utils.ts         # 通用工具
│   ├── constants.ts     # 常量定义
│   └── validators.ts    # 表单验证
├── hooks/                # 自定义 Hooks
├── types/                # TypeScript 类型定义
└── public/              # 静态资源
```

## 🎯 主要功能

### 用户端
- ✅ 邮箱/密码登录
- ✅ OAuth 登录（可配置第三方平台）
- ✅ 订单列表查看
- ✅ 订单详情查看
- ✅ 订单状态跟踪
- ✅ 物流信息查询
- ✅ 购物车功能
- ✅ 工单/客服中心
- ✅ 知识库浏览
- ✅ 公告系统

### 管理端
- ✅ 管理员仪表板
- ✅ 订单管理
- ✅ 用户管理
- ✅ 商品管理（实体 + 虚拟）
- ✅ 库存管理（实体库存 + 虚拟库存）
- ✅ 权限管理
- ✅ API密钥管理
- ✅ 隐私保护订单查看
- ✅ 付款方式管理
- ✅ 优惠码管理
- ✅ 工单管理
- ✅ 知识库管理
- ✅ 公告管理
- ✅ 数据分析
- ✅ 系统设置

### 表单功能
- ✅ 发货信息填写
- ✅ 表单验证
- ✅ 隐私保护选项
- ✅ 自动创建用户账号

## 🔧 开发命令

```bash
# 开发
npm run dev

# 构建
npm run build

# 生产环境运行
npm run start

# 代码检查
npm run lint

# 类型检查
npm run type-check

# 代码格式化
npm run format

# 测试
npm run test
```

## 📖 文档

详细文档请查看 `/docs` 目录：

- [API 接口文档](../docs/API.md)
- [付款方式 JS API](../docs/PAYMENT_JS_API.md)

## 🎨 UI 组件

项目使用 [shadcn/ui](https://ui.shadcn.com) 组件库。添加新组件：

```bash
# 添加单个组件
npx shadcn-ui@latest add button

# 添加多个组件
npx shadcn-ui@latest add button card dialog form
```

## 🔐 权限管理

系统支持基于角色的权限管理：

- **user**: 普通用户，可查看自己的订单
- **admin**: 管理员，可管理所有订单和用户
- **super_admin**: 超级管理员，拥有所有权限

## 🚀 部署

### Vercel 部署

```bash
# 安装 Vercel CLI
npm i -g vercel

# 部署
vercel
```

### Docker 部署

```bash
# 构建镜像
docker build -t order-frontend .

# 运行容器
docker run -p 3000:3000 order-frontend
```

## 📝 注意事项

1. 确保后端 API 服务已启动
2. 配置正确的 API 地址
3. 生产环境需要配置正确的域名
4. OAuth 登录需要在后端配置相应的 OAuth 平台（如 Google、Github）

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License

