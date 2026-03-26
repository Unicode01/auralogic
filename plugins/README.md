# AuraLogic 插件开发指南

这份文档面向“插件作者”，重点是：怎么写、怎么打包、怎么调试、怎么在系统里跑起来。

## 0. 开发入口（`plugins/`）

建议从以下目录开始：

- `plugins/sdk`：本地 TypeScript SDK（类型与工具函数）
- `plugins/js-worker-template`：`js_worker` 最小可用模板（已包含专属插件页、自执行表单、HTML bridge、workspace 命令、原生 order/user 查询示例）
- `plugins/js-worker-debugger`：完整示例（统一调试器，覆盖 workspace / hook / slot / host.* / market / 模板桥）

推荐顺序：

1. 先用 `plugins/js-worker-template` 启动新插件；
2. 需要更多能力时参考 `plugins/js-worker-debugger` 的实现结构；
3. 公共类型和 helper 统一复用 `plugins/sdk`，包括前端页面、执行结果、sandbox 类型。

## 1. 运行时模型

AuraLogic 当前支持两种插件运行时：

1. `grpc`

- 你的插件是独立服务进程，平台通过 gRPC 调用。

2. `js_worker`

- 你的插件是 JS 脚本包，平台把脚本分发给 goja Worker 执行。
- 主进程与 Worker 通过 Unix Socket（或 tcp）通信。

选择建议：

- 业务逻辑轻量、迭代快：优先 `js_worker`
- 需要复杂依赖或高性能：优先 `grpc`

---

## 2. 插件数据契约

每个插件核心字段（管理端配置）：

- `name`：插件唯一标识
- `type`：业务类型（受系统白名单约束）
- `runtime`：`grpc` / `js_worker`
- `address`：gRPC 地址或 JS 入口脚本路径
- `config`：插件配置（JSON Object）
- `runtime_params`：默认参数（JSON Object，执行时与请求参数合并）
- `capabilities`：插件能力与安全策略（JSON Object，例如权限授予、前端 HTML 渲染模式、前端 Hook 预筛选）

参数合并规则（重要）：

1. 先读 `runtime_params`
2. 再叠加调用参数 `params`
3. 同名参数以后者为准

### 2.1 `js_worker` 持久化存储（`Plugin.storage`）

`js_worker` 运行时提供插件级 KV 存储：

- `Plugin.storage.get(key)`：读取字符串值，不存在返回 `undefined`
- `Plugin.storage.set(key, value)`：写入字符串值，返回 `true/false`
- `Plugin.storage.delete(key)`：删除键，返回 `true/false`
- `Plugin.storage.list()`：返回当前插件全部 key（数组）
- `Plugin.storage.clear()`：清空当前插件全部存储，返回 `true/false`

运行时配额（可由管理员在系统设置中调整）：

- `max_keys`：默认 `512`
- `max_total_bytes`：默认 `4MB`（按 `key + value` 字节累计）
- `max_value_bytes`：默认 `64KB`（单个 value）

超限行为：

- `Plugin.storage.set(key, value)` 在任一配额超限时返回 `false`，不会写入。
- 持久化入库前平台会再次校验配额，作为兜底保护。

存储隔离与生命周期：

- 按 `plugin_id` 隔离，不同插件互不可见。
- 在插件 `start/pause/restart` 与版本切换间会保留。
- 当管理员删除插件时，平台会同步清理该插件全部存储数据。

可选性能声明：
- 可以在 `capabilities.execute_action_storage` 中按 action 声明存储访问模式，宿主会据此做更细粒度的并发控制。
- 支持的模式只有 `none`、`read`、`write`。
- 未声明时宿主会按保守模式串行执行 `js_worker` action。
- 如果声明为 `none/read`，但运行时实际发生了更强的存储访问，宿主会拒绝这次执行并返回错误，避免并发下写坏 `Plugin.storage`。

示例：
```json
{
  "capabilities": {
    "execute_action_storage": {
      "template.echo": "none",
      "template.page.get": "read",
      "template.page.save": "write"
    }
  }
}
```

运行时可观测字段：

- 宿主会把 `current_action`、`declared_storage_access_mode`、`execute_action_storage` 注入到 `sandbox`。
- `js_worker` 运行时还会在执行过程中实时更新 `sandbox.storageAccessMode`，插件可直接判断当前 action 已经发生了 `none/read/write` 中的哪一级访问。
- 建议在调试页或开发期日志里同时展示“声明值 + 实际值”，方便定位串行退化、误声明或无声明的 action。

### 2.2 `js_worker` Workspace（`Plugin.workspace`）

`js_worker` 运行时现已提供插件专属 workspace 缓冲区，适合做：

- 插件自己的调试输出
- 普通 action / hook 的补充日志
- 管理端 workspace 面板中的历史回放

当前可用 API：

- `Plugin.workspace.write(message, metadata?)`
- `Plugin.workspace.writeln(message?, metadata?)`
- `Plugin.workspace.info(message, metadata?)`
- `Plugin.workspace.warn(message, metadata?)`
- `Plugin.workspace.error(message, metadata?)`
- `Plugin.workspace.clear()`：清空当前插件 workspace buffer
- `Plugin.workspace.tail(limit?)`：读取最近若干条 entry
- `Plugin.workspace.snapshot(limit?)`：读取当前 workspace 快照

使用建议：

- 普通 action / hook 中优先用 `info/warn/error`
- 结构化上下文建议放入 `metadata`，例如 `action/hook/task_id/source`
- 不要把 workspace 当作长期业务存储；需要持久数据时仍然用 `Plugin.storage` / `Plugin.fs`
- `console.log/info/warn/error` 当前也会镜像进 workspace，所以可以把它理解为“插件专属控制台 + 缓冲区”

SDK 推荐通过 getter 访问：

- `getPluginWorkspace()`

最小示例：

```ts
import {
  definePlugin,
  getPluginWorkspace,
  successResult
} from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute(action: unknown) {
    const workspace = getPluginWorkspace();
    if (workspace?.enabled) {
      workspace.info(`running action: ${String(action || "unknown")}`, {
        source: "sample",
        action: String(action || "")
      });
    }
    return successResult({ message: "ok" });
  }
});
```

### 2.3 `js_worker` Workspace Commands（代码定义式命令目录）

现在推荐把 workspace 命令目录直接定义在 SDK 的 `definePlugin({ workspace: { ... } })` 里。

- SDK 会自动从 `workspace` 定义导出命令目录到 health metadata 的 `workspace_commands_json`
- 宿主只读取运行时目录和 host builtin commands
- `manifest.workspace` 不再参与 workspace 命令发现，新插件可以完全不写这段

SDK 示例：

```ts
import {
  definePlugin,
  defineWorkspaceCommand,
  defineWorkspaceCommands,
  successResult,
  type PluginWorkspaceCommandContext
} from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute(action) {
    return successResult({ message: String(action || "ok") });
  },
  workspace: defineWorkspaceCommands({
    "debugger.prompt": defineWorkspaceCommand(
      {
        name: "debugger/prompt",
        title: "Prompt Demo",
        description: "Read one line from the workspace input buffer.",
        interactive: true
      },
      (
        command: PluginWorkspaceCommandContext,
        _context,
        _config,
        _sandbox,
        workspace
      ) => {
        const input = workspace.readLine("debugger> ", { echo: true });
        workspace.info(`received: ${input}`, { command: command.name });
        return successResult({
          message: `workspace input = ${input}`
        });
      }
    )
  })
});
```

说明：

- `workspace: { "entry.name": handlerFn }` 的旧写法仍然可用
- 现在也推荐用：
  - `defineWorkspaceCommand(handler)`
  - `defineWorkspaceCommand(options, handler)`
  - `defineWorkspaceCommands({ ... })`
- SDK 会自动推导：
  - `name = entry.replace(/\./g, "/")`
  - `interactive = false`
  - `permissions = []`
- 若你需要精确控制标题、描述、interactive 或权限声明，请改用对象写法
- JS runtime console 的 Tab 补全现在会合并：
  - 宿主内置 helper（`help`, `keys`, `runtimeState`, `Plugin.*` 等）
  - 当前运行中 VM 的 live completion paths
  - 因此顶层 `function debug() {}`、`var report = ...`、`module.exports.execute`、`module.exports.workspace.debug` 这类当前 VM 可访问的函数/路径都会自动进入补全
- `const` / `let` 顶层绑定如果没有挂到 `globalThis` / `module.exports`，一般不会出现在补全目录里；如果你希望稳定可补全，建议显式导出或挂到 `globalThis`

当前阶段说明：

- 命令入口已经支持宿主声明式发现与管理端直接执行
- `Plugin.workspace.read()` / `readLine()` 现在会先消费“预置输入行”，不足时再切到管理端实时输入
- interactive workspace command 已支持异步启动、实时 attach、实时输入、中断与终止基础信号
- 管理端已支持 owner / viewer 仲裁、接管控制权与只读旁观
- 管理端现在会优先走 websocket 双工通道；如果宿主或链路不支持，再自动回退到 NDJSON stream
- 宿主 builtin commands 已覆盖 `help`、`clear`、`log.tail`、`pwd`、`ls`、`stat`、`cat`、`mkdir`、`find`、`grep`、`kv.get`、`kv.set`、`kv.list`、`kv.del`

### 2.3.1 管理端 JS 控制台 / Live VM

管理端插件工作台现在连接的是插件的实时 VM，而不是“每输入一行就新建一个临时 VM”。

这意味着：

- 运行时表达式、顶层函数、`module.exports`、运行中生成的状态都能在同一个 VM 内连续观察
- `console.log/info/warn/error/debug` 会继续进入宿主日志，同时镜像到插件 workspace 缓冲区
- 工作台输入框支持最近命令回填、Tab 自动补全、`:inspect` 对象检查和 JSON 可视化输出

当前推荐优先使用这些 helper：

- `help()`：查看 helper、全局能力和主题帮助
- `commands()`：查看当前可用 plugin exports、workspace alias 与 helper 目录
- `permissions()`：查看当前 action 的请求/授予权限和 sandbox 开关
- `workspaceState()`：查看当前 workspace 缓冲区和命令状态
- `inspect(value, depth?)`：在不污染输出的前提下生成结构化预览
- `clearOutput()`：清空当前 workspace 缓冲区
- `$_`、`$1`...`$5`：引用最近成功表达式结果

当前运行时默认可用的浏览器风格全局包括：

- `Worker`
- `structuredClone`
- `queueMicrotask`
- `setTimeout` / `clearTimeout`
- `TextEncoder` / `TextDecoder`
- `atob` / `btoa`

建议：

- 需要稳定补全的函数，显式挂到 `module.exports` 或 `globalThis`
- 需要调试输出时优先使用 `console.*` 或 `Plugin.workspace.*`
- 需要等待管理员输入时，仅在 interactive workspace command 中调用 `read()` / `readLine()`

### 2.4 `js_worker` 插件文件系统（`Plugin.fs`）

`js_worker` 文件系统采用双层结构（overlay）：

- 代码层（只读）：当前激活版本的插件解压目录（`codeRoot`）
- 数据层（可读写）：插件持久目录（`plugin.artifact_dir/data/plugin_<plugin_id>`，`dataRoot`）
- 对插件暴露的逻辑根目录固定为 `/`（`Plugin.fs.root`）

读写规则：

- 读操作（`exists/read*/stat/list`）优先读取数据层，再回退代码层。
- 写操作（`write*/delete/mkdir`）只落到数据层，不会覆盖代码层。
- `list` 会合并双层同名条目（数据层优先）。
- `usage` 只统计数据层占用，并执行配额限制。

可用 API：

- `Plugin.fs.exists(path)`：判断文件是否存在
- `Plugin.fs.readText(path)`：读取文本
- `Plugin.fs.readBase64(path)`：读取文件并返回 base64（适合 icon / 图片）
- `Plugin.fs.readJSON(path)`：读取并解析 JSON
- `Plugin.fs.writeText(path, content)`：写文本
- `Plugin.fs.writeJSON(path, value)`：写 JSON
- `Plugin.fs.writeBase64(path, base64)`：写入 base64 内容
- `Plugin.fs.delete(path)`：删除文件/目录
- `Plugin.fs.mkdir(path)`：创建目录（递归）
- `Plugin.fs.list(path)`：列出目录
- `Plugin.fs.stat(path)`：查看文件信息
- `Plugin.fs.usage()`：查看当前目录文件数/总大小使用量
- `Plugin.fs.recalculateUsage()`：重扫数据层并刷新 usage 缓存

运行时限制：

- 路径必须是相对路径，禁止绝对路径与 `..` 越界。
- 解析时会做根目录约束与符号链接安全校验，不能跳出插件层根目录。
- 配额默认值：`max_files=2048`、`max_total_bytes=128MB`、`max_read_bytes=4MB`（可通过配置调整）。
- `usage()` / `recalculateUsage()` 返回结构统一为：
  - `file_count`
  - `total_bytes`
  - `max_files`
  - `max_bytes`

当文件系统权限关闭时（`allow_file_system=false`），调用 `Plugin.fs.*` 会直接抛错。

版本切换与清理：

- 激活新版本时，会把旧版本可迁移的运行时文件迁移到数据层（不迁移 `manifest`、入口代码等源码文件）。
- 删除插件时，会同步清理该插件的数据层目录。

### 2.5 `js_worker` 子 Worker（`new Worker()`）

当前 `js_worker` 已支持在主插件 VM 内创建“子 Worker”做并行计算或异步宿主调用。

当前形态是受限版 `Worker v1`：

- 每个 `Worker` 都是独立的 goja VM
- 子 Worker 仅在当前父执行生命周期内存活；父执行结束会自动回收
- 支持：
  - `new Worker("./child.js")`
  - `await worker.request(payload)`：请求/响应式调用，推荐优先使用
  - `worker.postMessage(payload)` + `worker.onmessage = fn`
  - `worker.onerror = fn`
  - `worker.terminate()`
- 子 Worker 脚本可通过：
  - `globalThis.onmessage = fn`
  - `onmessage = fn`
  - `module.exports.onmessage = fn`
  - `module.exports.handleMessage = fn`
  来声明入口
- 子 Worker 内可用：
  - `postMessage(payload)`：向父 VM 发送消息
  - `close()`：请求在当前消息处理结束后关闭自身
  - `self`

最小示例：

```js
// index.js
module.exports.execute = async function () {
  const worker = new Worker("./child.js");
  const result = await worker.request({ value: 21 });
  return {
    success: true,
    data: result
  };
};
```

```js
// child.js
onmessage = async function (event) {
  return {
    doubled: event.data.value * 2
  };
};
```

消息事件结构：

- 父传子：`event.data`
- 子传父：`event.data`
- 附加字段：
  - `event.type = "message"`
  - `event.worker_id`
  - `event.script_path`

当前边界与限制：

- 子 Worker 的 `Plugin.storage` 当前为只读/空快照语义：
  - `get/list` 不会读取父执行中的实时写入结果
  - `set/delete/clear` 会直接返回 `false`
- 子 Worker 默认不开放嵌套 `new Worker()`
- 子 Worker 不继承父执行的 `Plugin.workspace` 交互输入；如需日志，请优先通过返回值或 `postMessage()` 回传
- 传输数据当前走 JSON 可序列化结构；函数、循环引用、复杂类实例不支持
- 若你只是想做“单次并行任务”，优先用 `worker.request()`，不要先造一套自定义消息协议

什么时候值得用：

- 并行拉多个宿主资源后再聚合
- 大量 JSON/文本处理、模板预计算、规则匹配
- 需要把耗时逻辑从主执行流里拆开，并配合 `async/await` 组织

补充的运行时异步全局：

- `structuredClone(value)`：深拷贝 JSON 可序列化值
- `queueMicrotask(callback)`：排入当前持久运行时的微任务队列，并在当前表达式返回前尽量完成 flush
- `setTimeout(callback, delayMs, ...args)`：在当前持久运行时上调度定时器回调
- `clearTimeout(timerId)`：取消待执行定时器

建议：

- 如果你要等待异步结果，直接 `return Promise` / `async function`，不要自己 busy loop
- `setTimeout()` 现在属于持久运行时级别；表达式返回后仍可继续触发，`clearTimeout()` 也可在后续输入中取消
- `structuredClone()` 当前只保证 JSON 可序列化结构；函数、循环引用、复杂类实例不支持

### 2.6 插件制品目录与公开上传目录隔离

平台将插件包/解压制品存放在 `plugin.artifact_dir`（默认 `data/plugins`）。

重要约束：

- `plugin.artifact_dir` 不允许配置到 `upload.dir` 下。
- 这样做是为了避免插件源码随 `/uploads` 静态目录被公开访问。
- 插件图标/配置文件等应通过 `Plugin.fs` 读取，而不是直接拼公网静态 URL 读取插件源码。

---

## 3. Hook 机制

平台通过 `hook.execute` 触发业务扩展，参数结构：

- `params.hook`：Hook 名称
- `params.payload`：JSON 字符串

当前不要再手抄 Hook 名称；以“宿主注册表 + SDK 导出目录 + 调试器示例”作为准源：

- 宿主注册表：`backend/internal/service/plugin_hook_registry.go`
- 管理端 catalog API：`GET /api/admin/plugins/hooks/catalog`
- SDK 常量导出：
  - `OFFICIAL_PLUGIN_HOOKS`
  - `OFFICIAL_PLUGIN_HOOK_GROUPS`
  - `OFFICIAL_FRONTEND_SLOTS`
  - `OFFICIAL_PLUGIN_PERMISSION_KEYS`
  - `OFFICIAL_HOST_PERMISSION_KEYS`
- SDK 辅助函数：
  - `listOfficialPluginHooks(group?)`
  - `listOfficialFrontendSlots()`
  - `listOfficialPluginPermissionKeys()`
  - `listOfficialHostPermissionKeys()`
  - `isOfficialPluginHook(...)`
  - `isOfficialFrontendSlot(...)`
  - `isOfficialPluginPermissionKey(...)`
  - `isOfficialHostPermissionKey(...)`
  - `inspectPluginManifestCompatibility(...)`

当前这套代码里，官方目录已经扩展到：

- `261` 个官方 Hook
- `342` 个前端插槽
- `58` 个官方插件权限键
- `50` 个 `host.*` 权限键

推荐分组（也是 `js-worker-debugger` 当前内置分组）：

- `frontend`：`frontend.bootstrap` / `frontend.slot.render`
- `auth`：认证、绑定、偏好、管理端用户治理
- `platform`：插件生命周期、包治理、API Key、上传、日志
- `commerce`：购物车、订单、支付、优惠码、序列号
- `catalog`：商品、库存、虚拟库存
- `support`：工单链路
- `content`：公告、知识库、营销、邮件、短信
- `settings`：系统设置、邮件模板、落地页、模板包

> 你的插件可在 `capabilities.hooks` 中声明要处理的 Hook。
>
> 现在建议直接复用 `plugins/sdk` 的官方目录导出，而不是在插件里手维护一份常量。

### 3.1 前端扩展协议（`frontend.slot.render` / `frontend.bootstrap`）

两类前端 Hook 都通过 `hook.execute` 触发，区别在于调用场景：

1. `frontend.slot.render`：

- 用于“现有页面插槽”注入。
- 平台请求 `payload` 典型字段：
  - `path`：当前页面路径
  - `slot`：插槽名（例如 `admin.orders.top`）
  - `query_params` / `query_string` / `full_path`：当前页面查询参数快照
  - `host_context`：宿主页面主动透传的精简上下文（例如当前订单摘要、筛选条件、勾选数量）

2. `frontend.bootstrap`：

- 用于“插件页面注册”（菜单 + 路由声明）。
- 平台请求 `payload` 典型字段：
  - `area`：`user` / `admin`
  - `path`：当前访问路径
  - `slot`：固定为 `bootstrap`

插件返回时，使用 `frontend_extensions` 数组。平台会按 `type` 识别：

- 菜单项：`menu_item` / `menu` / `nav_item`
- 页面路由：`route_page` / `page_route` / `route` / `plugin_page`
- 页面插槽内容：`text`、`html`、`alert`、`key_value`、`table` 等结构化块
- 动作区按钮：`action_button` / `toolbar_button` / `button`

管理端已新增一批更细粒度的订单扩展点：

- `admin.orders.actions`
- `admin.orders.batch_actions`
- `admin.orders.row_actions`
- `admin.order_detail.top`
- `admin.order_detail.actions`
- `admin.order_detail.info_actions`
- `admin.order_detail.product_actions`
- `admin.order_detail.virtual_stock_actions`
- `admin.order_detail.shipping_actions`
- `admin.order_detail.serials_actions`
- `admin.order_detail.bottom`

补充：

- 宿主在订单列表这类高重复视图中，会优先通过批量扩展接口聚合请求，再把结果分发到每一行，避免前端出现明显的 N+1 网络请求。
- 批量扩展接口在宿主侧会对等价请求去重，并以有界并发执行，降低高密度 slot 的总等待时间，同时避免瞬间放大插件执行压力。

示例：

```json
{
  "success": true,
  "frontend_extensions": [
    {
      "type": "menu_item",
      "data": {
        "area": "admin",
        "path": "/admin/plugin-pages/ticket-assistant",
        "title": "Ticket Assistant",
        "priority": 100,
        "required_permissions": ["ticket.view"],
        "super_admin_only": false
      }
    },
    {
      "type": "route_page",
      "data": {
        "area": "admin",
        "path": "/admin/plugin-pages/ticket-assistant",
        "title": "Ticket Assistant",
        "required_permissions": ["ticket.view"],
        "page": {
          "title": "Ticket Assistant",
          "description": "Plugin powered page",
          "blocks": [
            {
              "type": "text",
              "title": "Overview",
              "content": "This page is provided by plugin."
            }
          ]
        }
      }
    }
  ]
}
```

SDK 等价写法：

```ts
import {
  buildPluginPageBootstrap,
  buildPluginTextBlock
} from "@auralogic/plugin-sdk";

const frontendExtensions = buildPluginPageBootstrap({
  area: "admin",
  path: "/admin/plugin-pages/ticket-assistant",
  title: "Ticket Assistant",
  priority: 100,
  required_permissions: ["ticket.view"],
  page: {
    title: "Ticket Assistant",
    description: "Plugin powered page",
    blocks: [
      buildPluginTextBlock("This page is provided by plugin.", "Overview")
    ]
  }
});
```

插槽动作按钮示例：

```ts
import { buildPluginActionButtonExtension } from "@auralogic/plugin-sdk";

const frontendExtensions = [
  buildPluginActionButtonExtension({
    slot: "admin.orders.actions",
    title: "Open Logistics",
    href: "/admin/plugin-pages/logistics?source=orders",
    icon: "truck",
    variant: "outline",
    size: "sm"
  })
];
```

补充：

- `Plugin.fs.usage()` 返回当前数据层配额使用量（文件数/总字节）。
- Worker 内部对 usage 使用增量缓存，避免每次写入前全量扫描目录。
- 当检测到外部进程直接修改插件数据目录导致缓存偏差时，可调用 `Plugin.fs.recalculateUsage()` 触发重算并刷新缓存。

路径限制（平台强约束）：

- 用户侧只能注册到 `/plugin-pages/` 前缀。
- 管理侧只能注册到 `/admin/plugin-pages/` 前缀。
- 不符合前缀的菜单/路由会被平台直接丢弃。
- 若插件希望管理端列表页能直接跳到“默认管理页”，建议在 `manifest.json` 额外声明：
  - `frontend.admin_page.path`
  - `frontend.admin_page.title`（可选）
  - `frontend.user_page.path` / `frontend.user_page.title`（可选）
- 该声明只用于宿主做默认跳转入口；页面真正是否可访问，仍以后端执行 `frontend.bootstrap` 返回的路由为准。

权限与可见性字段：

- `required_permissions`：管理端菜单/路由可见性校验。
- `super_admin_only`：仅超级管理员可见/可访问。
- `guest_visible`：用户侧访客可见（仅用户侧生效）。
- `mobile_visible`：用户侧移动端底部导航可见性。
- 预筛选（插件级，写在 `capabilities`）：
  - `frontend_min_scope`：`guest` / `authenticated` / `super_admin`。
  - `frontend_required_permissions`：访问方需具备的权限列表（全部满足）。
  - `frontend_allowed_areas`：`user` / `admin`，不匹配时平台在执行前直接跳过插件。
  - `allowed_frontend_slots`：请求 slot 不匹配时，平台在执行前直接跳过插件。

插件页面渲染能力（当前阶段）：

- 路由页面支持 `page.blocks` 结构化渲染。
- 目前内置块类型：`text`、`html`、`link_list`、`action_form`、`alert`、`key_value`、`json_view`、`table`、`badge_list`、`stats_grid`。
- `html` 块默认经过平台前端清洗（移除脚本注入相关标签/属性），不应依赖内联脚本执行逻辑。
- 仅当同时满足以下条件时，`capabilities.frontend_html_mode=trusted` 才会生效：
  - 插件已请求并被授予权限 `frontend.html_trusted`
  - 系统未开启全局开关 `plugin.frontend.force_sanitize_html`（Kill Switch）
- 插件页面额外挂载插槽：
  - `admin.plugin_page.top`
  - `admin.plugin_page.bottom`
  - `user.plugin_page.top`
  - `user.plugin_page.bottom`

`action_form`（插件页，管理端/用户端均可）：

- 适合插件通过页面直接调用自身 `execute` 动作（例如读取/保存插件配置）。
- 平台会按当前匹配插件页下发 `execute_api` 元数据，`action_form` 会自动调用该端点。
- 若页面或 block 声明了 `stream_actions` / `execute_stream_actions`，平台会额外下发 `execute_api.stream_url`、`execute_api.stream_format=ndjson` 与 `execute_api.stream_actions`，前端会按声明切换到流式执行。
- `html` 块也可通过 `data-plugin-exec-*` bridge 调用同一路由执行端点。
- `html` bridge 可用 `data-plugin-exec-mode="stream"` 显式请求流式执行，但对应 action 仍需在页面 schema 的流式动作声明中出现。
- 插件页挂载后，宿主会额外暴露页面级 JS bridge：
  - `window.axios` / `window.pluginAxios`：宿主配置过的 axios 实例（复用现有 token / locale 拦截器）。
  - `window.AuraLogicPluginPage`：当前插件页上下文与 JS bridge，包含 `path/full_path/query_params/route_params/html_mode/execute_api`，以及 `execute()` / `should_stream()`。
  - 注意：这是插件页运行时 bridge，不代表 `html` 块内嵌 `<script>` 标签会自动执行。
- 推荐优先复用 `plugins/sdk` helper：`buildPluginPageBootstrap`、`buildPluginActionFormBlock`、`buildPluginExecuteHTMLBridge`、`buildPluginAlertBlock`、`buildPluginKeyValueBlock`、`successResult`、`errorResult`。
- 推荐写法：

```ts
import {
  buildPluginActionFormBlock,
  buildPluginAlertBlock,
  buildPluginKeyValueBlock,
  buildPluginPageBootstrap
} from "@auralogic/plugin-sdk";

const page = {
  title: "Plugin Config",
  blocks: [
    buildPluginAlertBlock({
      title: "Ready",
      content: "This page is provided by frontend.bootstrap.",
      variant: "success"
    }),
    buildPluginKeyValueBlock({
      title: "Route Meta",
      items: [
        { label: "Area", value: "admin" },
        { label: "Path", value: "/admin/plugin-pages/demo" }
      ]
    }),
    buildPluginActionFormBlock({
      title: "Plugin Config",
      load: "plugin.config.get",
      loadLabel: "Load",
      save: "plugin.config.set",
      saveLabel: "Save",
      reset: "plugin.config.reset",
      resetLabel: "Reset",
      fields: [
        { key: "enabled", type: "boolean", label: "Enable Feature" }
      ]
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

### 3.2 可改写字段治理（重要）

- 平台对当前内置 Hook 已启用显式治理。
- 允许改写的 Hook 使用“可写字段白名单”；例如：`order.create.before` 只允许改写 `items/remark/promo_code`。
- 当前新增放开的内置点位包括：`product.create.before`、`product.update.before`、`product.delete.before`、`payment.confirm.before`、`ticket.attachment.upload.before`、`inventory.reserve.before`、`order.admin.mark_paid.before`，且仍只允许白名单字段。
- 只读 Hook 的 `payload` 会被平台整体丢弃并记录日志。
- 未登记的自定义 Hook 仍保持兼容策略（不过滤 `payload`）；如需收口，建议同步补充到平台 Hook 注册表。

### 3.3 当前 Hook 覆盖范围（摘要）

按域摘要：

- `frontend`：前端 bootstrap / slot 渲染
- `auth`：注册、登录、重置密码、改密码、绑定邮箱/手机、偏好更新、管理端用户创建/更新/删除、权限更新
- `platform`：插件创建/更新/删除、包上传、版本激活/删除、生命周期（install/start/pause/restart/resume/retire/hot_reload）、市场安装、secret 更新、API Key、上传、日志重试
- `commerce`：购物车、订单全链路、支付方式、支付包、支付 market 安装、支付 webhook、优惠码、序列号、订单导入导出
- `catalog`：商品、库存、库存绑定、虚拟库存、虚拟库存绑定、库存导入/手动预占释放
- `support`：工单创建、消息、附件、指派、状态、自动关闭、订单分享
- `content`：公告、知识库、营销、邮件发送、短信发送
- `settings`：系统设置、邮件模板、落地页、模板包

如果你需要“精确到单个 Hook 名”的最新列表，不要再翻历史计划文档，直接：

1. 用 SDK 的 `OFFICIAL_PLUGIN_HOOKS`
2. 或调用管理端 `/api/admin/plugins/hooks/catalog`
3. 或打开 `plugins/js-worker-debugger` 的页面与测试

---

## 4. JS Worker 插件开发

## 4.1 最小入口

`index.js` 至少导出 `execute`：

```javascript
function execute(action, params, context, config, sandbox) {
  return { success: true, message: "ok" };
}
```

可选导出 `health`：

```javascript
function health(config, sandbox) {
  return { healthy: true, version: "1.0.0" };
}
```

## 4.2 一个 zip 包内多文件导入（CommonJS）

`js_worker` 支持在插件包内使用 CommonJS `require` 导入其他 JS 文件：

```javascript
const helper = require("./lib/helper");
```

支持能力：

- `require("./relative/path")`
- `require("package-name")` / `require("@scope/package")`（从插件包内 `node_modules` 解析）
- 可省略 `.js` 扩展名
- 可加载目录 `index.js`
- 多层级模块互相调用
- 循环依赖（按 CommonJS 缓存语义）

约束：

- npm 包解析范围仍限制在插件包根目录内的 `node_modules/`
- 不能越过插件包根目录访问外部文件。
- 推荐在构建阶段用 `esbuild` / `rollup` 把运行时依赖 bundle 成单文件 `index.js`，避免上传 zip 携带大量 `node_modules/`。
- 即使已支持 npm 包导入，也不代表插件拥有完整 Node.js / 浏览器运行时；依赖若要求 `window/document/fetch/process/Buffer` 等全局对象，仍需额外适配。

## 4.3 访问插件包内资源（icon / 配置）

示例：

```ts
import {
  asRecord,
  definePlugin,
  errorResult,
  getPluginFS,
  successResult,
  type UnknownMap
} from "@auralogic/plugin-sdk";

module.exports = definePlugin({
  execute(action: unknown, params: unknown) {
    const fs = getPluginFS();
    if (!fs || !fs.enabled) {
      return errorResult("Plugin.fs is unavailable");
    }

    if (action === "plugin.icon.load") {
      const iconBase64 = fs.readBase64("assets/icon.png");
      return successResult({
        values: {
          icon_data_uri: "data:image/png;base64," + iconBase64
        }
      });
    }

    if (action === "plugin.config.load") {
      const cfg = fs.exists("data/config.json")
        ? fs.readJSON<UnknownMap>("data/config.json")
        : {};
      return successResult({
        values: {
          config: cfg || {}
        }
      });
    }

    if (action === "plugin.config.save") {
      fs.writeJSON("data/config.json", {
        ...asRecord(params),
        updated_at: Date.now()
      });
      return successResult({
        message: "Config saved."
      });
    }

    return successResult();
  }
});
```

## 4.4 Hook 返回约定

当 `action === "hook.execute"` 时，建议返回：

```json
{
  "success": true,
  "payload": {
    "any_key": "any_value"
  }
}
```

`payload` 会参与平台侧 Hook 聚合逻辑。

## 4.5 SDK 快速参考（推荐）

如果你在写 `js_worker` 插件，建议直接复用 `plugins/sdk`，不要每个插件手写一套解析逻辑。

常用入口：

- 运行时访问：
  - `getPluginWorkspace()`
  - `getPluginRuntime()`
  - `getPluginStorage()`
  - `getPluginFS()`
  - 全局 `Worker`
- 基础解析：
  - `asString()`
  - `asBool()`
  - `asInteger()`
  - `asRecord()`
  - `safeParseJSON()`
  - `safeParseStringMap()`
  - `normalizePluginPermissionKey()`
- FS 兼容：
  - `normalizePluginFSUsage()`：兼容历史运行时字段大小写差异
- 官方目录：
  - `OFFICIAL_PLUGIN_HOOKS`
  - `OFFICIAL_PLUGIN_HOOK_GROUPS`
  - `OFFICIAL_FRONTEND_SLOTS`
  - `OFFICIAL_PLUGIN_PERMISSION_KEYS`
  - `OFFICIAL_HOST_PERMISSION_KEYS`
  - `listOfficialPluginHooks()`
  - `listOfficialFrontendSlots()`
  - `listOfficialPluginPermissionKeys()`
  - `listOfficialHostPermissionKeys()`
  - `isOfficialPluginHook()`
  - `isOfficialFrontendSlot()`
  - `isOfficialPluginPermissionKey()`
  - `isOfficialHostPermissionKey()`
  - `validatePluginManifestCatalog()`
  - `validatePluginManifestSchema()`
- 插件页上下文：
  - `resolvePluginPageContext()`：把 `context.metadata` 中的插件页 path/query/route 信息还原成结构化对象
- 插件页 block builder：
  - `buildPluginTextBlock()`
  - `buildPluginAlertBlock()`
  - `buildPluginKeyValueBlock()`
  - `buildPluginJSONViewBlock()`
  - `buildPluginTableBlock()`
  - `buildPluginBadgeListBlock()`
  - `buildPluginLinkListBlock()`
  - `buildPluginStatsGridBlock()`
  - `buildPluginActionFormBlock()`
  - `buildPluginHTMLBlock()`
- 插件页/菜单注册：
  - `buildPluginMenuItemExtension()`
  - `buildPluginRoutePageExtension()`
  - `buildPluginPageBootstrap()`
- 原生宿主桥 getter：
  - `getPluginMarket()`
  - `getPluginEmailTemplate()`
  - `getPluginLandingPage()`
  - `getPluginInvoiceTemplate()`
  - `getPluginAuthBranding()`
  - `getPluginPageRulePack()`
- 插件页自执行：
  - `buildPluginExecuteHTMLBridge()`
  - `buildPluginExecuteTemplatePlaceholder()`
  - `buildPluginExecuteTemplateValues()`
  - `renderPluginTemplate()`

建议：

- `Plugin.fs` 一律使用相对路径，例如 `assets/icon.png`、`notes/demo.txt`
- 需要展示文件系统配额时，优先显示 `usage()` 的 snake_case 字段
- 如果怀疑外部进程改过插件数据目录，再调用 `recalculateUsage()`
- 如果你想在打包前直接校验 manifest 里的 `hooks / disabled_hooks / allowed_frontend_slots / requested_permissions / granted_permissions / permissions[].key`，优先用 `validatePluginManifestCatalog()`
- 如果你还要补上 `config_schema / runtime_params_schema / frontend 路径 / webhooks / compatibility metadata` 这层结构校验，继续叠加 `validatePluginManifestSchema()`
- 若你只想拿到和宿主一致的版本兼容性原因码，直接用 `inspectPluginManifestCompatibility()`
- 如果你想直接在插件目录里接入统一的打包前校验脚本，可直接复用 `node ../sdk/scripts/validate-plugin-manifest.mjs .`
- 如果你维护官方 debugger 示例，不想再手工维护超长的 hook / slot / permission 列表，可直接复用 `node ../sdk/scripts/sync-plugin-manifest.mjs . --profile=debugger`
- 如果你维护官方 template / market 示例，也可以分别复用 `--profile=template`、`--profile=market`
- 如果你想一次性检查三套官方示例 manifest 是否都没有漂移，可以直接运行 `cd plugins/sdk && npm run check:sample-manifests`
- 如果宿主新增了 Hook / `host.*` 权限 / 前端 slot，先运行 `cd plugins/sdk && npm run generate:catalog`，再更新示例插件或打包产物

最小 FS 配额显示示例：

```ts
import {
  buildPluginJSONViewBlock,
  getPluginFS,
  normalizePluginFSUsage
} from "@auralogic/plugin-sdk";

const fs = getPluginFS();
const usage = fs ? normalizePluginFSUsage(fs.usage(), fs) : undefined;

const usageBlock = buildPluginJSONViewBlock({
  title: "Filesystem Usage",
  value: usage
});
```

---

## 5. 工单自动回复示例（默认 JS 示例）

示例目录：

- `plugins/js-worker-ticket-auto-reply/manifest.json`
- `plugins/js-worker-ticket-auto-reply/index.js`
- `plugins/js-worker-ticket-auto-reply/lib/ticket-hook.js`

该示例处理 `ticket.create.after`，返回 `auto_reply` 指令，平台会自动向新工单插入一条系统回复消息。

`auto_reply` 结构：

```json
{
  "auto_reply": {
    "enabled": true,
    "content": "感谢提交工单，我们会尽快处理。",
    "content_type": "text",
    "sender_name": "System Bot",
    "mark_processing": true,
    "metadata": {
      "source": "plugin"
    }
  }
}
```

### 5.2 Plugin Debugger 示例（端点/Hook/运行时联调）

示例目录：

- `plugins/js-worker-debugger/manifest.json`
- `plugins/js-worker-debugger/src/index.ts`
- `plugins/js-worker-debugger/src/lib/debugger.ts`

用途：

- 用作“统一调试器插件”，判断系统各业务端点触发的 Hook 是否正常进入插件链路。
- 同时演示 `frontend.bootstrap`、`frontend.slot.render`、`Plugin.storage`、`Plugin.fs`、payload patch、hook block 与 sandbox 权限画像。

管理端配置方式：

1. 上传并启动 `plugin-debugger` 插件。
2. 打开插件专属页：`/admin/plugin-pages/debugger`。
3. 在页面里直接查看最近 Hook 事件、sandbox 能力、storage / filesystem 摘要。
4. 使用页面内可视化表单切换 Hook 组、模拟 Hook、验证 `Plugin.storage` / `Plugin.fs`。

诊断建议：

- 触发对应业务端点后，到调试器页面的 `Recent Hook Events` 查看是否已捕获到真实载荷。
- 前端 Hook 开启时，可在页面插槽和插件专属页导航看到调试器扩展块。

---

## 6. 打包与上传（JS Worker）

推荐 zip 包结构：

```text
manifest.json
index.js
lib/
  ticket-hook.js
  utils/
    payload.js
    reply.js
```

`manifest.json` 示例：

```json
{
  "name": "ticket-auto-reply",
  "display_name": "Ticket Auto Reply",
  "type": "custom",
  "runtime": "js_worker",
  "entry": "index.js",
  "version": "1.0.0",
  "manifest_version": "1.0.0",
  "protocol_version": "1.0.0",
  "min_host_protocol_version": "1.0.0",
  "config": {
    "greeting": "hello from template"
  },
  "config_schema": {
    "title": "Template Config",
    "description": "Optional visual config schema for admin UI.",
    "fields": [
      {
        "key": "greeting",
        "label": "Greeting",
        "type": "textarea",
        "default": "hello from template"
      }
    ]
  },
  "frontend": {
    "admin_page": {
      "path": "/admin/plugin-pages/ticket-auto-reply",
      "title": "Ticket Auto Reply"
    }
  },
  "capabilities": {
    "hooks": ["ticket.create.after", "frontend.slot.render"],
    "requested_permissions": ["hook.execute", "frontend.extensions"],
    "granted_permissions": ["hook.execute", "frontend.extensions"],
    "allow_frontend_extensions": true,
    "frontend_min_scope": "authenticated",
    "frontend_allowed_areas": ["user"],
    "frontend_required_permissions": [],
    "allowed_frontend_slots": ["user.tickets.top", "user.plugin_page.top"]
  },
  "runtime_params": {
    "tenant": "demo-tenant"
  },
  "runtime_params_schema": {
    "title": "Template Runtime Params",
    "description": "Optional visual runtime_params schema for admin UI.",
    "fields": [
      {
        "key": "tenant",
        "label": "Tenant",
        "type": "string",
        "default": "demo-tenant"
      }
    ]
  }
}
```

兼容性字段说明：

- `manifest_version`：插件 manifest schema 版本。当前宿主支持 `1.0.0`。
- `protocol_version`：插件与宿主执行协议版本。当前宿主支持 `1.0.0`。
- `min_host_protocol_version`：可兼容的最小宿主协议版本（可选）。
- `max_host_protocol_version`：可兼容的最大宿主协议版本（可选）。

当前策略：

- 未声明上述字段时，平台会按“兼容旧插件”策略自动假定为当前宿主版本；
- 但管理端诊断会提示这是隐式兼容，建议新插件显式声明；
- 若声明了更高的 `manifest_version` / `protocol_version`，或宿主版本不在 `min/max` 范围内，上传、激活、启动注册都会被拒绝。

可选 manifest 可视化 schema（管理端使用）：

- `config_schema.fields`：定义 `config` 的预设可视化字段。
- `runtime_params_schema.fields`：定义 `runtime_params` 的预设可视化字段。
- 当前支持字段类型：`string`、`textarea`、`number`、`boolean`、`select`、`json`。
- 常用字段属性：`key`、`label`、`description`、`type`、`placeholder`、`default`、`required`、`options`。
- 未在 schema 中声明的字段，管理员仍可在“补充字段”或高级 JSON 编辑器中继续维护。

上传接口：

- `POST /api/admin/plugins/upload`（`multipart/form-data`）

支持表单字段：

- `plugin_id`（可选，覆盖已有插件）
- `runtime`
- `address` / `entry`
- `config`
- `runtime_params`
- `activate`
- `auto_start`

上传后的 JS 插件包会解压到 `plugin.artifact_dir` 下并解析入口脚本。
该目录默认不对公网静态暴露。

---

## 7. 本地调试流程

1. 管理端系统设置里开启插件平台，确认允许 `js_worker`。
2. 上传示例 zip 并激活版本。
3. 启动插件（生命周期 `start`）。
4. 在用户端创建工单，观察工单消息是否自动出现系统回复。
5. 若失败，查看：

- 插件执行日志（管理端插件页）
- 后端日志（`ticket.create.after` hook 错误）

## 7.1 平台执行策略（由管理员配置）

系统设置里的 `plugin.execution` 支持：

- `hook_max_inflight`：并发闸门
- `hook_max_retries`：执行错误重试次数
- `hook_retry_backoff_ms`：重试退避
- `hook_before_timeout_ms`：`*.before` 超时预算
- `hook_after_timeout_ms`：`*.after`/事件 Hook 超时预算
- `failure_threshold`：连续失败达到阈值后触发断路器
- `failure_cooldown_ms`：断路器打开后的冷却窗口；窗口结束后宿主只会放行一个半开探测请求

系统设置中的插件沙箱还可配置：

- `sandbox.max_memory_mb`：单次执行允许的最大堆增长预算（MB）
- `sandbox.max_concurrency`：Worker 全局并发上限
- `sandbox.js_allow_file_system`：是否允许 `Plugin.fs`
- `js_fs_max_files`：插件目录最大文件数
- `js_fs_max_total_bytes`：插件目录最大总容量（字节）
- `js_fs_max_read_bytes`：单次读取上限（字节）
- `js_storage_max_keys`：插件存储最大键数量
- `js_storage_max_total_bytes`：插件存储最大总容量（字节，`key+value`）
- `js_storage_max_value_bytes`：插件存储单值上限（字节）
- `artifact_dir`：插件制品目录（必须与 `upload.dir` 隔离）

说明：

- 超时和重试在平台侧统一执行，插件无需自己重复做一遍。
- `before` 建议快速返回，避免影响主链路时延。
- JS Worker 并发是双层限制：`Worker 全局并发` + `插件级并发`（按插件独立计数，取请求沙箱与全局上限的更严格值）。
- 当同一插件连续触发内存超限达到阈值时，会触发短时熔断冷却（默认 30 秒），冷却期内该插件执行会被拒绝。

## 7.2 观测与审计

系统已内置插件观测快照，管理员可通过以下接口读取：

- `GET /api/admin/plugins/observability`（需 `super_admin + system.config`）

当前包含指标：

- 执行耗时（平均/最大）
- 执行成功/失败与错误率
- 超时次数与超时率
- Hook 并发闸门限流命中次数
- 公开插件接口（`/api/config/plugin-extensions`、`/api/config/plugin-bootstrap`）限流命中与缓存命中率

此外，插件执行会输出结构化审计事件日志（参数与结果字段会做脱敏与截断）。

---

## 8. gRPC 插件开发（简要）

协议文件：

- `backend/proto/plugin.proto`

实现服务：

- `HealthCheck`
- `Execute`
- `ExecuteStream`（可选）

`ExecuteResponse.data` 需返回 JSON 字符串，平台会解析为对象。

---

## 9. 安全建议

- 不要在插件中保存明文密钥，优先走 `config/runtime_params` 注入。
- 对 `params`、`payload` 做严格校验。
- JS 插件尽量保持纯计算逻辑，涉及外部调用时遵循系统沙箱策略。
- 生产环境建议显式配置 Worker 可执行路径，不依赖 `go run`。
