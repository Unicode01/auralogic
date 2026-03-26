# JS Worker Template

AuraLogic `js_worker` 插件最小模板，已同步到当前 SDK 写法。

## 包含内容

- TypeScript 入口：`src/index.ts`
- 本地 SDK 依赖：`@auralogic/plugin-sdk`
- `frontend.bootstrap` 注册的管理端/用户端插件页
- 可视化 `action_form` 示例
- HTML execute bridge 示例
- `Plugin.order / Plugin.user` 原生桥读取示例
- `Plugin.workspace` 命令与交互输入示例
- `Plugin.storage` 读写示例
- `buildPluginAlertBlock` / `buildPluginKeyValueBlock` / `buildPluginJSONViewBlock` 示例
- `esbuild` 单文件 bundle 构建
- 后台可直接上传的 zip 打包脚本

## 当前版本

- 模板版本：`0.2.2`
- 产物文件：`js-worker-template.zip`

## 常用命令

1. `make deps`
2. `make typecheck`
3. `npm run sync:manifest`
4. `npm run validate:manifest`
5. `make build`
6. `make package`

## 上传使用

1. 执行 `make package`
2. 在管理后台上传 `js-worker-template.zip`
3. 激活并启动插件
4. 打开：
   - `/admin/plugin-pages/js-worker-template`
   - `/plugin-pages/js-worker-template`

## 这个模板演示了什么

- `hook.execute`
  - 处理 `frontend.bootstrap`
  - 返回插件页菜单和路由
- `template.page.get`
  - 从 manifest 默认值或 `Plugin.storage` 读取页面状态
- `template.page.save`
  - 把表单状态写入 `Plugin.storage`
- `template.echo`
  - 演示插件页通过自身 execute API 自调用
  - 展示当前 action 的声明存储模式与运行时实际访问模式
  - 通过 `resolvePluginPageContext()` 展示插件页 full path、query params、route params
- `template.host.lookup`
  - 演示插件页如何通过 `Plugin.order.get()` / `Plugin.user.get()` 访问宿主原生实体
  - 支持从 `?order_id`、`:orderNo`、`user_id`、`user_email` 自动拼默认查询
- `template/context`
  - 管理端 workspace 命令，适合直接验证 shell 变量展开后的 argv
- `template/prompt`
  - 管理端 interactive workspace 命令，读取一行输入并回写到 `Plugin.storage`

## 对应实现位置

- 插件主入口：`src/index.ts`
- 页面与前端扩展：`src/lib/frontend.ts`
- 常量：`src/lib/constants.ts`
- 打包脚本：`scripts/package.mjs`

## 当前 SDK 用法

模板已切到 SDK helper，适合直接复制起步：

- 运行时访问：
  - `getPluginStorage()`
- 插件页上下文：
  - `resolvePluginPageContext()`
- 工作台：
  - `getPluginWorkspace()`
- 原生宿主桥：
  - `getPluginOrder()`
  - `getPluginUser()`
- 插件定义：
  - `definePlugin()`
  - `successResult()`
  - `errorResult()`
- 页面 block：
  - `buildPluginAlertBlock()`
  - `buildPluginKeyValueBlock()`
  - `buildPluginJSONViewBlock()`
  - `buildPluginActionFormBlock()`
  - `buildPluginHTMLBlock()`
- 页面注册：
  - `buildPluginPageBootstrap()`
- HTML 自执行：
  - `buildPluginExecuteHTMLBridge()`
  - `buildPluginExecuteTemplatePlaceholder()`
- 沙箱执行画像：
  - `sandbox.currentAction`
  - `sandbox.declaredStorageAccessMode`
  - `sandbox.storageAccessMode`
  - `sandbox.executeActionStorage`

## 说明

- 运行时依赖默认 bundle 进 `index.js`
- zip 默认不再携带 `node_modules`
- `npm run package` 现在会先走一轮 SDK 提供的 manifest 校验，提前拦住 schema / hook / permission / compatibility 漂移
- `npm run sync:manifest` 会把模板示例的兼容性字段、Hook、slot 和权限目录同步回官方模板基线
- workspace 命令目录现在直接从 `definePlugin({ workspace })` 自动导出，不再需要 `manifest.workspace`
- 模板页的上下文卡片现在会直接显示当前 action 的声明存储模式和实际模式，适合开发期快速验收 `execute_action_storage`
- 模板页现在也会直接演示：
  - typed `Plugin.order / Plugin.user` 宿主查询
  - workspace 命令与 `Plugin.workspace.readLine()`
- 模板页还会同时演示：
  - 执行阶段如何通过 `resolvePluginPageContext()` 读取 `query/route/full_path`
  - HTML 模板阶段如何使用 `plugin.full_path`、`plugin.query.<key>`、`plugin.route.<key>`
- 在管理端工作台里可以直接试：
  - `template/context $PLUGIN_NAME $WORKSPACE_STATUS`
  - `template/prompt`
- 如果你要做更复杂的 Hook / FS / Sandbox 演示，直接参考 `plugins/js-worker-debugger`
