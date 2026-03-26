# Plugin Debugger JS Plugin

统一的 `js_worker` 调试器示例，当前版本 `2.4.0`。

它现在不再只是演示几个老 Hook，而是直接对齐宿主当前的完整扩展面：

- 覆盖全部官方 Hook / 前端插槽 / 插件权限 / `host.*` 权限目录
- 支持按域切换 Hook 组：`frontend / auth / platform / commerce / catalog / support / content / settings`
- 支持完整宿主原生桥联调：
  - `Plugin.order / user / product / inventory / inventoryBinding`
  - `Plugin.promo / ticket / serial / announcement / knowledge / paymentMethod`
  - `Plugin.virtualInventory / virtualInventoryBinding`
  - `Plugin.market`
  - `Plugin.emailTemplate / landingPage / invoiceTemplate / authBranding / pageRulePack`
- 继续覆盖 `frontend.bootstrap`、`frontend.slot.render`、`Plugin.storage`、`Plugin.fs`、`Plugin.http`
- 管理端 workspace 继续覆盖：
  - `debugger/catalog`
  - `debugger/prompt`
  - `debugger/context`
  - 可直接验证 shell 变量展开，例如 `debugger/context $PLUGIN_NAME $WORKSPACE_STATUS $TASK_ID`
- 保留 payload patch / hook block / execute / execute_stream / HTML bridge / sandbox 可视化
- 测试会自动校验 debugger 的 hook/slot/permission 目录与宿主源码保持同步，避免再出现“计划文档和实现漂移”

## 使用

1. 进入 `plugins/js-worker-debugger`
2. 执行 `npm install`
3. 执行 `npm test`
4. 宿主扩面后先执行 `npm run sync:manifest`
5. 可选执行 `npm run validate:manifest`
6. 执行 `npm run package`
7. 上传 `js-worker-debugger.zip`
8. 启动后打开 `/admin/plugin-pages/debugger`

## 页面内容

- Runtime Snapshot：当前 Hook、sandbox、storage、filesystem、network、host bridge 概览
- Recent Hook Events：最近真实 Hook 事件
- Debugger Controls：按域开关 Hook 组、事件持久化、payload marker、blocking demo
- Hook Simulator：模拟任意官方 Hook
- Host Data Lab：联调全部 typed host helper 与 raw `host.invoke`
- Storage Lab / Filesystem Lab / Network Lab：验证 `Plugin.storage`、`Plugin.fs`、`Plugin.http`
- Visual Exec Demo / HTML Stream Bridge：验证 execute / execute_stream / 页面 bridge
- Plugin Workspace：验证 workspace 缓冲区、实时输入，以及 shell 变量展开后的 argv

## 现在最适合用它做什么

- 检查某个业务流程到底触发了哪个 Hook
- 验证某个 slot 是否真的存在、是否能注入
- 验证插件权限授予后，`Plugin.market` / 模板桥 / 原生实体桥是否可用
- 看宿主对 payload patch 白名单和 block 权限是否按预期生效
- 作为新插件开发时的“活样板”直接复制结构

## 与 SDK 的关系

`plugins/sdk` 现在已经导出：

- `OFFICIAL_PLUGIN_HOOKS`
- `OFFICIAL_PLUGIN_HOOK_GROUPS`
- `OFFICIAL_FRONTEND_SLOTS`
- `OFFICIAL_PLUGIN_PERMISSION_KEYS`
- `OFFICIAL_HOST_PERMISSION_KEYS`
- `listOfficialPluginHooks()` / `listOfficialFrontendSlots()` / `listOfficialPluginPermissionKeys()` / `listOfficialHostPermissionKeys()`
- `isOfficialPluginHook()` / `isOfficialFrontendSlot()` / `isOfficialPluginPermissionKey()` / `isOfficialHostPermissionKey()`
- `inspectPluginManifestCompatibility()` / `validatePluginManifestCatalog()` / `validatePluginManifestSchema()`

Debugger 现在直接复用 SDK 导出的官方目录，不再维护一份独立的 hook/slot 副本，所以：

- 要找“当前官方 Hook/slot/permission 列表”，优先看 SDK
- 要做运行时联调，优先看 Debugger
- 不再需要依赖历史的 Hook 扩展计划文档

## 说明

- `frontend.bootstrap` 始终保留，避免把 debugger 页面自己关掉
- 若未授予 `frontend.extensions`、`api.execute`、`runtime.file_system`、`runtime.network`、`host.*`，页面会直接显示能力缺口
- Host Data Lab 已经覆盖市场与模板原生桥，不需要再手写 raw action 才能验证这些能力
- 该示例现在的测试会直接对比：
  - `backend/internal/service/plugin_hook_registry.go`
  - `backend/internal/service/plugin_permission_registry.go`
  - `frontend/**/*.tsx` 中的 `PluginSlot`
- `npm run package` 现在会先走一轮 SDK manifest 校验，再继续打包 zip，避免把结构错误的 manifest 打进产物
- 当前 `validate:manifest` 直接复用 `plugins/sdk/scripts/validate-plugin-manifest.mjs`，模板插件和市场插件也可共用同一套校验入口
- 当前 `sync:manifest` 直接复用 `plugins/sdk/scripts/sync-plugin-manifest.mjs --profile=debugger`，可以一键把 debugger manifest 对齐到宿主最新 Hook / slot / permission 目录
- `plugins/sdk` 还提供了 `npm run check:sample-manifests`，会串行检查 debugger / template / market 三个官方示例的 manifest 是否仍与 SDK profile 对齐
