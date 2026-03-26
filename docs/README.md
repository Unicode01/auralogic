# AuraLogic 文档索引

这份索引用来说明当前仓库里哪些文档是主文档，哪些只是人工维护的参考摘要，以及哪些历史设计稿已经收敛。

## 当前主文档

- [插件开发文档](../plugins/README.md)
  - `js_worker` 插件开发、SDK、workspace、runtime console、`Plugin.fs`、`Plugin.storage`、`Worker`、Hook 与前端扩展的主文档。
- [Payment JS API](./PAYMENT_JS_API.md)
  - `payment_js` 运行时 API、回调函数、HTTP、storage、webhook URL 与模板包示例。
- [虚拟库存 JS 发货](./VIRTUAL_INVENTORY_JS_DELIVERY.md)
  - `type=script` 虚拟库存发货链路、兼容逻辑和代码定位。
- [市场注册表文档](../market_registry/README.md)
  - market_registry 的唯一文档入口，包含运行方式、发布流程、管理后台和兼容说明。

## 人工维护的参考文档

- [API 总览](./API.md)
  - 这是人工维护的接口总览，适合快速浏览，不保证覆盖最新所有字段与路由。
  - 实际集成时请以 `backend/internal/router`、handler/service、前端调用代码和测试为准。

## 已收敛的历史文档

- `docs/PLUGIN_JS_WORKSPACE_DESIGN.md`
  - 这份文档原本用于推动 `js_worker` workspace / runtime console / attach 设计。
  - 相关能力已经进入正式实现与开发文档，主入口改为 [插件开发文档](../plugins/README.md)。
  - 仓库中不再保留这类“已基本落地但仍以 Draft/设计稿形态存在”的重复文档。
- `market_registry` 历史拆分文档
  - 市场相关文档已经整体收敛到 [market_registry/README.md](../market_registry/README.md)。
  - 仓库中不再保留额外的市场协议拆分文档，避免重复维护。

## 维护约定

- 面向插件作者的能力说明，优先写进 `plugins/README.md`。
- 面向市场的说明统一保留在 `market_registry/README.md`。
- 若某份文档仍然写着“计划”“草稿”“后续将支持”，但能力已经稳定落地，应优先合并进正式文档并删除历史稿。
