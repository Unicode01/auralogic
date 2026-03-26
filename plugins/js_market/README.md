# AuraLogic JS Market Plugin

官方市场插件，运行在 AuraLogic `js_worker` 运行时中。

当前 MVP 已支持：

- 浏览受信市场源并检查源详情
- 查询插件包、支付包、邮件模板、落地页模板目录
- 查询账单模板、认证品牌模板、页面规则包目录
- 查询 artifact 详情与版本清单
- 查询 artifact / raw release 详情
- 预览 host install / import release
- 安装插件包
- 通过原生支付方式导入流程接入支付包
- 通过原生 `Plugin.emailTemplate` / `Plugin.landingPage` 导入模板
- 通过原生 `Plugin.invoiceTemplate` / `Plugin.authBranding` / `Plugin.pageRulePack` 导入宿主管理制品
- 记录模板导入前快照并支持回滚
- 查看插件包安装任务与历史
- 在宿主允许 `frontend.html_trusted` 时启用 trusted workspace，自绘目录浏览和弹窗预览；未开启时自动回退到 sanitize 块渲染

## 缓冲区与工作台

`js_market` 已适配当前 `Plugin.workspace` / admin workspace 新写法：

- 关键市场动作会把摘要镜像到插件 workspace 缓冲区，包括目录查询、预览、安装/导入、任务查询、历史和回滚
- 市场插件自身暴露了一组 workspace 命令，可直接在管理端工作台里执行

常用命令示例：

```bash
market/context
market/source official
market/catalog plugin_package debugger
market/release js-worker-template 0.2.2
market/preview js-worker-template 0.2.2
market/install js-worker-template 0.2.2 activate=true auto_start=false
market/tasks
market/tasks task_123
market/history js-worker-template
market/rollback js-worker-template 0.2.1
```

补充说明：

- 除位置参数外，也支持 `key=value` 形式，例如 `kind=landing_page_template source=official landing=home`
- 模板类制品可附带 `email_key=` / `landing_slug=` 等上下文参数
- 插件包安装可附带 `granted=`、`note=`、`activate=`、`auto_start=` 等参数

## 开发

```bash
npm install
npm run sync:manifest
npm run validate:manifest
npm run typecheck
npm run build
npm run package
```

打包后产物为：

- `dist/index.js`
- `js-market.zip`

说明：

- `npm run validate:manifest` 会复用 `plugins/sdk` 的统一校验脚本，检查 manifest 目录字段、schema 结构和宿主兼容性
- `npm run sync:manifest` 会把市场示例的兼容性字段、Hook、slot 和权限目录同步回官方市场基线
- `npm run package` 现在会先跑 manifest 校验与 `i18n:check`，再继续构建打包
