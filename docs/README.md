# AuraLogic 文档索引

本目录只保留主分支当前仍在维护的宿主文档。

## 主文档

- [API 总览](./API.md)
  - 人工维护的接口概览，适合快速浏览。
  - 具体字段、权限与最新行为请以 `backend/internal/router`、handler/service、前端调用代码和测试为准。
- [Payment JS API](./PAYMENT_JS_API.md)
  - `payment_js` 付款方式脚本运行时 API、回调约定与集成说明。
- [虚拟库存 JS 发货](./VIRTUAL_INVENTORY_JS_DELIVERY.md)
  - `type=script` 虚拟库存发货链路、兼容逻辑与代码定位。

## 相关 README

- [仓库总览](../README.md)
- [部署指南](../DEPLOYMENT.md)
- [后端文档](../backend/README.md)
- [前端文档](../frontend/README.md)
- [插件 SDK 文档](../plugins/sdk/README.md)
- [JS Market 插件文档](../plugins/js_market/README.md)

## 派生分支

以下内容已不再放在主分支维护：

- `feat/market-registry`
  - 市场注册表相关实现与文档。
- `feat/official-packages`
  - 官方模板、支付包、插件示例等次要内容。

## 维护约定

- 主分支文档只描述主分支当前实际存在的目录、脚本和能力。
- 若某项内容已经迁移到派生分支，不在主分支保留重复说明与失效链接。
- 若某份文档只是实现笔记、迁移草稿或阶段性设计稿，应优先合并到正式文档后删除。
