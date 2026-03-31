# AuraLogic Plugin SDK

本目录提供 `js_worker` 插件作者使用的本地 TypeScript SDK。

## SDK 覆盖范围

- 运行时类型：`Plugin.storage`、`Plugin.http`、`Plugin.fs`、`Plugin.host`、`Plugin.order`、`Plugin.user`、`Plugin.market`、`Plugin.emailTemplate`、`Plugin.landingPage`、`sandbox`
- 执行契约：`PluginExecuteResult`、`PluginHealthResult`、`PluginExecutionContext`
- 流式执行契约：`PluginExecuteStreamWriter`、`PluginDefinition.executeStream`
- 基础 helper：`definePlugin`、`successResult`、`errorResult`
- 安全解析：`asString`、`asBool`、`asInteger`、`asRecord`、`safeParseJSON`、`safeParseStringMap`
- 运行时访问：`getPluginRuntime`、`getPluginStorage`、`getPluginHTTP`、`getPluginFS`、`getPluginHost`、`getPluginOrder`、`getPluginUser`、`getPluginMarket`、`getPluginEmailTemplate`、`getPluginLandingPage`、`getPluginInvoiceTemplate`、`getPluginAuthBranding`、`getPluginPageRulePack`
- Hook / 目录 / FS helper：`normalizeHookName`、`normalizePluginPermissionKey`、`normalizePluginFSUsage`、`resolvePluginPageContext`
- 官方目录导出：
  - `OFFICIAL_PLUGIN_HOOKS`
  - `OFFICIAL_PLUGIN_HOOK_GROUPS`
  - `OFFICIAL_FRONTEND_SLOTS`
  - `OFFICIAL_PLUGIN_PERMISSION_KEYS`
  - `OFFICIAL_HOST_PERMISSION_KEYS`
  - `listOfficialPluginHooks`
  - `listOfficialFrontendSlots`
  - `listOfficialPluginPermissionKeys`
  - `listOfficialHostPermissionKeys`
  - `isOfficialPluginHook`
  - `isOfficialFrontendSlot`
  - `isOfficialPluginPermissionKey`
  - `isOfficialHostPermissionKey`
  - `inspectPluginManifestCompatibility`
  - `validatePluginManifestCatalog`
  - `validatePluginManifestSchema`
- 插件页 builder：
  - `buildPluginTextBlock`
  - `buildPluginAlertBlock`
  - `buildPluginKeyValueBlock`
  - `buildPluginJSONViewBlock`
  - `buildPluginTableBlock`
  - `buildPluginBadgeListBlock`
  - `buildPluginLinkListBlock`
  - `buildPluginStatsGridBlock`
  - `buildPluginActionFormBlock`
  - `buildPluginHTMLBlock`
- 前端扩展 builder：
  - `buildPluginMenuItemExtension`
  - `buildPluginRoutePageExtension`
  - `buildPluginPageBootstrap`
- 插件页自执行 helper：
  - `buildPluginExecuteHTMLBridge`
  - `buildPluginExecuteTemplatePlaceholder`
  - `buildPluginExecuteTemplateValues`
  - `renderPluginTemplate`
  - `PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS`

## 常见用法

### 1. 定义插件入口

```ts
import {
  asRecord,
  asString,
  definePlugin,
  successResult,
  type PluginExecutionContext,
  type PluginSandboxProfile,
  type UnknownMap
} from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute(
    action: unknown,
    params: unknown,
    context: PluginExecutionContext,
    config: UnknownMap,
    sandbox: PluginSandboxProfile
  ) {
    return successResult({
      message: "ok",
      values: {
        action: asString(action),
        params: asRecord(params),
        context,
        sandbox,
        config
      }
    });
  }
});
```

### 2. 安全读取运行时对象

```ts
import { getPluginFS, getPluginStorage, normalizePluginFSUsage } from "@auralogic/plugin-sdk";

const storage = getPluginStorage();
const fs = getPluginFS();

const rawUsage = fs?.usage();
const usage = normalizePluginFSUsage(rawUsage, fs);
```

说明：

- `Plugin.fs` 路径必须使用相对路径，例如 `notes/demo.txt`
- 不要传绝对路径 `/notes/demo.txt`
- `Plugin.fs.usage()` / `Plugin.fs.recalculateUsage()` 统计的是数据层占用，不含代码层文件
- `normalizePluginFSUsage(...)` 会兼容历史运行时的 `FileCount` / `MaxFiles` 形态

### 3. 读取插件页参数

```ts
import { resolvePluginPageContext, type PluginExecutionContext } from "@auralogic/plugin-sdk";

function readPageContext(context: PluginExecutionContext) {
  const page = resolvePluginPageContext(context);
  return {
    fullPath: page.full_path,
    orderID: page.query_params.order_id || "",
    orderNo: page.route_params.orderNo || ""
  };
}
```

说明：

- `resolvePluginPageContext(...)` 会把 host 注入到 `context.metadata` 里的
  - `plugin_page_full_path`
  - `plugin_page_query_string`
  - `plugin_page_query_params`
  - `plugin_page_route_params`
  还原成结构化对象
- `safeParseStringMap(...)` 也可以单独用于解析插件配置或 metadata 里的 JSON string map

### 3.1 官方 Hook / Slot / 权限目录

```ts
import {
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_PERMISSION_KEYS,
  OFFICIAL_HOST_PERMISSION_KEYS,
  OFFICIAL_PLUGIN_HOOKS,
  listOfficialPluginHooks,
  isOfficialFrontendSlot,
  isOfficialPluginPermissionKey,
  isOfficialHostPermissionKey,
  isOfficialPluginHook
} from "@auralogic/plugin-sdk";

const allHooks = OFFICIAL_PLUGIN_HOOKS;
const commerceHooks = listOfficialPluginHooks("commerce");
const canHandle = isOfficialPluginHook("order.create.before");
const validSlot = isOfficialFrontendSlot("admin.orders.top");
const validPluginPermission = isOfficialPluginPermissionKey("frontend.extensions");
const validPermission = isOfficialHostPermissionKey("host.market.catalog.read");
```

说明：

- 这些常量直接从当前仓库的宿主注册表与前端插槽扫描生成，适合做：
  - `manifest.capabilities.hooks` 校验
  - `allowed_frontend_slots` 校验
  - `requested_permissions` / `granted_permissions` / `permissions[].key` 校验
- `OFFICIAL_PLUGIN_PERMISSION_KEYS` 覆盖全部官方插件权限键，包括基础能力权限（例如 `hook.execute` / `frontend.extensions` / `runtime.file_system`）以及全部 `host.*` 权限
- 当前官方目录已经覆盖完整 Hook 扩展面，官方 debugger 示例 manifest 也会通过测试持续校验这些导出与宿主保持同步

### 3.2 直接校验插件 manifest 目录字段

```ts
import {
  inspectPluginManifestCompatibility,
  validatePluginManifestCatalog,
  validatePluginManifestSchema
} from "@auralogic/plugin-sdk";

const catalogValidation = validatePluginManifestCatalog(manifest);
const schemaValidation = validatePluginManifestSchema(manifest);
const compatibility = inspectPluginManifestCompatibility(manifest);

if (!catalogValidation.valid) {
  console.log(catalogValidation.invalid_hooks);
  console.log(catalogValidation.invalid_allowed_frontend_slots);
  console.log(catalogValidation.invalid_requested_permissions);
}

if (!schemaValidation.valid) {
  console.log(schemaValidation.issues);
}

if (!compatibility.compatible) {
  console.log(compatibility.reason_code, compatibility.reason);
}
```

`validatePluginManifestCatalog(...)` 返回内容除了 `valid` 之外，还会给出：

- `invalid_hooks`
- `invalid_disabled_hooks`
- `invalid_allowed_frontend_slots`
- `invalid_requested_permissions`
- `invalid_granted_permissions`
- `invalid_declared_permissions`
- `requested_permissions_missing_declaration`
- `granted_permissions_missing_declaration`
- `declared_permissions_missing_request`

适合在插件自己的构建、打包、测试阶段直接拦截配置漂移。

`validatePluginManifestSchema(...)` 会继续补上结构校验，包括：

- `config_schema / secret_schema / runtime_params_schema` 字段结构
- `frontend.admin_page / frontend.user_page` 路径前缀
- `webhooks` method / auth_mode / secret_key
- `permissions[]` / `capabilities.execute_action_storage` / frontend capability 字段形态
- `manifest_version / protocol_version / min_host_protocol_version / max_host_protocol_version`

`inspectPluginManifestCompatibility(...)` 则会单独给出与宿主一致的兼容性判定结果，适合在发布前展示更明确的原因码。

如果你想直接在插件目录里复用 SDK 自带的校验脚本，也可以在插件 `package.json` 中加入：

```json
{
  "scripts": {
    "validate:manifest": "node ../sdk/scripts/validate-plugin-manifest.mjs ."
  }
}
```

这样在 `npm run package` 前先执行一次，就能把 catalog / schema / compatibility 三层检查统一串起来。

如果你维护的是官方示例插件，也可以继续复用 SDK 的同步脚本，避免再手工复制 hook / slot / permission 目录：

```json
{
  "scripts": {
    "sync:manifest": "node ../sdk/scripts/sync-plugin-manifest.mjs . --profile=debugger"
  }
}
```

当前 `--profile=debugger` 会把 debugger 官方示例 manifest 的：

- `capabilities.hooks`
- `capabilities.allowed_frontend_slots`
- `capabilities.requested_permissions`
- `capabilities.granted_permissions`
- `permissions[]`

同步到宿主最新官方目录，适合在 Hook / slot / 权限扩面后先跑一遍再打包。

另外两个官方示例也已经接入 profile：

- `--profile=template`：同步 template 官方示例 manifest 的兼容性字段、模板示例权限、Hook 与插件页 slot
- `--profile=market`：同步 `plugins/js_market/manifest.json` 的兼容性字段、市场示例权限、Hook 与插件页 slot

如果你只想检查官方示例是否已经漂移，而不实际改文件，可以直接运行：

```bash
npm run check:sample-manifests
```

它会串行检查当前分支可用的官方示例 manifest：

- 主分支通常包含 `plugins/js_market`
- `feat/official-packages` 分支还会包含 debugger / template 官方示例

### 4. 访问宿主原生数据 API

```ts
import {
  getPluginHost,
  getPluginOrder,
  getPluginUser
} from "@auralogic/plugin-sdk";

const host = getPluginHost();
const orders = getPluginOrder();
const users = getPluginUser();

const order = orders?.get({ order_no: "ORD-1001" });
const orderList = orders?.list({ status: "paid", page_size: 10 });
const user = users?.get({ email: "demo@example.com" });
const raw = host?.invoke("host.order.list", { page: 1, page_size: 20 });
```

说明：

- `getPluginOrder()` 会优先返回 `Plugin.order`，若运行时只暴露 `Plugin.host.order` 也会自动兼容
- `getPluginUser()` 同理，兼容 `Plugin.user` 与 `Plugin.host.user`
- `Plugin.order.get(...)` / `Plugin.user.get(...)` 推荐传对象参数；按当前契约，数字快捷值表示 `id`
- 实际可用性仍由宿主注入与权限控制决定，未启用时 getter 会返回 `undefined`

### 4.1 访问市场与模板桥

```ts
import {
  getPluginEmailTemplate,
  getPluginLandingPage,
  getPluginMarket
} from "@auralogic/plugin-sdk";

const market = getPluginMarket();
const emailTemplate = getPluginEmailTemplate();
const landingPage = getPluginLandingPage();

const sources = market?.source?.list();
const source = market?.source?.get({ source_id: "official" });
const catalog = market?.catalog?.list({ source_id: "official", kind: "plugin_package" });
const artifact = market?.artifact?.get({
  source_id: "official",
  kind: "plugin_package",
  name: "js-worker-debugger"
});
const release = market?.release?.get({
  source_id: "official",
  kind: "plugin_package",
  name: "js-worker-debugger",
  version: "1.0.0"
});
const preview = market?.install?.preview({
  source_id: "official",
  kind: "plugin_package",
  name: "js-worker-debugger",
  version: "1.0.0"
});
const installTask = market?.install?.execute({
  source_id: "official",
  kind: "plugin_package",
  name: "js-worker-debugger",
  version: "1.0.0",
  options: {
    activate: true
  }
});
const template = emailTemplate?.get({ key: "order_paid" });
const page = landingPage?.get({ page_key: "home" });
```

说明：

- `Plugin.market.*` 已覆盖 source / catalog / artifact / release 查询，以及 install preview / execute / task / history / rollback
- `Plugin.emailTemplate.*` / `Plugin.landingPage.*` / `Plugin.invoiceTemplate.*` / `Plugin.authBranding.*` / `Plugin.pageRulePack.*` 适合做模板 revision 编排和受控原生保存
- 这些桥接能力仍会同时受插件权限和当前操作员权限控制

### 5. 生成插件页

```ts
import {
  buildPluginActionFormBlock,
  buildPluginAlertBlock,
  buildPluginJSONViewBlock,
  buildPluginKeyValueBlock,
  buildPluginPageBootstrap
} from "@auralogic/plugin-sdk";

const page = {
  title: "Demo",
  blocks: [
    buildPluginAlertBlock({
      title: "Ready",
      content: "Plugin page loaded.",
      variant: "success"
    }),
    buildPluginKeyValueBlock({
      title: "Runtime",
      items: [
        { label: "Storage", value: Boolean(storage) },
        { label: "FS", value: Boolean(fs?.enabled) }
      ]
    }),
    buildPluginJSONViewBlock({
      title: "Usage",
      value: usage
    }),
    buildPluginActionFormBlock({
      title: "Config",
      load: "plugin.config.get",
      loadLabel: "Load",
      save: "plugin.config.save",
      saveLabel: "Save",
      fields: [{ key: "enabled", type: "boolean", label: "Enabled" }]
    })
  ]
};

const frontendExtensions = buildPluginPageBootstrap({
  area: "admin",
  path: "/admin/plugin-pages/demo",
  title: "Demo",
  page
});
```

### 6. 在 HTML 块里调用插件自己的 execute API

```ts
import {
  buildPluginExecuteHTMLBridge,
  buildPluginExecuteTemplatePlaceholder,
  buildPluginHTMLBlock
} from "@auralogic/plugin-sdk";

const html = buildPluginExecuteHTMLBridge({
  action: "plugin.echo",
  submitLabel: "Run"
});

const block = buildPluginHTMLBlock(html, "HTML Execute Demo");

const customHTML = `
  <div>
    <div>Current page: ${buildPluginExecuteTemplatePlaceholder("plugin.full_path")}</div>
    <div>Order ID: ${buildPluginExecuteTemplatePlaceholder("plugin.query.order_id")}</div>
    <div>Order No: ${buildPluginExecuteTemplatePlaceholder("plugin.route.orderNo")}</div>
  </div>
`;
```

说明：

- 内置模板占位符现在除了 `plugin.path` 外，还支持：
  - `plugin.full_path`
  - `plugin.query_string`
  - `plugin.query_params_json`
  - `plugin.route_params_json`
  - 动态键 `plugin.query.<key>`
  - 动态键 `plugin.route.<key>`

### 7. 实现真实流式输出

```ts
import {
  definePlugin,
  successResult,
  type PluginExecuteStreamWriter
} from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute(action) {
    return successResult({ values: { action } });
  },
  executeStream(action, params, context, _config, _sandbox, stream: PluginExecuteStreamWriter) {
    stream.progress("preparing", 20, { phase: "prepare" });
    stream.write({
      status: "running",
      progress: 70,
      action,
      session_id: context.session_id || ""
    }, { phase: "mid" });

    return successResult({
      values: { action, params, session_id: context.session_id || "" },
      message: "stream completed"
    });
  }
});
```

说明：

- `js_worker` 现在支持真实 `execute_stream` 多 chunk 输出，不再只回落成单个最终块。
- 运行时会优先调用导出的 `executeStream(...)`；若未导出，则仍会回退到普通 `execute(...)` 并返回最终块。
- `stream.write(...)` / `stream.emit(...)` 发送中间块，`stream.progress(...)` 用于常见的 `status + progress` 更新。

## 打包要求

- 依赖声明：`"@auralogic/plugin-sdk": "file:../sdk"`
- 推荐在构建阶段把依赖 bundle 到单个 `index.js`
- 如果不做 bundle，就需要把 SDK 的 `dist/` 与 `package.json` 一起打进插件 zip
- 官方 template / debugger 示例已演示完整打包方式；主分支不再内置这两个示例目录

## 构建

1. `make deps`
2. `npm run generate:catalog`
3. `make typecheck`
4. `npm test`
5. `make build`

补充说明：

- `npm run generate:catalog` 会从宿主 Hook 注册表、权限注册表和前端 slot 使用点重新生成 `src/generated-catalog.ts`
- `npm run ensure:dist` 会在 `dist/` 缺失或过期时自动重建 SDK，供示例插件的 `build/test/typecheck/dev` 复用
- `npm run check:sample-manifests` 会校验当前分支可用的官方示例 manifest 是否仍与 SDK profile 保持一致
- 当宿主新增 Hook / `host.*` 权限 / 前端 slot 后，先跑这一步，再跑测试和打包，会更不容易出现 catalog 漂移

## 参考

- 主分支插件文档入口：本文件与 `plugins/js_market/README.md`
- 官方 template / debugger 示例：`feat/official-packages` 派生分支
