import type {
  PluginExecutionContext,
  PluginExecuteResult,
  PluginFrontendExtension,
  PluginPageBlock,
  PluginPageSchema,
  UnknownMap
} from "@auralogic/plugin-sdk";

export type MarketLocale = "zh" | "en";

const MARKET_I18N_TOKEN_PREFIX = "__market_i18n__:";

const KEYED_MESSAGES = {
  "frontend.page.description": [
    "浏览可信市场源，先进入可安装包或宿主管理制品工作台，再执行预览、安装 / 导入、任务排查、历史与回滚。",
    "Browse trusted market sources, enter either the installable-package workspace or the host-managed artifact workspace, then run preview, install/import, task inspection, history, and rollback."
  ],
  "frontend.notice.marketAccess.title": ["市场接入说明", "Market Access Notes"],
  "frontend.notice.marketAccess.content": [
    "市场插件只会访问插件配置中声明的可信源。插件包通过宿主桥接安装，支付包会先进入统一支付包导入流程确认目标与配置，宿主管理的模板与页面规则则写入对应历史。",
    "The market plugin only connects to trusted sources declared in plugin config. Plugin packages install through the host bridge, payment packages continue in the unified payment import flow, and host-managed templates or page rules write into their managed revision history."
  ],
  "frontend.overview.html": [
    `<div style="display:grid;gap:1rem;">
  <section data-plugin-surface="card">
    <span data-plugin-eyebrow="">混合工作台</span>
    <h2 data-plugin-title="">{displayName}</h2>
    <p data-plugin-text="muted" style="margin:0;">市场页现在按两条主通道拆分：可安装包，以及宿主管理制品。先从下方入口进入对应工作台，再在表单里做目录查询、版本预览、安装 / 导入、历史或回滚。</p>
    <div data-plugin-chip-list="" style="margin-top:1rem;">
      <span data-plugin-chip="">源: {sourceId}</span>
      <span data-plugin-chip="">渠道: {channel}</span>
      <span data-plugin-chip="">路由: {adminRoute}</span>
      <span data-plugin-chip="">HTML: 可用时启用 trusted，否则自动回退 sanitize</span>
    </div>
  </section>
  <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:1rem;">
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">可安装包</span>
      <h3 data-plugin-title="">插件包与支付包</h3>
      <p data-plugin-text="muted" style="margin:0 0 1rem;">这一条通道只处理可安装制品。插件包会在当前页继续预览和安装，支付包会在预览后进入原生导入流程。</p>
      <div data-plugin-actions="">
        <a href="{pluginUrl}" data-plugin-button="primary">打开插件包工作台</a>
        <a href="{paymentUrl}" data-plugin-button="secondary">打开支付包工作台</a>
      </div>
    </section>
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">宿主管理</span>
      <h3 data-plugin-title="">模板与页面规则</h3>
      <p data-plugin-text="muted" style="margin:0 0 1rem;">这一条通道处理宿主管理目标。邮件模板和落地页需要显式目标键；账单模板、认证页品牌和页面规则包使用固定宿主目标。</p>
      <div data-plugin-actions="">
        <a href="{emailUrl}" data-plugin-button="primary">邮件模板</a>
        <a href="{landingUrl}" data-plugin-button="secondary">落地页模板</a>
        <a href="{invoiceUrl}" data-plugin-button="secondary">账单模板</a>
        <a href="{authBrandingUrl}" data-plugin-button="secondary">认证页品牌模板</a>
        <a href="{pageRuleUrl}" data-plugin-button="secondary">页面规则包</a>
      </div>
    </section>
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">操作顺序</span>
      <h3 data-plugin-title="">建议操作顺序</h3>
      <ol style="margin:0 0 1rem;padding-left:1.25rem;line-height:1.7;color:inherit;">
        <li>先固定源和制品类型</li>
        <li>再查询目录或查看制品详情</li>
        <li>预览版本后再安装、导入或回滚</li>
        <li>插件包重点看任务，模板和规则重点看历史</li>
      </ol>
      <code data-plugin-meta="">默认邮件模板键: {emailKey}
默认落地页 slug: {landingSlug}</code>
      <div data-plugin-actions="">
        <a href="{overviewUrl}" data-plugin-button="secondary">回到总览上下文</a>
      </div>
    </section>
  </div>
</div>`,
    `<div style="display:grid;gap:1rem;">
  <section data-plugin-surface="card">
    <span data-plugin-eyebrow="">Hybrid Workspace</span>
    <h2 data-plugin-title="">{displayName}</h2>
    <p data-plugin-text="muted" style="margin:0;">The market page is now split into two primary lanes: installable packages and host-managed artifacts. Start from the lane below, then use the forms for catalog queries, release preview, install/import, history, and rollback.</p>
    <div data-plugin-chip-list="" style="margin-top:1rem;">
      <span data-plugin-chip="">Source: {sourceId}</span>
      <span data-plugin-chip="">Channel: {channel}</span>
      <span data-plugin-chip="">Route: {adminRoute}</span>
      <span data-plugin-chip="">HTML: trusted when available, sanitize fallback otherwise</span>
    </div>
  </section>
  <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:1rem;">
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">Packages</span>
      <h3 data-plugin-title="">Plugin and Payment Packages</h3>
      <p data-plugin-text="muted" style="margin:0 0 1rem;">Use this lane for installable artifacts only. Plugin packages continue with preview and install on this page, while payment packages move into the native import flow after preview.</p>
      <div data-plugin-actions="">
        <a href="{pluginUrl}" data-plugin-button="primary">Open Plugin Package Workspace</a>
        <a href="{paymentUrl}" data-plugin-button="secondary">Open Payment Package Workspace</a>
      </div>
    </section>
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">Host-managed</span>
      <h3 data-plugin-title="">Templates and Page Rules</h3>
      <p data-plugin-text="muted" style="margin:0 0 1rem;">Use this lane for host-managed targets. Email templates and landing pages require an explicit target key, while invoice templates, auth branding, and page rule packs use fixed host targets.</p>
      <div data-plugin-actions="">
        <a href="{emailUrl}" data-plugin-button="primary">Email Template</a>
        <a href="{landingUrl}" data-plugin-button="secondary">Landing Page Template</a>
        <a href="{invoiceUrl}" data-plugin-button="secondary">Invoice Template</a>
        <a href="{authBrandingUrl}" data-plugin-button="secondary">Auth Branding Template</a>
        <a href="{pageRuleUrl}" data-plugin-button="secondary">Page Rule Pack</a>
      </div>
    </section>
    <section data-plugin-surface="card">
      <span data-plugin-eyebrow="">Workflow</span>
      <h3 data-plugin-title="">Recommended Order</h3>
      <ol style="margin:0 0 1rem;padding-left:1.25rem;line-height:1.7;color:inherit;">
        <li>Pin the source and artifact kind first</li>
        <li>Query the catalog or inspect artifact detail next</li>
        <li>Preview before install, import, or rollback</li>
        <li>Watch tasks for plugin packages, and history for templates or rules</li>
      </ol>
      <code data-plugin-meta="">Default email template key: {emailKey}
Default landing page slug: {landingSlug}</code>
      <div data-plugin-actions="">
        <a href="{overviewUrl}" data-plugin-button="secondary">Return to Overview Context</a>
      </div>
    </section>
  </div>
</div>`
  ],
  "frontend.defaults.title": ["当前默认值", "Current Defaults"],
  "frontend.defaults.adminRoute.label": ["管理端路径", "Admin Route"],
  "frontend.defaults.defaultSource.label": ["默认源", "Default Source"],
  "frontend.defaults.defaultChannel.label": ["默认渠道", "Default Channel"],
  "frontend.defaults.defaultEmailKey.label": ["默认邮件模板键", "Default Email Template Key"],
  "frontend.defaults.defaultLandingSlug.label": ["默认落地页标识", "Default Landing Page Key"],
  "frontend.stats.commonPaths.title": ["常见路径", "Common Paths"],
  "frontend.stats.commonPaths.plugin.label": ["插件包", "Plugin Package"],
  "frontend.stats.commonPaths.plugin.value": ["4 步", "4 steps"],
  "frontend.stats.commonPaths.plugin.description": ["目录 -> 预览 -> 安装 -> 任务 / 历史", "Catalog -> Preview -> Install -> Tasks / History"],
  "frontend.stats.commonPaths.payment.label": ["支付包", "Payment Package"],
  "frontend.stats.commonPaths.payment.value": ["3 步", "3 steps"],
  "frontend.stats.commonPaths.payment.description": ["目录 -> 预览 -> 原生导入页", "Catalog -> Preview -> Native Import Page"],
  "frontend.stats.commonPaths.email.label": ["邮件模板", "Email Template"],
  "frontend.stats.commonPaths.email.value": ["4 步", "4 steps"],
  "frontend.stats.commonPaths.email.description": ["选模板键 -> 目录 -> 预览 -> 导入 / 回滚", "Choose Template Key -> Catalog -> Preview -> Import / Rollback"],
  "frontend.stats.commonPaths.landing.label": ["落地页", "Landing Page"],
  "frontend.stats.commonPaths.landing.value": ["4 步", "4 steps"],
  "frontend.stats.commonPaths.landing.description": ["选 slug -> 目录 -> 预览 -> 导入 / 回滚", "Choose Slug -> Catalog -> Preview -> Import / Rollback"],
  "frontend.quickLinks.title": ["快速入口", "Quick Links"],
  "frontend.quickLinks.overview": ["市场概览", "Market Overview"],
  "frontend.quickLinks.pluginPackage": ["插件包", "Plugin Package"],
  "frontend.quickLinks.paymentPackage": ["支付包", "Payment Package"],
  "frontend.quickLinks.emailTemplate": ["邮件模板", "Email Template"],
  "frontend.quickLinks.landingTemplate": ["落地页模板", "Landing Page Template"],
  "frontend.notice.fillingAdvice.title": ["填写建议", "Filling Advice"],
  "frontend.notice.fillingAdvice.content": [
    "包模式里的“制品名”是市场中的 artifact 名；模板模式里的“邮件模板键”或“落地页 slug”是宿主原生目标，两者通常不是同一个值。先确认目标，再检索制品名，会更稳定。",
    "In package mode, Artifact Name refers to the market artifact. In template mode, Email Template Key or Landing Page Slug refers to the native host target. They are usually not the same value, so confirm the target first and then search for the artifact."
  ],
  "frontend.packageWorkspace.title": ["包市场工作台", "Package Market Workspace"],
  "frontend.packageWorkspace.presets.plugin.label": ["插件包安装", "Plugin Package Install"],
  "frontend.packageWorkspace.presets.plugin.description": ["用于预览版本、安装插件包，并决定是否激活与自动启动。", "Preview versions, install plugin packages, and decide whether to activate and auto-start."],
  "frontend.packageWorkspace.presets.payment.label": ["支付包导入", "Payment Package Import"],
  "frontend.packageWorkspace.presets.payment.description": ["用于预览支付包版本，并跳转到统一支付包导入流程确认目标。", "Preview payment package versions and jump into the unified payment import flow to confirm the target."],
  "frontend.packageWorkspace.presets.task.label": ["任务排查", "Task Inspect"],
  "frontend.packageWorkspace.presets.task.description": ["聚焦插件包安装任务、历史记录与回滚排查。", "Focus on plugin-package install tasks, history, and rollback diagnosis."],
  "frontend.packageWorkspace.recentTitle": ["最近包上下文", "Recent Package Context"],
  "frontend.packageWorkspace.loadLabel": ["刷新推荐上下文", "Refresh Recommended Context"],
  "frontend.packageWorkspace.resetLabel": ["清空包上下文", "Clear Package Context"],
  "frontend.packageWorkspace.saveLabel": ["查询包目录", "Query Package Catalog"],
  "frontend.templateWorkspace.title": ["宿主管理模板与规则工作台", "Host-managed Template and Rule Workspace"],
  "frontend.templateWorkspace.presets.email.label": ["邮件模板导入", "Email Template Import"],
  "frontend.templateWorkspace.presets.email.description": ["将市场模板导入到系统已有的邮件模板键。", "Import a market template into an existing system email-template key."],
  "frontend.templateWorkspace.presets.landing.label": ["落地页模板导入", "Landing Page Template Import"],
  "frontend.templateWorkspace.presets.landing.description": ["将市场模板导入到系统已有的落地页 slug。", "Import a market template into an existing landing-page slug."],
  "frontend.templateWorkspace.presets.invoice.label": ["账单模板导入", "Invoice Template Import"],
  "frontend.templateWorkspace.presets.invoice.description": ["检查、预览、导入并回滚宿主管理的账单模板。", "Inspect, preview, import, and rollback the host-managed invoice template."],
  "frontend.templateWorkspace.presets.authBranding.label": ["认证页品牌模板导入", "Auth Branding Template Import"],
  "frontend.templateWorkspace.presets.authBranding.description": ["检查、预览、导入并回滚宿主管理的认证页品牌模板。", "Inspect, preview, import, and rollback the host-managed auth branding template."],
  "frontend.templateWorkspace.presets.pageRulePack.label": ["页面规则包导入", "Page Rule Pack Import"],
  "frontend.templateWorkspace.presets.pageRulePack.description": ["检查、预览、导入并回滚宿主管理的页面规则包。", "Inspect, preview, import, and rollback the host-managed page rule pack."],
  "frontend.templateWorkspace.presets.history.label": ["模板历史排查", "Template History Inspect"],
  "frontend.templateWorkspace.presets.history.description": ["保留当前模板类型和目标，仅清理版本筛选后回顾导入历史与回滚。", "Keep the current template kind and target, clear only the version filter, and then review import history and rollback."],
  "frontend.templateWorkspace.recentTitle": ["最近模板上下文", "Recent Template Context"],
  "frontend.templateWorkspace.loadLabel": ["刷新推荐上下文", "Refresh Recommended Context"],
  "frontend.templateWorkspace.resetLabel": ["清空模板上下文", "Clear Template Context"],
  "frontend.templateWorkspace.saveLabel": ["查询模板目录", "Query Template Catalog"],
  "frontend.trustedWorkspace.badge": ["受信工作台", "Trusted Workspace"],
  "frontend.trustedWorkspace.title": ["市场目录工作台", "Market Directory Workspace"],
  "frontend.trustedWorkspace.description": ["这是一页可直接浏览、筛选和操作市场制品的目录工作台。上面看概况，中间改条件，下面直接在浏览器里选条目。", "This page is a directory workspace for browsing, filtering, and operating market artifacts. Read the overview first, adjust filters in the middle, then work directly from the browser below."],
  "frontend.trustedWorkspace.guide.step1": ["先确认源、类型和渠道。", "Confirm source, kind, and channel first."],
  "frontend.trustedWorkspace.guide.step2": ["查询目录后，从下方浏览器里点选条目。", "Query the catalog, then select an item from the browser below."],
  "frontend.trustedWorkspace.guide.step3": ["先预览，再安装 / 导入；任务和回滚放在高级字段。", "Preview first, then install or import; tasks and rollback stay under advanced fields."],
  "frontend.trustedWorkspace.overview.eyebrow": ["概述", "Overview"],
  "frontend.trustedWorkspace.directory.eyebrow": ["目录分类", "Directory Categories"],
  "frontend.trustedWorkspace.directory.description": ["点击分类卡片可切换当前上下文，再回到下方筛选区执行动作。", "Click a category card to switch context, then run actions in the filter section below."],
  "frontend.trustedWorkspace.currentKind.label": ["当前分类", "Current Category"],
  "frontend.trustedWorkspace.query.eyebrow": ["查询条件", "Query Conditions"],
  "frontend.trustedWorkspace.query.title": ["查询与操作", "Query and Actions"],
  "frontend.trustedWorkspace.query.description": ["这里放源、类型、关键词、制品名、版本以及安装参数。大部分场景只需要改这一块。", "This section holds source, kind, keyword, artifact name, version, and install parameters. Most tasks only need changes here."],
  "frontend.trustedWorkspace.filter.eyebrow": ["分类筛选", "Category Filters"],
  "frontend.trustedWorkspace.filter.description": ["在当前分类下筛选源、渠道、关键词和版本。制品名为空时可先看目录。", "Filter source, channel, keyword, and version under the current category. Leave artifact name empty to browse catalog first."],
  "frontend.trustedWorkspace.browser.eyebrow": ["目录浏览器", "Directory Browser"],
  "frontend.trustedWorkspace.browser.title": ["目录浏览与快速操作", "Directory Browser and Quick Actions"],
  "frontend.trustedWorkspace.browser.description": ["左侧看树目录，右侧看表格。点击表头可排序，点击行内按钮可直接预览或安装。", "Use the tree on the left and the table on the right. Sort from the header, then preview or install from row actions."],
  "frontend.trustedWorkspace.browser.tip": ["常用方式是先查目录，再在下方直接选中条目继续预览或安装。", "A common flow is to query the catalog first, then select an item below to preview or install."],
  "frontend.trustedWorkspace.browser.status.label": ["执行状态", "Execution Status"],
  "frontend.trustedWorkspace.browser.panel.title": ["目录结果", "Catalog Results"],
  "frontend.trustedWorkspace.browser.panel.description": ["点击表头排序，点击行内按钮继续预览、查看详情或安装。", "Sort from the header, then use row actions to preview, inspect, or install."],
  "frontend.trustedWorkspace.browser.tree.title": ["目录树", "Tree"],
  "frontend.trustedWorkspace.browser.tree.description": ["按源、类型和名称快速定位条目。", "Navigate items by source, kind, and artifact name."],
  "frontend.trustedWorkspace.browser.tree.empty": ["还没有目录结果。先点一次“查询目录”。", "No catalog results yet. Run Query Catalog first."],
  "frontend.trustedWorkspace.browser.table.empty": ["当前条件下没有匹配到目录项。", "No catalog items matched the current filters."],
  "frontend.trustedWorkspace.browser.console.title": ["执行结果", "Execution Result"],
  "frontend.trustedWorkspace.browser.console.empty": ["执行结果会显示在这里。", "Execution results will appear here."],
  "frontend.trustedWorkspace.browser.selection.label": ["当前选择", "Current Selection"],
  "frontend.trustedWorkspace.browser.selection.none": ["当前还没有选中任何条目。", "No item is selected yet."],
  "frontend.trustedWorkspace.browser.selection.hint": ["查询目录后，直接在树目录或表格里点击条目即可。", "After querying the catalog, click an item from the tree or table."],
  "frontend.trustedWorkspace.browser.selection.required": ["请先在浏览器里选一个条目，或手动填写制品名。", "Select an item in the browser first, or fill the artifact name manually."],
  "frontend.trustedWorkspace.browser.summary.returned": ["返回条目", "Returned"],
  "frontend.trustedWorkspace.browser.summary.selected": ["当前选中", "Selected"],
  "frontend.trustedWorkspace.browser.actions.useSelection": ["使用当前选择", "Use Selection"],
  "frontend.trustedWorkspace.browser.actions.menu": ["操作", "Actions"],
  "frontend.trustedWorkspace.browser.actions.quickMenu": ["快速操作", "Quick Actions"],
  "frontend.trustedWorkspace.browser.actions.advancedMenu": ["高级操作", "Advanced Actions"],
  "frontend.trustedWorkspace.browser.actions.selectedMenu": ["已选条目操作", "Selected Item Actions"],
  "frontend.trustedWorkspace.browser.actions.install": ["安装", "Install"],
  "frontend.trustedWorkspace.browser.actions.import": ["导入", "Import"],
  "frontend.trustedWorkspace.browser.columns.name": ["制品名称", "Artifact Name"],
  "frontend.trustedWorkspace.browser.columns.kind": ["类型", "Kind"],
  "frontend.trustedWorkspace.browser.columns.version": ["版本", "Version"],
  "frontend.trustedWorkspace.browser.columns.size": ["大小", "Size"],
  "frontend.trustedWorkspace.browser.columns.publishedAt": ["时间", "Published At"],
  "frontend.trustedWorkspace.browser.columns.actions": ["操作", "Actions"],
  "frontend.trustedWorkspace.browser.status.ready": ["目录浏览器已就绪。", "Directory browser ready."],
  "frontend.trustedWorkspace.browser.status.loading": ["正在执行操作...", "Running action..."],
  "frontend.trustedWorkspace.browser.status.bridgeUnavailable": ["当前页面路由或能力策略下不可用执行 API。", "Execute API is unavailable for the current route or capability policy."],
  "frontend.trustedWorkspace.browser.status.loaded": ["操作已完成。", "Action completed."],
  "frontend.trustedWorkspace.browser.status.error": ["操作失败。", "Action failed."],
  "frontend.trustedWorkspace.browser.status.restored": ["已恢复上次目录结果。", "Restored previous catalog results."],
  "frontend.trustedWorkspace.browser.size.unknown": ["未知", "Unknown"],
  "frontend.trustedWorkspace.browser.version.unknown": ["未标记", "Unspecified"],
  "frontend.trustedWorkspace.modal.close": ["关闭", "Close"],
  "frontend.trustedWorkspace.modal.empty": ["当前结果没有可展示的详情。", "No detail is available for this result yet."],
  "frontend.trustedWorkspace.modal.raw": ["原始响应", "Raw Response"],
  "frontend.trustedWorkspace.result.eyebrow": ["执行结果", "Execution Result"],
  "frontend.trustedWorkspace.action.label": ["动作", "Action"],
  "frontend.trustedWorkspace.advanced": ["高级字段", "Advanced Fields"],
  "frontend.trustedWorkspace.advancedHint": ["只有调试或回滚时才需要改这里。", "Change these only for troubleshooting or rollback."],
  "frontend.trustedWorkspace.workflowStage.label": ["工作流阶段", "Workflow Stage"],
  "frontend.trustedWorkspace.runSelected": ["执行当前动作", "Run Selected Action"],
  "frontend.trustedWorkspace.ready": ["受信桥已就绪。", "Trusted bridge ready."],
  "frontend.trustedWorkspace.actions.queryCatalog": ["查询目录", "Query Catalog"],
  "frontend.actions.sourceDetail": ["查看源详情", "Inspect Source"],
  "frontend.actions.artifactDetail": ["查看制品详情", "Inspect Artifact Detail"],
  "frontend.actions.releaseDetail": ["查看版本详情", "Inspect Release Detail"],
  "frontend.actions.previewVersion": ["预览版本", "Preview Version"],
  "frontend.actions.installOrImport": ["安装 / 导入", "Install / Import"],
  "frontend.actions.queryTask": ["查询任务", "Query Task"],
  "frontend.actions.listTasks": ["列出任务", "List Tasks"],
  "frontend.actions.viewHistory": ["查看历史", "View History"],
  "frontend.actions.executeRollback": ["执行回滚", "Execute Rollback"],
  "frontend.actions.previewTemplate": ["预览模板版本", "Preview Template Version"],
  "frontend.actions.importTemplate": ["导入模板", "Import Template"],
  "frontend.actions.rollbackTemplate": ["回滚模板", "Rollback Template"],
  "frontend.fields.sourceId.label": ["源 ID", "Source ID"],
  "frontend.fields.sourceId.description": ["必须命中当前市场插件配置中声明的可信源。", "Must match a trusted source declared in the current market plugin config."],
  "frontend.fields.sourceId.placeholder": ["official", "official"],
  "frontend.packageFields.kind.label": ["包类型", "Package Type"],
  "frontend.packageFields.kind.description": ["在统一市场页切换插件包与支付包；支付包会在预览后进入支付方式页完成导入。", "Switch between plugin and payment packages in the unified market page. Payment packages continue in the payment-method page after preview."],
  "frontend.packageFields.kind.options.pluginPackage": ["插件包", "Plugin Package"],
  "frontend.packageFields.kind.options.paymentPackage": ["支付包", "Payment Package"],
  "frontend.packageFields.channel.label": ["发布渠道", "Release Channel"],
  "frontend.packageFields.channel.description": ["用于目录查询与版本预览的渠道筛选。", "Channel filter used for catalog queries and release previews."],
  "frontend.packageFields.q.label": ["检索关键词", "Search Keywords"],
  "frontend.packageFields.q.description": ["仅目录查询时使用，可按名称或关键字过滤。", "Only used for catalog queries. Filter by name or keyword."],
  "frontend.packageFields.q.placeholder": ["例如：debugger、payment、template", "For example: debugger, payment, template"],
  "frontend.packageFields.name.label": ["制品名", "Artifact Name"],
  "frontend.packageFields.name.description": ["预览、安装、导入、历史查询与回滚都会使用这个制品名。", "Previews, installs, imports, history lookups, and rollbacks all use this artifact name."],
  "frontend.packageFields.name.placeholder": ["例如：auralogic-debugger", "For example: auralogic-debugger"],
  "frontend.packageFields.version.label": ["版本号", "Version"],
  "frontend.packageFields.version.description": ["留空时由市场插件解析最新版本；先留空预览通常更省事。", "Leave empty to let the market plugin resolve the latest version. Previewing with it empty first is usually simpler."],
  "frontend.packageFields.version.placeholder": ["留空则解析最新版本", "Leave empty to resolve the latest version"],
  "frontend.packageFields.taskId.label": ["任务 ID", "Task ID"],
  "frontend.packageFields.taskId.description": ["当你只想排查插件包安装任务时填写；支付包与模板流程通常不会生成异步任务。", "Fill this only when you want to inspect plugin-package install tasks. Payment-package and template flows usually do not create async tasks."],
  "frontend.packageFields.taskId.placeholder": ["例如：pkg-install-20260315-001", "For example: pkg-install-20260315-001"],
  "frontend.packageFields.activate.label": ["安装后激活", "Activate After Install"],
  "frontend.packageFields.activate.description": ["仅插件包有效。安装完成后请求立即切换为当前生效版本。", "Plugin packages only. Request immediate activation after install completes."],
  "frontend.packageFields.autoStart.label": ["激活后自动启动", "Auto Start After Activate"],
  "frontend.packageFields.autoStart.description": ["仅插件包有效。需要和“安装后激活”配合使用。", "Plugin packages only. Use together with Activate After Install."],
  "frontend.packageFields.permissions.label": ["授予权限 JSON", "Granted Permissions JSON"],
  "frontend.packageFields.permissions.description": ["仅插件包有效。通常先保持 []，按预览里给出的新增权限再授予。", "Plugin packages only. Usually keep this as [] first, then grant permissions based on the new permissions shown in preview."],
  "frontend.packageFields.note.label": ["操作备注", "Operation Note"],
  "frontend.packageFields.note.description": ["会附加到安装、导入、历史查询或回滚审计记录里。", "Appended to install, import, history, or rollback audit records."],
  "frontend.packageFields.note.placeholder": ["例如：beta 升级验证 / 回滚排查", "For example: beta upgrade verification / rollback diagnosis"],
  "frontend.templateFields.kind.label": ["模板类型", "Template Type"],
  "frontend.templateFields.kind.description": ["决定当前处理的是哪一种宿主管理制品。邮件模板和落地页需要目标键；账单模板、认证页品牌和页面规则包使用固定宿主目标。", "Choose which host-managed artifact you are working with. Email templates and landing pages require an explicit target key, while invoice templates, auth branding, and page rule packs use fixed host targets."],
  "frontend.templateFields.channel.label": ["发布渠道", "Release Channel"],
  "frontend.templateFields.channel.description": ["用于目录查询与版本预览的渠道筛选。", "Channel filter used for catalog queries and release previews."],
  "frontend.templateFields.q.label": ["检索关键词", "Search Keywords"],
  "frontend.templateFields.q.description": ["仅目录查询时使用，可按模板名称或关键字过滤。", "Only used for catalog queries. Filter by template name or keyword."],
  "frontend.templateFields.q.placeholder": ["例如：welcome、checkout、brand", "For example: welcome, checkout, brand"],
  "frontend.templateFields.name.label": ["制品名", "Artifact Name"],
  "frontend.templateFields.name.description": ["这是市场里的模板 / 规则制品名，不是宿主目标键。", "This is the market template or rule artifact name, not the host target key."],
  "frontend.templateFields.name.placeholder": ["例如：auralogic-email-suite", "For example: auralogic-email-suite"],
  "frontend.templateFields.version.label": ["版本号", "Version"],
  "frontend.templateFields.version.description": ["留空时由市场插件解析最新版本；推荐先预览再决定是否导入。", "Leave empty to let the market plugin resolve the latest version. It is recommended to preview first and then decide whether to import."],
  "frontend.templateFields.version.placeholder": ["留空则解析最新版本", "Leave empty to resolve the latest version"],
  "frontend.templateFields.emailKey.label": ["邮件模板键", "Email Template Key"],
  "frontend.templateFields.emailKey.description": ["当模板类型为邮件模板时，导入目标必须是现有系统模板键。", "When the template type is email_template, the target must be an existing system template key."],
  "frontend.templateFields.emailKey.placeholder": ["例如：order_paid", "For example: order_paid"],
  "frontend.templateFields.landingSlug.label": ["落地页 slug", "Landing Page Slug"],
  "frontend.templateFields.landingSlug.description": ["当模板类型为落地页时，导入目标必须是现有落地页 slug。", "When the template type is landing_page_template, the target must be an existing landing-page slug."],
  "frontend.templateFields.landingSlug.placeholder": ["例如：home", "For example: home"],
  "frontend.templateFields.note.label": ["操作备注", "Operation Note"],
  "frontend.templateFields.note.description": ["会附加到模板导入、历史查询与回滚审计记录里。", "Appended to template import, history, and rollback audit records."],
  "frontend.templateFields.note.placeholder": ["例如：春节活动页替换 / 邮件文案回滚", "For example: Spring Festival campaign page replacement / email copy rollback"],
  "frontend.templateFields.kind.options.emailTemplate": ["邮件模板", "Email Template"],
  "frontend.templateFields.kind.options.landingPageTemplate": ["落地页模板", "Landing Page Template"],
  "frontend.templateFields.kind.options.invoiceTemplate": ["账单模板", "Invoice Template"],
  "frontend.templateFields.kind.options.authBrandingTemplate": ["认证页品牌模板", "Auth Branding Template"],
  "frontend.templateFields.kind.options.pageRulePack": ["页面规则包", "Page Rule Pack"],
  "index.recommendation.title": ["建议操作", "Recommended Actions"],
  "index.recommendation.primary.label": ["优先动作", "Primary Action"],
  "index.recommendation.rationale.label": ["原因", "Rationale"],
  "index.recommendation.links.title": ["快捷入口", "Shortcuts"],
  "index.workflow.common.reuse": ["复用 {label}", "Reuse {label}"],
  "index.workflow.package.title": ["包上下文快捷入口", "Package Context Shortcuts"],
  "index.workflow.package.reset": ["从空白上下文重新检索", "Search from Clean Context"],
  "index.workflow.package.switchToPlugin": ["切换到插件包", "Switch to Plugin Package"],
  "index.workflow.package.switchToPayment": ["切换到支付包", "Switch to Payment Package"],
  "index.workflow.package.clearTask": ["清除任务筛选 ({taskId})", "Clear Task Filter ({taskId})"],
  "index.workflow.template.title": ["模板上下文快捷入口", "Template Context Shortcuts"],
  "index.workflow.template.searchOther": ["在当前目标下检索其他模板", "Search Other Templates for the Current Target"],
  "index.workflow.template.switchToEmail": ["切换到邮件模板", "Switch to Email Template"],
  "index.workflow.template.switchToLanding": ["切换到落地页模板", "Switch to Landing Page Template"],
  "index.guidance.landing.noSource.action": ["先在市场插件配置里声明至少一个可信源，再使用这个工作台。", "Declare at least one trusted source in the market plugin config before using this workspace."],
  "index.guidance.landing.noSource.rationale": ["如果没有可信源，插件就无法浏览目录数据，也无法从任何注册表解析版本元数据。", "Without a trusted source, the plugin cannot browse catalog data or resolve release metadata from any registry."],
  "index.guidance.landing.invalidSource.action": ["把 source_id 切换到一个已配置可信源，再刷新当前上下文。", "Switch source_id to a configured trusted source, then refresh the current context."],
  "index.guidance.landing.invalidSource.rationale": ["当前 source_id 不存在于插件配置中，所以目录查询和详情查询都会失败，必须先修正源上下文。", "The current source_id does not exist in plugin config, so catalog and detail queries will fail until the source context is fixed."],
  "index.guidance.landing.invalidSource.useSource": ["使用 {sourceId}", "Use {sourceId}"],
  "index.guidance.landing.ready.action": ["先决定进入包模式还是模板模式，再从目录查询或源详情开始。", "Decide whether to enter package mode or template mode first, then start from catalog query or source detail."],
  "index.guidance.landing.ready.rationale": ["当前已经成功解析到可信源，接下来应该先明确是在处理插件包、支付包，还是宿主管理模板。", "A trusted source has been resolved successfully. The next step is to decide whether you are working with plugin packages, payment packages, or host-managed templates."],
  "index.guidance.landing.ready.openPlugin": ["打开插件包市场", "Open Plugin Package Market"],
  "index.guidance.landing.ready.openPayment": ["打开支付包市场", "Open Payment Package Market"],
  "index.guidance.landing.ready.openEmail": ["打开邮件模板市场", "Open Email Template Market"],
  "index.guidance.landing.ready.openLanding": ["打开落地页模板市场", "Open Landing Page Template Market"],
  "index.guidance.package.task.action": ["先查看已固定的任务 {taskId}。", "Inspect pinned task {taskId} first."],
  "index.guidance.package.task.rationale": ["当前上下文已经固定了任务 ID，先解析任务阶段是最快的判断方式，可以立即知道这个包到底成功、失败还是仍在运行。", "The current context is pinned to a task ID, so inspecting the task stage first is the fastest way to determine whether the package succeeded, failed, or is still running."],
  "index.guidance.package.task.openTask": ["打开任务 {taskId}", "Open Task {taskId}"],
  "index.guidance.package.openHistory": ["打开历史上下文", "Open History Context"],
  "index.guidance.package.noName.action": ["先查询{kind}目录。", "Query the {kind} catalog first."],
  "index.guidance.package.noName.rationale": ["当前还没有选中制品，先做目录检索是缩小范围的最快方式，之后再预览或安装会更顺手。", "No artifact is selected yet. A catalog query is the fastest way to narrow the scope before previewing or installing."],
  "index.guidance.package.viewSource": ["查看源上下文", "View Source Context"],
  "index.guidance.package.noVersion.action": ["下一步查看制品详情，或直接预览最新版本。", "Inspect artifact detail next, or preview the latest release directly."],
  "index.guidance.package.noVersion.rationale": ["当前已经选中了制品，但还没有固定版本。现在最有价值的动作就是决定先看版本列表，还是直接让预览去解析最新版本。", "An artifact is selected, but no version is pinned yet. The next valuable choice is whether to inspect the release list first or let preview resolve the latest version directly."],
  "index.guidance.package.viewArtifact": ["查看制品上下文", "View Artifact Context"],
  "index.guidance.package.ready.action": ["先预览 {release}，确认通过后再安装或导入。", "Preview {release} first, then install or import after checks pass."],
  "index.guidance.package.ready.rationale.payment": ["支付包最终仍会在原生导入器里完成，但预览是确认桥接兼容性和默认解析结果的最佳位置。", "Payment packages still finish in the native importer, but preview is the best place to confirm bridge compatibility and default resolution results."],
  "index.guidance.package.ready.rationale.plugin": ["插件包在预览确认兼容性和权限影响之后，就可以直接从当前页提交安装。", "After preview confirms compatibility and permission impact, plugin packages can be installed directly from the current page."],
  "index.guidance.package.viewVersion": ["查看版本上下文", "View Release Context"],
  "index.guidance.template.noTarget.action": ["先设置原生目标键，再查询目录或预览版本。", "Set the native target key first, then query the catalog or preview a release."],
  "index.guidance.template.noTarget.rationale": ["模板导入必须先绑定到明确的邮件模板键或落地页 slug，宿主才能比较内容并写入管理历史。", "Template import must be bound to a concrete email template key or landing-page slug before the host can compare content and write managed history."],
  "index.guidance.template.viewSource": ["查看源上下文", "View Source Context"],
  "index.guidance.template.switchToEmail": ["切换到邮件模板", "Switch to Email Template"],
  "index.guidance.template.switchToLanding": ["切换到落地页模板", "Switch to Landing Page Template"],
  "index.guidance.template.noName.action": ["下一步先查询模板目录。", "Query the template catalog next."],
  "index.guidance.template.noName.rationale": ["当前目标已经确定，接下来最有价值的动作是决定哪个模板制品和哪个版本要映射到这个原生目标上。", "The target is already fixed, so the next valuable step is deciding which template artifact and release should map to this native target."],
  "index.guidance.template.noVersion.action": ["下一步查看制品详情，或预览最新模板版本。", "Inspect artifact detail next, or preview the latest template release."],
  "index.guidance.template.noVersion.rationale": ["当前已经选中了模板制品，但还没有固定版本。现在应该在版本列表和直接预览最新版本之间做选择。", "A template artifact is selected, but no version is pinned yet. Choose between inspecting the release list and previewing the latest release directly."],
  "index.guidance.template.viewArtifact": ["查看制品上下文", "View Artifact Context"],
  "index.guidance.template.ready.action": ["先对 {targetKey} 预览 {release}，确认差异正确后再导入。", "Preview {release} against {targetKey} first, then import if the diff is correct."],
  "index.guidance.template.ready.rationale": ["模板预览会由宿主确认目标解析、摘要变化，以及这个版本是否应成为下一个受管修订。", "Template preview lets the host confirm target resolution, digest changes, and whether this release should become the next managed revision."],
  "index.guidance.template.viewVersion": ["查看版本上下文", "View Release Context"],
  "index.load.overview.sourceStatus.title": ["源状态", "Source Status"],
  "index.load.overview.sourceStatus.configuredSources.label": ["已配置源", "Configured Sources"],
  "index.load.overview.sourceStatus.currentSource.label": ["当前源", "Current Source"],
  "index.load.overview.sourceStatus.currentKind.label": ["当前类型", "Current Kind"],
  "index.load.overview.sourceStatus.revisionBackend.label": ["修订后端", "Revision Backend"],
  "index.load.overview.currentContext.title": ["当前上下文", "Current Context"],
  "index.load.overview.currentContext.adminRoute.label": ["管理端路由", "Admin Route"],
  "index.load.overview.currentContext.currentPath.label": ["当前路径", "Current Path"],
  "index.load.overview.currentContext.queryParams.label": ["查询参数", "Query Params"],
  "index.load.overview.currentContext.routeParams.label": ["路由参数", "Route Params"],
  "index.load.overview.currentContext.selectedSourceSummary.label": ["当前源摘要", "Current Source Summary"],
  "index.load.overview.currentContext.selectedSourceSummary.missing": ["当前配置中未找到该源", "This source was not found in the current config"],
  "index.load.overview.trustedSources.title": ["可信源列表", "Trusted Source List"],
  "index.load.overview.trustedSources.empty": ["当前市场插件还没有可用的可信源。", "No trusted sources are currently available to this market plugin."],
  "index.load.package.title": ["包模式", "Package Mode"],
  "index.load.package.content.payment": ["支付包走宿主导入桥接，不会产生异步任务。", "Payment packages use the host import bridge and do not create async tasks."],
  "index.load.package.content.plugin": ["插件包走宿主安装桥接，可能产生异步安装任务。", "Plugin packages use the host install bridge and may create async install tasks."],
  "index.load.package.context.title": ["包上下文", "Package Context"],
  "index.load.template.title": ["模板模式", "Template Mode"],
  "index.load.template.content": ["模板的导入、历史查询和回滚都会立即通过宿主管理的模板历史执行。", "Template import, history queries, and rollbacks run immediately through host-managed template history."],
  "index.load.template.context.title": ["模板上下文", "Template Context"],
  "index.load.context.currentPath.label": ["当前路径", "Current Path"],
  "index.load.context.sourceId.label": ["源 ID", "Source ID"],
  "index.load.context.kind.label": ["类型", "Kind"],
  "index.load.context.name.label": ["制品名", "Artifact Name"],
  "index.load.context.version.label": ["版本", "Version"],
  "index.load.context.taskId.label": ["任务 ID", "Task ID"],
  "index.load.context.selectedSource.label": ["当前选中源", "Selected Source"],
  "index.load.context.emailKey.label": ["邮件模板键", "Email Template Key"],
  "index.load.context.landingSlug.label": ["落地页 Slug", "Landing Page Slug"],
  "index.message.consoleLoaded": ["已加载市场工作台默认值和当前管理端页面上下文。", "Loaded market workspace defaults and the current admin page context."],
  "index.message.packageLoaded": ["已加载包操作上下文。", "Loaded package operation context."],
  "index.message.packageReset": ["已清空包相关筛选，并恢复到干净的包上下文。", "Cleared package filters and restored a clean package context."],
  "index.message.templateLoaded": ["已加载模板操作上下文。", "Loaded template operation context."],
  "index.message.templateReset": ["已清空模板相关筛选，并保留当前目标上下文。", "Cleared template filters while keeping the current target context."],
  "index.workspace.command.start": ["开始执行 {command}。", "Running {command}."],
  "index.workspace.overview.title": ["市场工作台概况", "Market Workspace Overview"],
  "index.workspace.overview.sourceCount.label": ["可信源数量", "Trusted Sources"],
  "index.workspace.overview.entryCount.label": ["缓冲区条目", "Buffer Entries"],
  "index.workspace.overview.bufferCapacity.label": ["缓冲区上限", "Buffer Capacity"],
  "index.workspace.overview.defaultChannel.label": ["默认渠道", "Default Channel"],
  "index.workspace.context.title": ["当前上下文", "Current Context"],
  "index.workspace.context.command.label": ["命令", "Command"],
  "index.workspace.context.defaultKind.label": ["默认类型", "Default Kind"],
  "index.workspace.context.pagePath.label": ["当前页面", "Page Path"],
  "index.workspace.context.currentAction.label": ["运行中动作", "Current Action"],
  "index.workspace.context.commandHint.title": ["命令提示", "Command Hint"],
  "index.workspace.context.allowedKinds.named": ["允许制品类型: {kinds}", "Allowed kinds: {kinds}"],
  "index.workspace.context.allowedKinds.all": ["允许制品类型：全部", "Allowed kinds: all"],
  "index.workspace.context.loadedNotice": ["已写入市场工作台上下文。", "Wrote market workspace context."],
  "index.workspace.context.bufferWritten": ["市场工作台上下文已写入缓冲区。", "Market workspace context written to the buffer."],
  "index.workspace.context.loadSourcesFailed": ["读取可信源列表失败：{error}", "Failed to load trusted sources: {error}"],
  "index.sourceSummary.sourceId.label": ["源 ID", "Source ID"],
  "index.sourceSummary.configuredSources.label": ["已配置源", "Configured Sources"],
  "index.sourceSummary.sourceName.label": ["源名称", "Source Name"],
  "index.sourceSummary.baseUrl.label": ["基础 URL", "Base URL"],
  "index.sourceSummary.defaultChannel.label": ["默认渠道", "Default Channel"],
  "index.sourceSummary.signature.label": ["签名", "Signing"],
  "index.sourceSummary.signature.supported": ["支持", "Supported"],
  "index.sourceSummary.signature.notExposed": ["未暴露", "Not Exposed"],
  "index.sourceSummary.enabledStatus.label": ["启用状态", "Enabled State"],
  "index.sourceSummary.enabledStatus.enabled": ["已启用", "Enabled"],
  "index.sourceSummary.enabledStatus.disabled": ["未启用", "Disabled"],
  "index.sourceSummary.allowedKinds.label": ["允许的类型", "Allowed Kinds"],
  "index.catalog.links.artifactContext.title": ["快速查看制品上下文", "Quick Artifact Context"],
  "index.catalog.links.latestPackageContext.title": ["快速进入最新版本上下文", "Quick Latest Release Context"],
  "index.catalog.links.latestTemplateContext.title": ["快速进入模板版本上下文", "Quick Template Release Context"],
  "index.catalog.links.installArtifact": ["安装 {release}", "Install {release}"],
  "index.catalog.links.importArtifact": ["导入 {release}", "Import {release}"],
  "index.catalog.links.openNativeInstallFlow": ["打开原生安装流程", "Open Native Install Flow"],
  "index.catalog.links.openNativeImportFlow": ["打开原生导入流程", "Open Native Import Flow"],
  "index.releaseLinks.downloadArtifact": ["下载制品", "Download Artifact"],
  "index.releaseLinks.openDocs": ["打开文档", "Open Docs"],
  "index.releaseLinks.openSupport": ["打开支持页", "Open Support Page"],
  "index.releaseLinks.openIcon": ["打开图标", "Open Icon"],
  "index.releaseLinks.screenshot": ["预览图 {index}", "Screenshot {index}"],
  "index.sourceDetail.title": ["源详情", "Source Detail"],
  "index.sourceDetail.content.disabled": ["当前选中的源在插件配置中已被禁用。", "The selected source is disabled in plugin config."],
  "index.sourceDetail.content.allowed": ["当前源已启用，并且允许当前所选市场类型。", "The selected source is enabled and allows the current market kind."],
  "index.sourceDetail.content.disallowed": ["当前源已启用，但暂不允许 {kind}。继续之前请先切换类型，或更新插件源配置。", "The selected source is enabled, but it does not allow {kind} yet. Switch kind or update the plugin source config before continuing."],
  "index.sourceDetail.summary.title": ["源配置摘要", "Source Config Summary"],
  "index.sourceDetail.summary.baseUrl.label": ["基础 URL", "Base URL"],
  "index.sourceDetail.summary.currentKind.label": ["当前类型", "Current Kind"],
  "index.sourceDetail.summary.kindAllowed.label": ["允许当前类型", "Allows Current Kind"],
  "index.sourceDetail.summary.enabled.label": ["已启用", "Enabled"],
  "index.sourceDetail.summary.supportsSignature.label": ["支持签名", "Supports Signature"],
  "index.sourceDetail.summary.currentChannel.label": ["当前渠道", "Current Channel"],
  "index.sourceDetail.summary.currentArtifact.label": ["当前制品", "Current Artifact"],
  "index.sourceDetail.summary.currentVersion.label": ["当前版本", "Current Version"],
  "index.sourceDetail.allowedKinds.title": ["允许的市场类型", "Allowed Market Kinds"],
  "index.sourceDetail.switchContext.title": ["切换类型上下文", "Switch Kind Context"],
  "index.sourceDetail.payload.title": ["原始源载荷", "Raw Source Payload"],
  "index.sourceDetail.payload.summary": ["Plugin.market.source.get() 返回的原始可信源配置。", "Raw trusted source config returned by Plugin.market.source.get()."],
  "index.message.sourceDetailLoaded": ["已加载源 {sourceId} 的详情。", "Loaded source detail for {sourceId}."],
  "index.artifactDetail.title": ["制品详情", "Artifact Detail"],
  "index.artifactDetail.content.noVersions": ["已加载制品详情，但当前源没有暴露版本列表。若你已经知道版本号，可以继续查看版本详情。", "Artifact detail loaded, but the current source does not expose a release list. If you already know the version, continue to release detail."],
  "index.artifactDetail.content.withVersions": ["已加载制品详情，共返回 {count} 条版本记录。若你要确认单个版本的宿主影响，请继续查看版本详情或直接预览。", "Artifact detail loaded with {count} release record(s). Inspect release detail or preview directly to confirm host impact for a specific release."],
  "index.artifactDetail.overview.title": ["制品概览", "Artifact Overview"],
  "index.artifactDetail.overview.kind.label": ["类型", "Kind"],
  "index.artifactDetail.overview.name.label": ["制品名", "Artifact Name"],
  "index.artifactDetail.overview.latestVersion.label": ["最新版本", "Latest Version"],
  "index.artifactDetail.overview.versionCount.label": ["版本数", "Releases"],
  "index.artifactDetail.overview.channelCount.label": ["渠道数", "Channels"],
  "index.artifactDetail.overview.governanceFields.label": ["治理字段", "Governance Fields"],
  "index.artifactDetail.metadata.title": ["制品元数据", "Artifact Metadata"],
  "index.artifactDetail.metadata.displayTitle.label": ["显示标题", "Display Title"],
  "index.artifactDetail.metadata.summary.label": ["摘要", "Summary"],
  "index.artifactDetail.metadata.description.label": ["描述", "Description"],
  "index.artifactDetail.metadata.governanceMode.label": ["治理模式", "Governance Mode"],
  "index.artifactDetail.metadata.installStrategy.label": ["安装策略", "Install Strategy"],
  "index.artifactDetail.metadata.supportsRollback.label": ["支持回滚", "Supports Rollback"],
  "index.artifactDetail.channels.title": ["可用渠道", "Available Channels"],
  "index.artifactDetail.versions.title": ["版本列表", "Release List"],
  "index.artifactDetail.versions.empty": ["当前制品没有暴露版本列表。", "This artifact does not expose a release list."],
  "index.artifactDetail.quickContext.title": ["快速切换到版本上下文", "Quick Release Context"],
  "index.common.nextSteps.title": ["下一步", "Next Steps"],
  "index.common.backToSourceContext": ["返回源上下文", "Back to Source Context"],
  "index.common.backToArtifactContext": ["返回制品上下文", "Back to Artifact Context"],
  "index.common.inspectRelease": ["查看 {release}", "Inspect {release}"],
  "index.common.openNativeImportPage": ["打开原生导入页", "Open Native Import Page"],
  "index.common.openNativeInstallPage": ["打开原生安装页", "Open Native Install Page"],
  "index.artifactDetail.payload.title": ["原始制品载荷", "Raw Artifact Payload"],
  "index.artifactDetail.payload.summary": ["Plugin.market.artifact.get() 返回的原始制品元数据。", "Raw artifact metadata returned by Plugin.market.artifact.get()."],
  "index.artifactDetail.failed": ["加载制品详情失败：{error}", "Failed to load artifact detail: {error}"],
  "index.message.artifactDetailLoaded": ["已加载制品 {name} 的详情。", "Loaded artifact detail for {name}."],
  "index.catalog.title": ["目录查询结果", "Catalog Results"],
  "index.catalog.content.empty": ["当前筛选条件下没有匹配到任何市场制品。可以尝试清空关键词、切换渠道，或调整源/类型上下文。", "No market artifacts matched the current filters. Try clearing keywords, switching channel, or adjusting source/kind context."],
  "index.catalog.content.named": ["已加载 {count} 个候选版本。可以直接用下方上下文快捷入口去预览或安装 {name}。", "Loaded {count} candidate release(s). Use the context shortcuts below to preview or install {name}."],
  "index.catalog.content.default": ["已加载目录结果。你可以直接用下方快捷入口把某个版本带入预览或安装上下文。", "Catalog results loaded. Use the shortcuts below to move a release directly into preview or install context."],
  "index.catalog.overview.title": ["目录概览", "Catalog Overview"],
  "index.catalog.overview.returnedCount.label": ["返回条数", "Returned Items"],
  "index.catalog.overview.totalCount.label": ["总数", "Total"],
  "index.catalog.overview.publisherCount.label": ["发布者数", "Publishers"],
  "index.catalog.overview.channelCount.label": ["渠道数", "Channels"],
  "index.catalog.overview.hasMore.label": ["还有更多", "Has More"],
  "index.catalog.overview.kind.label": ["类型", "Kind"],
  "index.catalog.overview.currentArtifact.label": ["当前制品", "Current Artifact"],
  "index.catalog.context.title": ["查询上下文", "Query Context"],
  "index.catalog.context.sourceBaseUrl.label": ["源基础 URL", "Source Base URL"],
  "index.catalog.context.channel.label": ["渠道", "Channel"],
  "index.catalog.context.keyword.label": ["关键词", "Keyword"],
  "index.catalog.channels.title": ["结果中的渠道", "Channels in Results"],
  "index.catalog.table.title": ["目录条目", "Catalog Items"],
  "index.catalog.table.empty": ["当前查询没有匹配到任何目录条目。", "No catalog items matched the current query."],
  "index.catalog.empty.action": ["先放宽当前目录筛选，再重新查询。", "Broaden the current catalog filter and query again first."],
  "index.catalog.empty.rationale.keyword": ["有关键词时最容易出现零结果，其次是渠道不匹配或选错了市场类型。", "Keywords are the most common reason for zero results, followed by channel mismatches or the wrong market kind."],
  "index.catalog.empty.rationale.default": ["零结果通常意味着当前源、渠道或制品类型下本来就没有可用条目。", "Zero results usually mean there are no available entries under the current source, channel, or artifact kind."],
  "index.catalog.empty.clearKeyword": ["清空关键词", "Clear Keyword"],
  "index.catalog.empty.clearArtifactVersion": ["清空制品与版本", "Clear Artifact and Version"],
  "index.catalog.empty.backToSource": ["返回源上下文", "Back to Source Context"],
  "index.releaseDetail.title": ["版本详情", "Release Detail"],
  "index.releaseDetail.content.channelMismatch": ["已加载版本元数据，但源返回的渠道是 {channel}，当前表单上下文是 {selectedChannel}。若要确认宿主兼容性和影响，请继续预览。", "Release metadata loaded, but the source returned channel {channel} while the current form context is {selectedChannel}. Continue to preview if you need to confirm host compatibility and impact."],
  "index.releaseDetail.content.default": ["已从可信源加载版本元数据。若要在安装或导入前确认宿主兼容性和目标影响，请继续预览。", "Release metadata loaded from a trusted source. Continue to preview if you need to confirm host compatibility and target impact before install or import."],
  "index.releaseDetail.overview.title": ["版本概览", "Release Overview"],
  "index.releaseDetail.overview.downloadSize.label": ["下载大小", "Download Size"],
  "index.releaseDetail.overview.requestedPermissions.label": ["申请权限数", "Requested Permissions"],
  "index.releaseDetail.overview.defaultGranted.label": ["默认授予数", "Default Grants"],
  "index.releaseDetail.overview.targetCount.label": ["目标数", "Targets"],
  "index.releaseDetail.overview.hostCompatibility.label": ["宿主适配", "Host Compatibility"],
  "index.releaseDetail.overview.hostCompatibility.ready": ["就绪", "Ready"],
  "index.releaseDetail.overview.hostCompatibility.rejected": ["已拒绝", "Rejected"],
  "index.releaseDetail.context.title": ["版本上下文", "Release Context"],
  "index.releaseDetail.context.publishedAt.label": ["发布时间", "Published At"],
  "index.releaseDetail.context.releaseNotes.label": ["更新说明", "Release Notes"],
  "index.releaseDetail.compatibility.title": ["宿主适配信息", "Host Compatibility"],
  "index.releaseDetail.compatibility.runtime.label": ["运行时", "Runtime"],
  "index.releaseDetail.compatibility.minBridge.label": ["最低宿主桥版本", "Min Host Bridge Version"],
  "index.releaseDetail.compatibility.packageFormat.label": ["包格式", "Package Format"],
  "index.releaseDetail.compatibility.entry.label": ["入口", "Entry"],
  "index.releaseDetail.compatibility.sha256.label": ["SHA256", "SHA256"],
  "index.releaseDetail.compatibility.signatureAlgorithm.label": ["签名算法", "Signature Algorithm"],
  "index.releaseDetail.permissions.requested.title": ["申请的权限", "Requested Permissions"],
  "index.releaseDetail.permissions.defaultGranted.title": ["默认授予权限", "Default Granted Permissions"],
  "index.releaseDetail.targets.title": ["声明的目标", "Declared Targets"],
  "index.releaseDetail.links.title": ["版本相关链接", "Release Links"],
  "index.releaseDetail.next.reuse": ["复用 {release}", "Reuse {release}"],
  "index.releaseDetail.payload.title": ["原始版本载荷", "Raw Release Payload"],
  "index.releaseDetail.payload.summary": ["Plugin.market.release.get() 返回的原始版本元数据。", "Raw release metadata returned by Plugin.market.release.get()."],
  "index.message.releaseDetailLoaded": ["已加载版本 {release} 的元数据。", "Loaded release metadata for {release}."],
  "index.message.pluginInstallSubmitted": ["已通过宿主桥接提交插件包安装。", "Submitted plugin package install through the host bridge."],
  "index.message.templateImported": ["已通过宿主桥接导入 {kind}。", "Imported {kind} through the host bridge."],
  "index.message.taskOnlyPlugin": ["只有插件包会创建异步宿主任务。", "Only plugin packages create async host tasks."],
  "index.message.taskListOnlyPlugin": ["只有插件包会创建宿主任务。", "Only plugin packages create host tasks."],
  "index.message.taskLoaded": ["已加载任务 {taskId}。", "Loaded task {taskId}."],
  "index.message.taskListLoaded": ["已加载 {count} 个宿主任务。", "Loaded {count} host task(s)."],
  "index.message.historyLoaded": ["已加载 {count} 条宿主安装历史。", "Loaded {count} host install history item(s)."],
  "index.task.none.title": ["没有宿主任务", "No Host Tasks"],
  "index.task.none.content.instant": ["payment_package 与所有宿主管理模板/页面规则制品都是即时执行，不会产生异步宿主任务。", "payment_package and all host-managed template/page-rule artifacts execute immediately and do not create async host tasks."],
  "index.task.none.content.pluginOnly": ["只有 plugin_package 导入会产生宿主管理的安装任务。", "Only plugin_package imports create host-managed install tasks."],
  "index.task.context.links.title": ["任务上下文快捷入口", "Task Context Shortcuts"],
  "index.task.status.title": ["任务状态", "Task Status"],
  "index.task.status.content.failed": ["宿主任务已进入失败状态。请先检查原始载荷，再结合安装历史决定是重试还是回滚。", "The host task entered a failed state. Inspect the raw payload first, then use install history to decide whether to retry or roll back."],
  "index.task.status.content.completed": ["宿主任务已成功结束。可以直接用下方快捷入口继续检查结果版本和历史记录。", "The host task completed successfully. Use the shortcuts below to inspect resulting releases and history."],
  "index.task.status.content.running": ["宿主任务仍在进行中。可以刷新当前任务，或返回任务列表观察阶段和进度变化。", "The host task is still running. Refresh this task or return to the task list to watch phase and progress changes."],
  "index.task.overview.title": ["任务概览", "Task Overview"],
  "index.task.overview.taskId.label": ["任务 ID", "Task ID"],
  "index.task.overview.status.label": ["状态", "Status"],
  "index.task.overview.phase.label": ["阶段", "Phase"],
  "index.task.overview.progress.label": ["进度", "Progress"],
  "index.task.overview.artifact.label": ["制品", "Artifact"],
  "index.task.overview.updatedAt.label": ["更新时间", "Updated At"],
  "index.task.coordinates.title": ["任务坐标", "Task Coordinates"],
  "index.task.coordinates.createdAt.label": ["创建时间", "Created At"],
  "index.task.payload.title": ["原始任务载荷", "Raw Task Payload"],
  "index.task.payload.summary": ["宿主返回的原始任务载荷，用于更细粒度的安装排查。", "Raw task payload returned by the host for deeper install diagnosis."],
  "index.task.guidance.failed.action": ["先检查失败版本上下文，再结合历史决定重试还是回滚。", "Inspect the failed release context first, then use history to decide whether to retry or roll back."],
  "index.task.guidance.failed.rationale": ["任务失败后，下一步通常就是判断要不要重试同一版本，或从历史执行回滚。", "After a task fails, the next step is usually deciding whether to retry the same release or roll back from history."],
  "index.task.guidance.completed.action": ["继续检查结果版本上下文和宿主历史。", "Continue by inspecting the resulting release context and host history."],
  "index.task.guidance.completed.rationale": ["任务已经结束，最有价值的后续动作是验证结果版本和当前激活历史记录。", "The task is finished, so the most valuable follow-up is validating the resulting release and the currently active history entry."],
  "index.task.guidance.running.action": ["稍后再次刷新这个任务，或切换到任务列表观察整体进度。", "Refresh this task again shortly, or switch to the task list to watch overall progress."],
  "index.task.guidance.running.rationale": ["任务还在运行，保持任务上下文固定是观察进度变化最直接的方法。", "The task is still running, so keeping the task context pinned is the most direct way to watch progress changes."],
  "index.task.guidance.openCurrent": ["打开当前任务上下文", "Open Current Task Context"],
  "index.taskList.title": ["任务列表", "Task List"],
  "index.taskList.content.empty": ["当前包上下文下没有匹配到任何宿主任务。可以先提交一次安装，或清空版本筛选以查看更早的执行记录。", "No host tasks matched the current package context. Submit an install first, or clear the version filter to inspect older executions."],
  "index.taskList.content.failed": ["已加载 {count} 个宿主任务，其中有 {failed} 个失败任务。建议先排查失败任务，再决定重试或回滚。", "Loaded {count} host task(s), including {failed} failed task(s). Inspect failed tasks first before deciding whether to retry or roll back."],
  "index.taskList.content.running": ["已加载 {count} 个宿主任务，其中 {running} 个仍在运行或等待中。", "Loaded {count} host task(s). {running} task(s) are still running or pending."],
  "index.taskList.content.default": ["已加载宿主任务。可以直接用下方快捷入口跳到最新任务或安装历史。", "Host tasks loaded. Use the shortcuts below to jump to the newest task or install history."],
  "index.taskList.stats.title": ["任务统计", "Task Stats"],
  "index.taskList.stats.count.label": ["条数", "Count"],
  "index.taskList.stats.total.label": ["总数", "Total"],
  "index.taskList.stats.running.label": ["运行中", "Running"],
  "index.taskList.stats.completed.label": ["已完成", "Completed"],
  "index.taskList.stats.failed.label": ["失败", "Failed"],
  "index.taskList.stats.artifactCount.label": ["制品数", "Artifacts"],
  "index.taskList.table.title": ["任务明细", "Task Details"],
  "index.taskList.table.empty": ["没有找到匹配的宿主安装任务。", "No matching host install tasks were found."],
  "index.taskList.guidance.failed.action": ["先检查失败任务，再决定重试还是回滚。", "Inspect failed tasks first, then decide whether to retry or roll back."],
  "index.taskList.guidance.failed.rationale": ["失败任务是列表里风险最高的项，通常需要优先人工介入。", "Failed tasks are the highest-risk items in the list and usually require manual attention first."],
  "index.taskList.guidance.running.action": ["优先检查最近仍在运行的任务，并保持版本上下文固定。", "Prioritize the most recent running task and keep the release context pinned."],
  "index.taskList.guidance.running.rationale": ["运行中的任务最能代表近期状态变化，尤其是在刚提交安装之后。", "A running task is the best indicator of near-term state changes, especially right after a submission."],
  "index.taskList.guidance.completed.action": ["检查最新成功版本上下文，或切换到历史记录。", "Inspect the latest successful release context, or switch to history."],
  "index.taskList.guidance.completed.rationale": ["可见任务都已结束时，版本上下文和历史记录会成为最有价值的验证面。", "When all visible tasks are settled, release context and history become the most useful validation surfaces."],
  "index.taskList.guidance.openFirstFailed": ["打开首个失败任务", "Open First Failed Task"],
  "index.taskList.guidance.openLatestRunning": ["打开最新运行中任务", "Open Latest Running Task"],
  "index.taskList.guidance.openLatestContext": ["打开最新任务上下文", "Open Latest Task Context"],
  "index.taskList.guidance.empty.action": ["先提交一次插件包安装，或放宽当前任务筛选。", "Submit a plugin package install first, or broaden the current task filter."],
  "index.taskList.guidance.empty.rationale.version": ["固定版本筛选后的任务查询通常会过窄，尤其是在刚切换上下文之后。", "Task queries filtered by a fixed version are often too narrow, especially right after switching context."],
  "index.taskList.guidance.empty.rationale.default": ["如果当前上下文里还没有任务，通常需要先提交一次安装，任务跟踪才会有意义。", "If there are no tasks in the current context yet, you usually need to submit an install before task tracking becomes meaningful."],
  "index.taskList.guidance.empty.clearTaskVersion": ["清空任务和版本筛选", "Clear Task and Version Filters"],
  "index.taskList.guidance.empty.backToPackage": ["返回包上下文", "Back to Package Context"],
  "index.history.title": ["安装历史", "Install History"],
  "index.history.content.empty": ["当前上下文下没有匹配到任何宿主管理安装历史。可以切换制品或版本筛选来查看更早记录。", "No host-managed install history matched the current context. Switch artifact or version filters to inspect older records."],
  "index.history.content.active": ["已加载 {count} 条历史记录，其中 {active} 条为当前激活版本。", "Loaded {count} history item(s), including {active} active version record(s)."],
  "index.history.content.noActive": ["已加载 {count} 条历史记录，但返回页里没有标记当前激活版本。建议放宽上下文，或返回任务结果继续确认。", "Loaded {count} history item(s), but no active version is marked in the returned page. Broaden the context or return to task results to confirm."],
  "index.history.stats.title": ["历史统计", "History Stats"],
  "index.history.stats.count.label": ["条数", "Count"],
  "index.history.stats.total.label": ["总数", "Total"],
  "index.history.stats.active.label": ["激活中", "Active"],
  "index.history.stats.artifactCount.label": ["制品数", "Artifacts"],
  "index.history.stats.versionCount.label": ["版本数", "Versions"],
  "index.history.stats.targetCount.label": ["目标数", "Targets"],
  "index.history.table.title": ["宿主安装历史", "Host Install History"],
  "index.history.table.empty": ["没有找到匹配的宿主安装历史。", "No matching host install history was found."],
  "index.history.guidance.active.action": ["先打开当前激活版本上下文，再对比页面里的其他旧版本。", "Open the current active release context first, then compare it with other older releases on the page."],
  "index.history.guidance.active.rationale": ["当前激活历史项通常是验证宿主当前实际生效版本的最快方式。", "The active history entry is usually the fastest way to validate what the host is currently serving."],
  "index.history.guidance.noActive.action": ["先打开最新可见历史项，确认预期版本是否已经激活。", "Open the newest visible history item first to confirm whether the expected release is already active."],
  "index.history.guidance.noActive.rationale": ["当前看不到激活标记时，下一步最有价值的动作是检查最新上下文，并决定是否需要放宽筛选。", "When no active marker is visible here, the next valuable step is to inspect the newest context and decide whether filters should be broadened."],
  "index.history.guidance.openActive": ["打开激活版本上下文", "Open Active Release Context"],
  "index.history.guidance.openLatest": ["打开最新历史上下文", "Open Latest History Context"],
  "index.history.guidance.empty.action": ["放宽历史筛选，或返回当前制品上下文。", "Broaden the history filter or return to the current artifact context."],
  "index.history.guidance.empty.rationale.version": ["固定版本筛选是预期历史项从当前页消失的最常见原因。", "A fixed version filter is the most common reason an expected history entry disappears from the current page."],
  "index.history.guidance.empty.rationale.template": ["模板历史还会按目标键做额外限定，所以当前选择的邮件模板键或落地页 slug 可能和你预期的版本不一致。", "Template history is additionally scoped by target key, so the currently selected email template key or landing-page slug may not match the release you expected."],
  "index.history.guidance.empty.rationale.default": ["当前上下文没有历史通常意味着安装尚未完成，或当前筛选条件过窄。", "No history in this context usually means the install never completed or the current filters are too narrow."],
  "index.history.guidance.empty.clearVersion": ["清空版本筛选", "Clear Version Filter"],
  "index.history.guidance.empty.clearArtifact": ["清空制品上下文", "Clear Artifact Context"],
  "index.history.links.releaseContext.title": ["快速进入版本上下文", "Quick Release Context"],
  "index.history.links.operationContext.title": ["快速进入操作上下文", "Quick Operation Context"],
  "index.message.pluginPreviewLoaded": ["已加载插件包安装预览。", "Loaded plugin package install preview."],
  "index.message.paymentPreviewLoaded": ["已加载支付包导入预览。", "Loaded payment package import preview."],
  "index.message.templatePreviewLoaded": ["已加载模板导入预览。", "Loaded template import preview."],
  "index.message.paymentImportFlow": ["支付包会通过原生支付方式导入器完成导入，便于在应用前确认目标和配置。", "Payment packages complete through the native payment importer so you can confirm target and config before applying."],
  "index.message.paymentRollbackExecuted": ["已执行支付包回滚。", "Executed payment package rollback."],
  "index.message.templateRollbackExecuted": ["已执行模板回滚。", "Executed template rollback."],
  "index.message.pluginRollbackSubmitted": ["已提交插件包回滚。", "Submitted plugin package rollback."],
  "index.preview.common.overview.title": ["预览概览", "Preview Overview"],
  "index.preview.common.currentVersion.label": ["当前版本", "Current Version"],
  "index.preview.common.warningCount.label": ["警告数", "Warnings"],
  "index.preview.common.hostCompatibility.label": ["兼容性", "Compatibility"],
  "index.preview.common.runtime.label": ["运行时", "Runtime"],
  "index.preview.common.requiredBridge.label": ["要求桥版本", "Required Bridge"],
  "index.preview.common.currentBridge.label": ["当前桥版本", "Current Bridge"],
  "index.preview.common.warningTitle": ["警告", "Warnings"],
  "index.preview.common.rawPayload.title": ["原始预览载荷", "Raw Preview Payload"],
  "index.preview.common.backToSource": ["返回源上下文", "Back to Source Context"],
  "index.preview.common.openAdminInstall": ["打开管理端安装页", "Open Admin Install Page"],
  "index.preview.common.openPaymentImport": ["打开支付导入页", "Open Payment Import Page"],
  "index.pluginPreview.title": ["插件包预览", "Plugin Package Preview"],
  "index.pluginPreview.content.incompatible": ["当前选择的版本与宿主桥接不兼容。", "The selected release is not compatible with the current host bridge."],
  "index.pluginPreview.content.newPermissions": ["宿主预览检查已通过，但这个版本会引入新的权限，安装前应先确认。", "Host preview checks passed, but this release introduces new permissions that should be reviewed before install."],
  "index.pluginPreview.content.upgrade": ["宿主预览检查已通过，这个版本会升级现有插件安装。", "Host preview checks passed. This release will upgrade the existing plugin install."],
  "index.pluginPreview.content.ready": ["宿主预览检查已通过，这个版本已经可以通过宿主桥接安装。", "Host preview checks passed. This release is ready to install through the host bridge."],
  "index.pluginPreview.overview.package.label": ["包", "Package"],
  "index.pluginPreview.overview.installMode.label": ["安装模式", "Install Mode"],
  "index.pluginPreview.overview.requestedPermissions.label": ["申请权限数", "Requested Permissions"],
  "index.pluginPreview.overview.newPermissions.label": ["新增权限数", "New Permissions"],
  "index.pluginPreview.context.title": ["版本上下文", "Release Context"],
  "index.pluginPreview.context.packageName.label": ["包名", "Package Name"],
  "index.pluginPreview.impact.title": ["安装影响", "Install Impact"],
  "index.pluginPreview.impact.installed.label": ["已安装", "Installed"],
  "index.pluginPreview.impact.updateAvailable.label": ["存在更新", "Update Available"],
  "index.pluginPreview.impact.installTarget.label": ["安装目标", "Install Target"],
  "index.pluginPreview.impact.targetId.label": ["目标 ID", "Target ID"],
  "index.pluginPreview.impact.compatibilityNotes.label": ["兼容性说明", "Compatibility Notes"],
  "index.pluginPreview.requestedPermissions.none": ["这个版本没有申请额外宿主权限。", "This release does not request additional host permissions."],
  "index.pluginPreview.newPermissions.title": ["新增权限", "New Permissions"],
  "index.pluginPreview.newPermissions.noneTitle": ["权限变化", "Permission Changes"],
  "index.pluginPreview.newPermissions.none": ["相对于当前已安装授权，没有新增权限。", "No new permissions are introduced relative to the current installed grants."],
  "index.pluginPreview.warnings.none": ["这个版本的宿主预览没有产生任何警告。", "This release produced no host-preview warnings."],
  "index.pluginPreview.guidance.incompatible.action": ["先查看上方版本详情，再切换到其他兼容版本。", "Inspect release detail above first, then switch to another compatible release."],
  "index.pluginPreview.guidance.incompatible.rationale": ["宿主桥接拒绝了当前版本，下一步最有价值的是对比原始版本元数据，或切换到其他版本。", "The host bridge rejected this release, so the next useful step is to compare raw release metadata or choose another version."],
  "index.pluginPreview.guidance.ready.action": ["如果当前预览确认无误，就直接使用上方安装 / 导入动作。", "If the preview looks correct, use the install or import action above directly."],
  "index.pluginPreview.guidance.ready.permissions": ["这个版本可以安装，但它会引入新的权限，提交前应先完成权限审查。", "This release is installable, but it introduces new permissions that should be reviewed before submission."],
  "index.pluginPreview.guidance.ready.upgrade": ["预览已经通过，这次操作会升级当前已安装的插件版本。", "The preview is ready, and this action will upgrade the currently installed plugin version."],
  "index.pluginPreview.guidance.ready.default": ["宿主预览已经通过，所以上方安装动作就是激活这个包的最快路径。", "The host preview is green, so the install action above is the fastest way to activate this package."],
  "index.pluginPreview.payload.summary": ["宿主返回的原始预览载荷，用于包兼容性和安装排查。", "Raw preview payload returned by the host for package compatibility and install diagnosis."],
  "index.paymentPreview.title": ["支付包预览", "Payment Package Preview"],
  "index.paymentPreview.content.incompatible": ["当前选择的支付包与宿主桥接不兼容。", "The selected payment package is not compatible with the current host bridge."],
  "index.paymentPreview.content.existing": ["支付包会通过原生支付导入器导入。当前已存在匹配的支付方式，建议先确认解析出的默认值，再决定是否覆盖。", "Payment packages import through the native payment importer. A matching payment method already exists, so verify the resolved defaults before deciding whether to overwrite it."],
  "index.paymentPreview.content.new": ["支付包会通过原生支付导入器导入。请打开管理端导入页选择目标支付方式并应用。", "Payment packages import through the native payment importer. Open the admin import page to choose the target payment method and apply it."],
  "index.paymentPreview.overview.scriptBytes.label": ["脚本字节数", "Script Bytes"],
  "index.paymentPreview.context.title": ["版本上下文", "Release Context"],
  "index.paymentPreview.context.packageName.label": ["包名", "Package Name"],
  "index.paymentPreview.defaults.title": ["解析出的导入默认值", "Resolved Import Defaults"],
  "index.paymentPreview.defaults.name.label": ["解析名称", "Resolved Name"],
  "index.paymentPreview.defaults.entry.label": ["解析入口", "Resolved Entry"],
  "index.paymentPreview.impact.title": ["目标影响", "Target Impact"],
  "index.paymentPreview.impact.icon.label": ["图标", "Icon"],
  "index.paymentPreview.impact.pollInterval.label": ["轮询间隔", "Poll Interval"],
  "index.paymentPreview.impact.scriptBytes.label": ["脚本字节数", "Script Bytes"],
  "index.paymentPreview.impact.checksum.label": ["校验和", "Checksum"],
  "index.paymentPreview.impact.importTarget.label": ["导入目标", "Import Target"],
  "index.paymentPreview.impact.targetId.label": ["目标 ID", "Target ID"],
  "index.paymentPreview.warnings.none": ["这个支付包的宿主预览没有产生任何警告。", "This payment package produced no host-preview warnings."],
  "index.paymentPreview.guidance.incompatible.action": ["先查看上方版本详情，再选择其他兼容的支付包版本。", "Inspect release detail above first, then choose another compatible payment-package release."],
  "index.paymentPreview.guidance.incompatible.rationale": ["当前宿主桥无法直接接受这个支付包。", "The current host bridge cannot accept this payment package as-is."],
  "index.paymentPreview.guidance.ready.action": ["直接从当前预览打开原生支付导入页。", "Open the native payment import page directly from this preview."],
  "index.paymentPreview.guidance.ready.existing": ["已经存在匹配的支付方式，因此应在原生导入器里确认默认值并决定是否覆盖。", "A matching payment method already exists, so the native importer is the right place to review defaults and decide whether to overwrite it."],
  "index.paymentPreview.guidance.ready.default": ["支付包要在原生导入器里完成目标选择和最终配置确认。", "Payment packages finish in the native importer because target selection and final config confirmation happen there."],
  "index.paymentPreview.payload.summary": ["支付包导入预览返回的原始载荷，用于桥接排查和字段确认。", "Raw payload returned by payment-package import preview for bridge diagnosis and field confirmation."],
  "index.templatePreview.title": ["模板预览", "Template Preview"],
  "index.templatePreview.content.incompatible": ["当前选择的模板版本与宿主桥接不兼容。", "The selected template release is not compatible with the current host bridge."],
  "index.templatePreview.content.createFirst": ["模板预览检查已通过。原生目标还不存在，因此这次导入会为当前目标创建第一条受管版本。", "Template preview checks passed. The native target does not exist yet, so this import will create the first managed revision for the current target."],
  "index.templatePreview.content.noDiff": ["当前模板版本与原生目标内容摘要一致；如果你希望把这个版本纳入宿主管理历史，仍然可以继续导入。", "The selected template release matches the current native content digest. Import is still available if you want this release tracked in host-managed history."],
  "index.templatePreview.content.update": ["模板预览检查已通过，这次导入会更新当前原生目标内容。", "Template preview checks passed. This import will update the current native target content."],
  "index.templatePreview.overview.targetExists.label": ["目标已存在", "Target Exists"],
  "index.templatePreview.overview.hostManaged.label": ["宿主已接管", "Host Managed"],
  "index.templatePreview.overview.contentBytes.label": ["内容大小", "Content Bytes"],
  "index.templatePreview.context.title": ["版本上下文", "Release Context"],
  "index.templatePreview.context.targetKey.label": ["目标键", "Target Key"],
  "index.templatePreview.context.hostCompatibility.label": ["宿主兼容性", "Host Compatibility"],
  "index.templatePreview.diff.title": ["内容对比", "Content Diff"],
  "index.templatePreview.diff.newDigest.label": ["新内容摘要", "New Digest"],
  "index.templatePreview.diff.currentDigest.label": ["当前摘要", "Current Digest"],
  "index.templatePreview.diff.currentBytes.label": ["当前字节数", "Current Bytes"],
  "index.templatePreview.diff.changeSummary.label": ["变化摘要", "Change Summary"],
  "index.templatePreview.diff.updateAvailable.label": ["存在更新", "Update Available"],
  "index.templatePreview.warnings.none": ["这个模板版本的宿主预览没有产生任何警告。", "This template release produced no host-preview warnings."],
  "index.templatePreview.guidance.incompatible.action": ["先查看上方版本详情，再切换到其他兼容模板版本。", "Inspect release detail above first, then switch to another compatible template release."],
  "index.templatePreview.guidance.incompatible.rationale": ["宿主在导入前就拒绝了当前版本，因此下一步最有价值的是更换版本或目标上下文。", "The host rejected this release before import, so changing version or target context is the next useful step."],
  "index.templatePreview.guidance.ready.action": ["确认目标键和内容差异无误后，直接使用上方模板导入动作。", "Once the target key and content diff look correct, use the template import action above directly."],
  "index.templatePreview.guidance.ready.createFirst": ["这次导入会为当前目标创建第一条受管修订，因此提交前应先确认目标键。", "This import will create the first managed revision for the current target, so verify the target key before submission."],
  "index.templatePreview.guidance.ready.noDiff": ["当前内容和目标一致，因此只有在你希望把这个版本纳入宿主管理历史时才值得导入。", "The content already matches the current target, so importing is only useful if you want this release captured in host-managed history."],
  "index.templatePreview.guidance.ready.update": ["预览显示当前目标会发生真实内容变化，因此现在导入会更新受管版本历史。", "The preview shows a real content change against the current target, so importing now will update managed revision history."],
  "index.templatePreview.payload.summary": ["模板预览返回的原始载荷，用于目标解析和差异排查。", "Raw payload returned by template preview for target resolution and diff diagnosis."],
  "index.templateInstall.title": ["模板导入结果", "Template Import Result"],
  "index.templateInstall.content.status": ["宿主导入状态：{status}。", "Host import status: {status}."],
  "index.templateInstall.result.title": ["导入结果", "Import Result"],
  "index.templateInstall.result.importVersion.label": ["导入版本", "Imported Version"],
  "index.templateInstall.result.historyId.label": ["历史记录 ID", "History Entry ID"],
  "index.templateInstall.result.contentDigest.label": ["内容摘要", "Content Digest"],
  "index.templateInstall.payload.title": ["原始导入载荷", "Raw Import Payload"],
  "index.templateInstall.payload.summary": ["宿主桥接返回的原始模板导入结果。", "Raw template import result returned by the host bridge."],
  "index.templateInstall.guidance.action": ["先检查导入后的版本上下文，再回看宿主管理历史。", "Inspect the imported release context first, then review host-managed history."],
  "index.templateInstall.guidance.rationale": ["宿主已经写入了这次模板修订，下一步最有价值的是确认当前激活目标状态，并把回滚点保留在视野内。", "The host has written this template revision, so the next valuable step is to confirm the active target state and keep rollback points visible."],
  "index.templateInstall.guidance.openImported": ["打开导入后的版本上下文", "Open Imported Release Context"],
  "index.pluginInstall.title": ["插件包安装结果", "Plugin Package Install Result"],
  "index.pluginInstall.content.status": ["宿主安装状态：{status}。", "Host install status: {status}."],
  "index.pluginInstall.result.title": ["安装结果", "Install Result"],
  "index.pluginInstall.result.activateRequested.label": ["请求激活", "Activate Requested"],
  "index.pluginInstall.result.autoStart.label": ["自动启动", "Auto Start"],
  "index.pluginInstall.payload.title": ["原始安装载荷", "Raw Install Payload"],
  "index.pluginInstall.payload.summary": ["宿主桥接返回的原始插件安装结果。", "Raw plugin install result returned by the host bridge."],
  "index.pluginInstall.guidance.withTask.action": ["持续检查任务 {taskId}，直到它进入终态。", "Keep checking task {taskId} until it reaches a terminal state."],
  "index.pluginInstall.guidance.withTask.rationale": ["插件包安装通常会异步继续执行，所以任务跟踪是确认激活结果或失败细节的最快方式。", "Plugin-package install usually continues asynchronously, so task tracking is the fastest way to confirm activation results or failure details."],
  "index.pluginInstall.guidance.noTask.action": ["检查已安装版本上下文和宿主历史。", "Inspect the installed release context and host history."],
  "index.pluginInstall.guidance.noTask.rationale": ["当前没有返回任务 ID，因此已安装版本和历史视图就是验证结果的最佳位置。", "No task ID was returned, so installed release context and history are the best places to validate the outcome."],
  "index.pluginInstall.guidance.openInstalled": ["打开已安装版本上下文", "Open Installed Release Context"],
  "index.rollback.title.payment": ["支付包回滚", "Payment Package Rollback"],
  "index.rollback.title.template": ["模板回滚", "Template Rollback"],
  "index.rollback.title.plugin": ["插件包回滚", "Plugin Package Rollback"],
  "index.rollback.content.status": ["回滚状态：{status}。", "Rollback status: {status}."],
  "index.rollback.result.title": ["回滚结果", "Rollback Result"],
  "index.rollback.result.coordinates.label": ["坐标", "Coordinates"],
  "index.rollback.result.targetKey.label": ["目标键", "Target Key"],
  "index.rollback.result.error.label": ["错误", "Error"],
  "index.rollback.payload.title": ["原始回滚载荷", "Raw Rollback Payload"],
  "index.rollback.payload.summary": ["宿主桥接返回的原始回滚结果。", "Raw rollback result returned by the host bridge."],
  "index.rollback.guidance.template.action": ["检查已恢复模板版本，并确认目标状态。", "Inspect the restored template release and confirm target state."],
  "index.rollback.guidance.template.rationale": ["模板回滚是即时完成的，所以下一步最有价值的是确认当前目标内容和历史位置。", "Template rollback completes immediately, so the next valuable step is confirming current target content and history position."],
  "index.rollback.guidance.package.action": ["检查已恢复的包上下文，并确认当前激活历史或任务状态。", "Inspect the restored package context and confirm active history or task state."],
  "index.rollback.guidance.package.withTask": ["包回滚可能还会继续经过宿主编排，所以继续操作前应先验证任务状态或已恢复版本上下文。", "Package rollback may still continue through host orchestration, so verify task state or restored release context before doing anything else."],
  "index.rollback.guidance.package.noTask": ["回滚结果现在已经可用，建议先确认恢复后的版本，并保留历史视图以便后续再次回滚。", "The rollback result is already available. Confirm the restored release first and keep history visible for any later rollback."],
  "index.rollback.guidance.openRestoredTemplate": ["打开已恢复版本上下文", "Open Restored Release Context"],
  "index.rollback.guidance.openRestoredPackage": ["打开已恢复包上下文", "Open Restored Package Context"],
  "index.templateTarget.snapshot.title": ["原生目标快照", "Native Target Snapshot"],
  "index.templateTarget.snapshot.fileOrSlug.label": ["文件名 / Slug", "Filename / Slug"],
  "index.templateTarget.snapshot.digest.label": ["摘要", "Digest"],
  "index.templateTarget.snapshot.updatedAt.label": ["更新时间", "Updated At"],
  "index.templateTarget.snapshot.nativeBytes.label": ["原生字节数", "Native Bytes"],
  "index.templateTarget.snapshot.contentSnippet.title": ["原生内容片段", "Native Content Snippet"],
  "index.templateTarget.snapshot.failed": ["加载原生目标快照失败：{error}", "Failed to load native target snapshot: {error}"],
  "index.templateTarget.status.title": ["宿主管理模板状态", "Host Managed Template State"],
  "index.templateTarget.status.historyCount.label": ["历史条数", "History Items"],
  "index.templateTarget.status.activeVersion.label": ["当前激活版本", "Active Version"],
  "index.templateTarget.status.activeArtifact.label": ["当前激活制品", "Active Artifact"],
  "index.templateTarget.recentVersions.title": ["最近目标版本", "Recent Target Versions"],
  "index.templateTarget.recentVersions.empty": ["当前目标下没有找到宿主管理模板历史。", "No host-managed template history was found under the current target."],
  "index.templateTarget.recentVersions.activeSuffix": ["{release}（当前激活）", "{release} (active)"],
  "index.templateTarget.history.failed": ["加载当前目标的模板历史失败：{error}", "Failed to load template history for the current target: {error}"],
  "index.emailTargets.title": ["原生邮件模板目标", "Native Email Template Targets"],
  "index.emailTargets.empty": ["Plugin.emailTemplate.list() 没有返回任何原生邮件模板。", "Plugin.emailTemplate.list() returned no native email templates."],
  "index.emailTargets.failed": ["加载邮件模板目标失败：{error}", "Failed to load email template targets: {error}"]
} as const;

export type MarketMessageKey = keyof typeof KEYED_MESSAGES;
type MarketMessageValue = string | number | boolean;
type MarketMessageVars = Record<string, MarketMessageValue>;

export function marketMessage(key: MarketMessageKey, vars: MarketMessageVars = {}): string {
  return `${MARKET_I18N_TOKEN_PREFIX}${encodeURIComponent(JSON.stringify({ key, vars }))}`;
}

function isMarketMessageKey(value: unknown): value is MarketMessageKey {
  return typeof value === "string" && Object.prototype.hasOwnProperty.call(KEYED_MESSAGES, value);
}

function parseMarketMessageToken(
  value: string
): { key: MarketMessageKey; vars: MarketMessageVars } | null {
  if (!value.startsWith(MARKET_I18N_TOKEN_PREFIX)) {
    return null;
  }
  try {
    const parsed = JSON.parse(decodeURIComponent(value.slice(MARKET_I18N_TOKEN_PREFIX.length)));
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return null;
    }
    const key = (parsed as { key?: unknown }).key;
    if (!isMarketMessageKey(key)) {
      return null;
    }
    const vars: MarketMessageVars = {};
    const rawVars = (parsed as { vars?: unknown }).vars;
    if (rawVars && typeof rawVars === "object" && !Array.isArray(rawVars)) {
      Object.entries(rawVars as Record<string, unknown>).forEach(([name, candidate]) => {
        if (
          typeof candidate === "string" ||
          typeof candidate === "number" ||
          typeof candidate === "boolean"
        ) {
          vars[name] = candidate;
        }
      });
    }
    return { key, vars };
  } catch {
    return null;
  }
}

function interpolateMarketMessage(
  locale: MarketLocale,
  template: string,
  vars: MarketMessageVars
): string {
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(vars, name)
      ? typeof vars[name] === "string"
        ? translateMarketText(locale, vars[name])
        : String(vars[name])
      : match
  );
}

function translateKeyed(locale: MarketLocale, value: string): string | null {
  const token = parseMarketMessageToken(value);
  if (!token) {
    return null;
  }
  const pair = KEYED_MESSAGES[token.key];
  const message = locale === "zh" ? pair[0] : pair[1];
  return interpolateMarketMessage(locale, message, token.vars);
}

function translateEmbeddedMarketTokens(locale: MarketLocale, value: string): string {
  const tokenPattern = /__market_i18n__:[A-Za-z0-9%_.!~*'()-]+/g;
  let replaced = false;
  const output = value.replace(tokenPattern, (token) => {
    const translated = translateKeyed(locale, token);
    if (translated === null) {
      return token;
    }
    replaced = true;
    return translated;
  });
  return replaced ? output : value;
}

const PHRASE_PAIRS = [
  ["AuraLogic 市场", "AuraLogic Market"],
  ["包市场", "Package Market"],
  ["模板导入", "Template Import"],
  ["刷新包上下文", "Refresh Package Context"],
  ["刷新模板上下文", "Refresh Template Context"],
  ["快速版本上下文", "Quick Version Context"],
  ["已配置源信息", "Configured Source"],
  ["已配置源数", "Configured Sources"],
  ["制品摘要", "Artifact Summary"],
  ["制品渠道", "Artifact Channels"],
  ["制品版本", "Artifact Versions"],
  ["当前生效制品", "Active Artifact"],
  ["当前生效版本", "Active Version"],
  ["治理字段数", "Governance Fields"],
  ["下载字节数", "Download Bytes"],
  ["所需桥版本", "Required Bridge"],
  ["最低宿主桥版本", "Min Host Bridge"],
  ["签名", "Signature"],
  ["回滚", "Rollback"],
  ["结果渠道", "Result Channels"],
  ["搜索词", "Search"],
  ["当前选择源", "Selected Source"],
  ["当前选择类型", "Selected Kind"],
  ["当前选择渠道", "Selected Channel"],
  ["当前选择名称", "Selected Name"],
  ["当前源摘要", "Selected Source Summary"],
  ["当前制品", "Focused Artifact"],
  ["当前版本", "Focused Version"],
  ["原生邮件目标", "Native Email Targets"],
  ["包上下文链接", "Package Context Links"],
  ["模板上下文链接", "Template Context Links"],
  ["使用任务上下文", "Use Task Context"],
  ["使用制品版本上下文", "Use Artifact Version Context"],
  ["上下文快捷入口", "Context Shortcuts"],
  ["打开插件包列表", "Open Plugin Packages"],
  ["打开支付包列表", "Open Payment Packages"],
  ["打开邮件模板列表", "Open Email Templates"],
  ["打开落地页列表", "Open Landing Pages"],
  ["查看版本上下文", "Inspect Release Context"],
  ["重置包上下文", "Reset Package Context"],
  ["重置模板上下文", "Reset Template Context"],
  ["请求权限数", "Requested Perms"],
  ["新增权限数", "New Perms"],
  ["权限差异", "Permission Delta"],
  ["可更新", "Update Available"],
  ["安装目标", "Installed Target"],
  ["当前宿主桥版本", "Current Bridge"],
  ["已解析导入默认值", "Resolved Import Defaults"],
  ["已解析名称", "Resolved Name"],
  ["已解析入口", "Resolved Entry"],
  ["受宿主管理", "Host Managed"],
  ["内容大小", "Content Size"],
  ["传入摘要", "Incoming Digest"],
  ["变更摘要", "Change Summary"],
  ["已导入版本", "Imported Version"],
  ["任务", "Tasks"],
  ["打开已导入版本上下文", "Open Imported Version Context"],
  ["打开包上下文", "Open Package Context"],
  ["清除版本筛选", "Clear Version Filter"],
  ["重置到当前制品上下文", "Reset To Current Artifact Context"],
  ["返回项数", "Items Returned"],
  ["发布说明", "Release Notes"],
  ["版本链接", "Release Links"],
  ["版本摘要", "Release Summary"],
  ["类型允许", "Kind Allowed"],
  ["历史记录数", "History Entries"],
  ["无宿主任务", "No Host Task"],
  ["无宿主任务列表", "No Host Tasks"],
  ["已解析上下文", "Resolved Context"],
  ["源负载", "Source Payload"],
  ["制品负载", "Artifact Payload"],
  ["版本负载", "Release Payload"],
  ["预览负载", "Preview Payload"],
  ["宿主安装负载", "Host Install Payload"],
  ["宿主导入负载", "Host Import Payload"],
  ["回滚负载", "Rollback Payload"],
  ["任务负载", "Task Payload"],
  ["落地页标识", "Landing Slug"],
  ["打开活动版本上下文", "Open Active Version Context"],
  ["打开已安装版本上下文", "Open Installed Version Context"],
  ["打开最新运行任务", "Open Latest Running Task"],
  ["打开原生导入器", "Open Native Importer"],
  ["打开原生安装器", "Open Native Installer"],
  ["打开已恢复版本上下文", "Open Restored Version Context"],
  ["打开支持链接", "Open Support"],
  ["使用操作上下文", "Use Action Context"],
  ["使用最新版本上下文", "Use Latest Release Context"],
  ["使用模板版本上下文", "Use Template Release Context"],
  ["使用版本上下文", "Use Version Context"],
  ["切换到邮件模板", "Switch To Email Template"],
  ["切换到落地页", "Switch To Landing Page"],
  ["切换到支付包", "Switch To Payment Package"],
  ["切换到插件包", "Switch To Plugin Package"],
  ["任务摘要", "Task Summary"],
  ["预览摘要", "Preview Summary"],
  ["内容对比", "Content Comparison"],
  ["宿主适配", "Host Fit"],
  ["允许类型", "Allowed Kinds"],
  ["查看源上下文", "Inspect Source Context"],
  ["查看制品上下文", "Inspect Artifact Context"],
  ["打开管理端安装器", "Open Admin Installer"],
  ["打开支付导入器", "Open Payment Importer"],
  ["清除搜索", "Clear Search"],
  ["重置版本上下文", "Reset Version Context"],
  ["重置任务筛选", "Reset Task Filter"],
  ["重置制品上下文", "Reset Artifact Context"],
  ["目录结果", "Catalog Result"],
  ["目录摘要", "Catalog Summary"],
  ["目录项", "Catalog Items"],
  ["可信源", "Trusted Sources"],
  ["推荐操作", "Recommended Action"],
  ["主操作", "Primary Action"],
  ["为什么现在", "Why Now"],
  ["请求权限", "Requested Permissions"],
  ["声明目标", "Declared Targets"],
  ["名称", "Name"],
  ["概要", "Summary"],
  ["标题", "Title"],
  ["数量", "Items"],
  ["活动中", "Active"],
  ["是", "Yes"],
  ["否", "No"],
  ["name", "Name"],
  ["version", "Version"],
  ["source_id", "Source ID"],
  ["kind", "Kind"],
  ["task_id", "Task ID"],
  ["status", "Status"],
  ["phase", "Phase"],
  ["progress", "Progress"],
  ["created_at", "Created At"],
  ["updated_at", "Updated At"],
  ["target_key", "Target Key"],
  ["is_active", "Active"],
  ["installed_at", "Installed At"]
] as const;

const EXACT_TEXTS = [
  [
    "浏览可信市场源，查询插件包与支付包，导入邮件模板和落地页模板，并跟踪宿主安装任务与回滚记录。",
    "Browse trusted market sources, inspect plugin and payment packages, import email or landing-page templates, and track host install tasks and rollback history."
  ],
  [
    "市场插件只会访问插件配置中声明的可信源。插件包通过宿主桥接安装，支付包会先进入统一支付包导入流程确认目标与配置，邮件模板和落地页模板则写入宿主管理的模板历史。",
    "The market plugin only connects to trusted sources declared in the plugin config. Plugin packages install through the host bridge, payment packages continue in the unified payment import flow, and email or landing templates are written into host-managed template history."
  ],
  ["将市场模板导入到系统已有的落地页 slug。", "Import a market template into an existing system landing-page slug."],
  ["用于回顾导入历史、版本预览与回滚。", "Review import history, preview versions, and run rollbacks."],
  ["留空时由市场插件解析最新版本。", "Leave empty to let the market plugin resolve the latest version."],
  ["仅插件包有效。传入 host.market.install.execute 的权限数组。", "Plugin packages only. Pass the granted permission list into host.market.install.execute."],
  ["已加载市场控制台默认值和当前管理页上下文。", "Loaded market console defaults and current admin-page context."],
  ["已通过宿主桥接提交插件包安装。", "Submitted plugin package installation through the host bridge."],
  ["支付包会通过原生支付方式导入器进入导入流程，以便在应用前确认目标配置。", "Payment packages import through the native payment-method importer so target settings can be confirmed before apply."],
  ["已经存在匹配的支付方式，因此原生导入器才是检查默认值并决定是否覆盖的正确位置。", "A matching payment method already exists, so the native importer is the right place to review defaults and decide whether to overwrite."],
  ["支付包与所有宿主管理模板/页面规则制品操作都会立即执行，不会创建异步宿主任务。", "payment_package and all host-managed template/page-rule actions execute immediately and do not create async host tasks."],
  ["只有插件包导入会创建宿主管理安装任务。", "Only plugin_package imports create host-managed install tasks."],
  ["模板导入、历史查询和回滚都会立即通过宿主管理的模板历史执行。", "Templates import, history, and rollback all execute immediately through host-managed template history."],
  ["已加载目录结果。可复用下方上下文链接，直接跳到某个版本的预览或安装上下文。", "Loaded catalog results. Reuse the context links below to jump a release directly into preview or install context."],
  ["已加载宿主任务。可复用下方上下文链接，直接跳到最新任务或已安装版本历史。", "Loaded host tasks. Reuse the context links below to jump into the latest task or installed version history."],
  ["可信源返回了原始版本元数据。需要安装或导入前的宿主兼容性与目标影响检查时，请使用预览。", "Loaded raw release metadata from the trusted source. Use preview when you need host compatibility and target impact checks before install or import."],
  ["当前查询没有匹配的目录项。", "No catalog items matched the current query."],
  ["市场插件当前没有可用的可信源。", "No trusted sources are available for the market plugin."],
  ["当前没有选择制品，先通过目录搜索收敛到一个包，再做预览或安装会更快。", "No artifact is selected yet, so catalog search is the quickest way to narrow to a package before preview or install."],
  ["当前包上下文没有匹配的宿主任务。先提交一次安装，或清除版本筛选后再查看更早的运行记录。", "No host tasks matched the current package context. Submit an install first or clear the version filter to inspect older runs."],
  ["当前上下文没有匹配的宿主管理安装历史。请调整制品或版本筛选以查看更早的记录。", "No host-managed install history matched the current context. Change the artifact or version filter to inspect older records."],
  ["当前筛选条件下没有匹配的市场制品。请清除搜索词、调整渠道，或切换源 / 类型上下文。", "No market artifacts matched the current filters. Clear the search term, change the channel, or switch the source/kind context."],
  ["Plugin.emailTemplate.list() 没有返回任何原生邮件模板。", "No native email templates were returned by Plugin.emailTemplate.list()."],
  ["当前配置里未找到该项。", "Not found in current config"],
  ["需要制品名称", "Artifact name is required"],
  ["无法解析制品版本", "Artifact version could not be resolved"],
  ["Plugin.market 不可用", "Plugin.market is unavailable"],
  ["Plugin.emailTemplate 不可用", "Plugin.emailTemplate is unavailable"],
  ["Plugin.landingPage 不可用", "Plugin.landingPage is unavailable"],
  ["发生未知错误", "unknown error"],
  ["先查询模板目录。", "Query the template catalog next."],
  ["先配置至少一个可信源，再使用这个市场控制台。", "Configure at least one trusted source in the market plugin config before using this console."],
  ["先选择包模式或模板模式，再从目录查询或源详情开始。", "Choose package or template mode, then start from catalog or source detail."],
  ["先设置原生目标键，然后再查询目录或预览某个版本。", "Set the native target key first, then query catalog or preview a release."],
  ["先放宽当前目录筛选条件，再重新查询。", "Relax the current catalog filters, then query again."],
  ["先把 source_id 切换到受信任配置中的某一个，再刷新当前上下文。", "Switch source_id to one of the configured trusted sources, then refresh the current context."],
  ["打开活动版本上下文，然后对比当前视图中的旧版本。", "Open the active version context first, then compare any older versions in view."],
  ["打开当前预览对应的原生支付导入器。", "Open the native payment importer from this preview."],
  ["打开最新可见的历史项，确认期望的版本是否处于活动状态。", "Open the newest visible history item and confirm whether the expected version is active."],
  ["优先查看失败任务，再决定是重试还是回滚。", "Inspect a failed task first, then decide whether to retry or roll back."],
  ["查看制品详情，或者直接预览最新模板版本。", "Inspect artifact detail or preview the latest template version."],
  ["查看制品详情，或者下一步预览最新版本。", "Inspect artifact detail or preview the latest version next."],
  ["查看失败版本上下文，然后在重试或回滚前先看历史。", "Inspect the failed version context, then review history before retrying or rolling back."],
  ["查看已导入版本上下文，然后再查看宿主管理历史。", "Inspect the imported version context and then review host-managed history."],
  ["查看已安装版本上下文和宿主历史。", "Inspect the installed version context and host history."],
  ["查看最新成功版本上下文，或切换到历史。", "Inspect the latest successful version context or switch to history."],
  ["查看最近的运行中任务，并保持版本上下文固定。", "Inspect the most recent running task and keep the version context pinned."],
  ["查看已恢复的包上下文，并确认活动历史或任务状态。", "Inspect the restored package context and confirm active history or task status."],
  ["查看已恢复的模板版本，并确认目标状态。", "Inspect the restored template version and confirm the target state."],
  ["查看最终版本上下文和宿主历史。", "Inspect the resulting version context and host history."],
  ["没有任务 ID 返回，所以最适合验证结果的地方是已安装版本和历史视图。", "No task id was returned, so the installed version and history views are the best places to verify the outcome."],
  ["没有活动记录显示在这里，下一步最好查看最新上下文，并判断是否需要放宽筛选。", "No active record is visible here, so the next useful step is to inspect the newest context and decide whether filters need to be broadened."],
  ["当前上下文没有历史，通常意味着安装未完成，或者当前筛选过窄。", "No history in this context usually means either the install never completed or the current filters are too narrow."],
  ["支付包会通过原生导入器完成，因为目标选择和最终配置确认都在那里发生。", "Payment packages complete through the native importer because target selection and final config confirmation happen there."],
  ["支付包通过原生支付导入器导入。已存在匹配的支付方式时，应先检查已解析的默认值再决定是否覆盖。", "Payment packages import through the native payment importer. A matching payment method already exists, so review the resolved defaults before applying."],
  ["支付包通过原生支付导入器导入。请打开管理端导入器选择目标支付方式并应用该包。", "Payment packages import through the native payment importer. Open the admin importer to choose the target method and apply the package."],
  ["支付包最终仍然通过原生导入器完成，但预览阶段最适合先确认桥接兼容性和已解析默认值。", "Payment packages still complete through the native importer, but preview is the right place to confirm bridge compatibility and resolved defaults first."],
  ["支付包使用宿主导入桥接，不会创建异步任务。", "Payment packages use the host import bridge and do not create async tasks."],
  ["插件包安装通常会异步继续，所以任务跟踪是观察激活或失败细节的最快方式。", "Plugin package installs usually continue asynchronously, so task tracking is the fastest way to see activation or failure details."],
  ["预览确认兼容性和权限影响后，可以直接在此页提交插件包。", "Plugin packages can be submitted directly from this page after preview confirms compatibility and permission impact."],
  ["插件包使用宿主安装桥接，并且可能创建异步安装任务。", "Plugin packages use the host install bridge and can create async install tasks."],
  ["原始制品元数据，来自 Plugin.market.artifact.get()。", "Raw artifact metadata returned by Plugin.market.artifact.get()."],
  ["原始宿主预览负载，用于排查包兼容性和安装器问题。", "Raw host preview payload for package compatibility and installer troubleshooting."],
  ["原始宿主任务负载，用于底层安装诊断。", "Raw host task payload for low-level install diagnostics."],
  ["原始支付导入预览负载，用于桥接调试和字段核对。", "Raw payment import preview payload for bridge debugging and field verification."],
  ["宿主桥接返回的原始插件安装负载。", "Raw plugin install payload returned by the host bridge."],
  ["原始版本元数据，来自 Plugin.market.release.get()。", "Raw release metadata returned by Plugin.market.release.get()."],
  ["宿主桥接返回的原始回滚负载。", "Raw rollback payload returned by the host bridge."],
  ["宿主桥接返回的原始模板导入负载。", "Raw template import payload returned by the host bridge."],
  ["原始模板预览负载，用于排查目标解析和差异对比。", "Raw template preview payload for target resolution and diff troubleshooting."],
  ["原始可信源负载，来自 Plugin.market.source.get()。", "Raw trusted-source payload returned by Plugin.market.source.get()."],
  ["失败任务通常意味着下一步要在重试同一版本和从历史回滚之间做决定。", "A failed task usually means the next decision is whether to retry the same release or roll back from history."],
  ["固定版本筛选是期望历史记录在当前页消失的最常见原因。", "A fixed version filter is the most common reason an expected history entry disappears from the current page."],
  ["搜索词非空通常是目录查询零结果的最常见原因，其次是渠道不匹配或市场类型选错。", "A non-empty search term is the most common reason for zero catalog results, followed by channel mismatches or the wrong market kind."],
  ["运行中的任务最能反映短期状态变化，尤其是在刚提交之后。", "A running task is the best indicator of near-term state changes, especially right after a submission."],
  ["上下文里已经固定了任务 ID，所以直接解析当前阶段是判断这个包已成功、失败还是仍在运行的最快方式。", "A task id is already pinned in context, so resolving its current phase is the fastest way to understand whether this package already succeeded, failed, or is still running."],
  ["可信源已经解析完成，下一步就是决定当前是在处理插件包、支付包还是宿主管理模板。", "A trusted source is already resolved, so the next step is deciding whether you are operating on packages, payments, or host-managed templates."],
  ["当前可见任务都已稳定，因此版本上下文和历史会成为最有价值的验证面。", "All visible tasks are settled, so version context and history become the most useful validation surfaces."],
  ["已经选中了制品，但还没有固定具体版本；下一步最好决定是查看版本列表，还是让预览解析最新版本。", "An artifact is selected but no explicit version is pinned, so the next useful step is choosing whether to inspect the version list or let preview resolve the latest release."],
  ["已经选中了制品，但还没有固定具体版本；此时应在浏览版本列表和对当前目标预览最新版本之间做选择。", "An artifact is selected but no explicit version is pinned, so this is the point where you choose between browsing the version list and previewing the latest release against the current target."],
  ["制品详情已加载，但源没有暴露版本记录。如果你已经知道要查看的版本，请直接使用版本详情。", "Artifact detail loaded, but the source did not expose version records. Use release detail if you already know the version you want to inspect."],
  ["放宽历史筛选，或者返回当前制品上下文。", "Broaden the history filter or return to the current artifact context."],
  ["列表中的失败项风险最高，通常应优先处理。", "Failures are the highest-risk items in the list and usually require intervention before anything else."],
  ["宿主预览已通过，但这个版本引入了新权限，安装前应先审查。", "Host preview checks passed, but this release introduces new permissions that should be reviewed before install."],
  ["宿主预览已通过，这个版本可以通过宿主桥直接安装。", "Host preview checks passed. The release is ready to install through the host bridge."],
  ["宿主预览已通过，这个版本会升级现有插件安装。", "Host preview checks passed. This release will upgrade an existing plugin installation."],
  ["如果当前上下文还没有任务，通常需要先提交一次安装，任务跟踪才有意义。", "If no task exists in this context yet, you usually need to submit an install first before task tracking becomes useful."],
  ["查看上方版本详情，然后切换到另一个兼容的支付包版本。", "Inspect Release Detail above and choose another compatible payment package version."],
  ["查看上方版本详情，然后切换到另一个兼容的模板版本。", "Inspect Release Detail above and switch to another compatible template version."],
  ["查看上方版本详情，然后切换到另一个兼容版本。", "Inspect Release Detail above and switch to another compatible version."],
  ["包回滚可能还会继续经过宿主编排，所以接下来应核对任务或已恢复的版本上下文。", "Package rollback may continue through host orchestration, so verify the task or restored version context before taking further action."],
  ["稍后再刷新这个任务，或切换到任务列表查看更多进度。", "Refresh this task again after a short delay or switch to task list for broader progress tracking."],
  ["提交一次插件包安装，或者放宽当前任务筛选。", "Submit a plugin package install or broaden the current task filter."],
  ["固定版本的任务查询在刚切换上下文后通常过窄。", "Task queries scoped to a fixed version are often too narrow right after switching context."],
  ["模板历史还会按目标键分组，因此当前选择的邮件键或落地页 slug 可能与你期望查看的版本不一致。", "Template history is also scoped by target key, so the selected email key or landing slug may not match the release you expected."],
  ["宿主在比对内容并写入托管历史之前，模板导入必须先解析到明确的邮件键或落地页 slug。", "Template imports must resolve against a concrete email key or landing slug before the host can compare content and write managed history."],
  ["模板预览阶段由宿主确认目标解析、摘要变化，以及选中的版本是否应成为下一个托管修订。", "Template preview is where the host confirms target resolution, digest changes, and whether the selected release should become the next managed revision."],
  ["模板回滚会立即完成，因此下一步最好确认当前目标内容和历史位置。", "Template rollback completes immediately, so the next useful step is confirming the current target content and history position."],
  ["活动历史记录通常是验证宿主当前实际提供内容的最快方式。", "The active history entry is usually the quickest way to validate what the host is currently serving."],
  ["当前内容与目标一致，因此只有在你希望该版本被记录到宿主管理历史中时，导入才有意义。", "The content matches the current target, so importing is only useful if you want the release captured in host-managed history."],
  ["当前宿主桥无法按当前状态接受这个支付包。", "The current host bridge cannot accept this payment package as-is."],
  ["当前 source_id 不在插件配置里，因此修正源上下文之前，所有目录和详情查询都会失败。", "The current source_id is not present in plugin config, so every catalog or detail query will fail until the source context is corrected."],
  ["宿主桥拒绝了这个版本，下一步最好对比原始版本元数据或改选其他版本。", "The host bridge rejected this release, so the next useful step is to compare raw release metadata or choose another version."],
  ["宿主已经写入了模板修订，下一步最好验证当前活动目标状态，并保留一个可回滚点。", "The host has already written the template revision, so the next useful step is validating the active target state and keeping a rollback point in view."],
  ["宿主预览已通过，因此上方安装操作是激活这个包的最快路径。", "The host preview is green, so the install action above is the fastest path to activate this package."],
  ["宿主在导入前拒绝了这个版本，因此下一步最好改版本或改目标上下文。", "The host rejected this release before import, so changing version or target context is the next useful step."],
  ["宿主任务仍在进行中。请刷新任务视图或再次列出任务以跟踪阶段和进度变化。", "The host task is still in progress. Refresh the task view or list tasks again to track phase and progress changes."],
  ["宿主任务已进入终态成功。可复用下方上下文链接查看结果版本和历史。", "The host task reached a terminal success state. Reuse the context links below to inspect the resulting version and history."],
  ["宿主任务报告为失败状态。请先查看原始负载，并在提交重试或回滚后切回安装历史。", "The host task reported a failure state. Inspect the raw payload and switch back to install history once a retry or rollback has been submitted."],
  ["预览已就绪，此操作将升级当前已安装的插件版本。", "The preview is ready and this action will upgrade the currently installed plugin version."],
  ["预览显示当前目标确实存在内容变化，因此现在导入会更新宿主管理版本历史。", "The preview shows a real content change against the current target, so importing now will update the managed version history."],
  ["回滚结果已可用，因此请确认已恢复版本的上下文，并保留历史视图以便再次回滚。", "The rollback result is available now, so confirm the restored version in context and keep history visible for another rollback if needed."],
  ["选中的支付包与当前宿主桥不兼容。", "The selected payment package is not compatible with the current host bridge."],
  ["选中的版本与当前宿主桥不兼容。", "The selected release is not compatible with the current host bridge."],
  ["插件配置里当前已禁用所选源。", "The selected source is currently disabled in plugin configuration."],
  ["所选源当前已启用，并允许当前激活的市场类型。", "The selected source is enabled and currently allows the active market kind."],
  ["选中的模板版本与当前宿主桥不兼容。", "The selected template release is not compatible with the current host bridge."],
  ["选中的模板版本与当前原生内容摘要一致。如果你希望将该版本记录进宿主管理历史，仍然可以导入。", "The selected template release matches the current native content digest. Import is still available if you want the release tracked in host-managed history."],
  ["选中的模板版本已通过宿主预览检查，并会更新当前原生目标内容。", "The selected template release passed host preview checks and will update the current native target content."],
  ["选中的模板版本已通过宿主预览检查。原生目标尚不存在，因此导入会为这个目标创建第一条托管版本记录。", "The selected template release passed host preview checks. The native target does not exist yet, so the import will create the first managed version for this target."],
  ["目标已经选定，下一步最好决定哪个模板制品和哪个版本应该映射到这个原生目标。", "The target is already selected, so the next useful step is choosing which template artifact and release should map into that native target."],
  ["任务已经结束，因此最有价值的后续动作是验证结果版本和活动历史记录。", "The task has finished, so the most valuable follow-up is validating the resulting version and active history record."],
  ["任务仍在运行中，因此保持任务上下文固定是观察进度变化最直接的方式。", "The task is still running, so keeping the task context pinned is the most direct way to observe progress changes."],
  ["这个制品没有暴露版本列表。", "This artifact did not expose a version list."],
  ["这次导入会为所选目标创建第一条托管修订，因此提交前请再次确认目标键。", "This import will create the first managed revision for the selected target, so verify the target key before submission."],
  ["这个版本可以安装，但它引入了新权限，提交前应先审查。", "This release is installable, but it introduces new permissions that should be reviewed before submission."],
  ["在确认目标键和内容差异后，使用上方“导入模板”。", "Use Import Template above after confirming the target key and content diff."],
  ["如果这个预览结果正确，就使用上方“安装 / 导入”。", "Use Install / Import above when this preview looks correct."],
  ["如果没有可信源，插件就无法从任何注册表浏览目录数据或解析版本元数据。", "Without a trusted source, the plugin cannot browse catalog data or resolve release metadata from any registry."],
  ["零结果通常意味着所选源、渠道或制品类型当前没有暴露匹配的目录项。", "Zero results usually mean the chosen source, channel, or artifact kind does not currently expose matching catalog entries."],
  ["此版本没有请求额外的宿主权限。", "This release does not request any additional host permissions."],
  ["相比当前已安装的授权集，没有新增权限。", "No new permissions were introduced beyond the currently installed grant set."],
  ["此版本预览未返回宿主警告。", "No host preview warnings were raised for this release."],
  ["此支付包预览未返回宿主警告。", "No host preview warnings were raised for this payment package."],
  ["此模板版本预览未返回宿主警告。", "No host preview warnings were raised for this template release."],
  ["当前目标未找到宿主管理的模板历史。", "No host-managed template history was found for the current target."],
  ["未找到匹配的宿主安装任务。", "No matching host install tasks were found."],
  ["未找到匹配的宿主安装历史。", "No matching host install history was found."],
  ["需要 source_id", "source_id is required"],
  ["需要 name", "name is required"],
  ["不支持的市场预览类型", "unsupported market preview kind"],
  ["不支持的市场安装类型", "unsupported market install kind"],
  ["不支持的市场历史类型", "unsupported market history kind"],
  ["不支持的市场回滚类型", "unsupported market rollback kind"],
  ["宿主管理回滚需要 source_id、name 和 version", "source_id, name, and version are required for host-managed rollback"],
  ["需要 task_id，或无法从当前坐标解析匹配任务", "task_id is required or no matching task could be resolved from the current coordinates"],
  ["AuraLogic 市场已就绪。", "AuraLogic Market is ready."]
] as const;

const ADDITIONAL_PHRASE_PAIRS = [
  ["版本列表", "Version List"],
  ["版本相关链接", "Release Related Links"],
  ["从空白上下文重新检索", "Start Fresh Catalog Search"],
  ["在当前目标下检索其他模板", "Browse Another Template For Current Target"],
  ["当前选中源", "Current Selected Source"],
  ["目录查询结果", "Catalog Query Result"],
  ["目录条目", "Catalog Entries"],
  ["快速切换到版本上下文", "Quick Version Context"],
  ["快速进入操作上下文", "Quick Action Context"],
  ["源配置摘要", "Source Configuration Summary"],
  ["快速进入版本上下文", "Quick Version Context"],
  ["宿主适配信息", "Host Compatibility Details"],
  ["当前桥版本", "Current Bridge Version"],
  ["要求桥版本", "Required Bridge Version"],
  ["结果中的渠道", "Result Channels"],
  ["申请权限数", "Requested Permission Count"],
  ["历史条数", "History Entries"],
  ["允许当前类型", "Current Kind Allowed"],
  ["允许的类型", "Allowed Types"],
  ["条数", "Items"],
  ["新内容摘要", "Incoming Digest"],
  ["任务统计", "Task Statistics"],
  ["历史统计", "History Statistics"],
  ["当前激活）", "active)"],
  ["打开激活版本上下文", "Open Active Version Context"],
  ["打开导入后的版本上下文", "Open Imported Version Context"],
  ["返回源上下文", "Return To Source Context"],
  ["返回制品上下文", "Return To Artifact Context"],
  ["返回当前制品上下文", "Return To Current Artifact Context"],
  ["返回包上下文", "Return To Package Context"],
  ["原因", "Why Now"],
  ["建议操作", "Recommended Action"],
  ["关键词", "Keywords"],
  ["相对于当前已安装授权，没有新增权限。", "No new permissions were introduced beyond the currently installed grant set."],
  ["快捷入口", "Quick Links"],
  ["返回条数", "Items Returned"],
  ["警告数", "Warning Count"],
  ["切换到落地页模板", "Switch To Landing Page Template"],
  ["清空关键词", "Clear Search Keywords"],
  ["权限变化", "Permission Delta"],
  ["（当前激活）", " (active)"]
] as const;

const ADDITIONAL_EXACT_TEXTS = [
  ["已加载市场工作台默认值和当前管理端页面上下文。", "Loaded market workspace defaults and the current admin-page context."],
  ["已清空包相关筛选，并恢复到干净的包上下文。", "Cleared package-specific filters and restored a clean package context."],
  ["已清空模板相关筛选，并保留当前目标上下文。", "Cleared template-specific filters and preserved the current target context."],
  ["已加载源 ${state.source_id} 的详情。", "Loaded source detail."],
  ["支付包会通过原生支付方式导入器完成导入，便于在应用前确认目标和配置。", "Payment packages go through the native payment-method importer so the target and configuration can be confirmed before applying."],
  ["已通过宿主桥接导入 ${state.kind}。", "Imported through the host bridge."],
  ["当前筛选条件下没有匹配到任何市场制品。可以尝试清空关键词、切换渠道，或调整源/类型上下文。", "No market artifacts matched the current filters. Try clearing the keywords, switching channels, or adjusting the source/kind context."],
  ["已加载目录结果。你可以直接用下方快捷入口把某个版本带入预览或安装上下文。", "Loaded catalog results. You can use the shortcuts below to jump a version directly into preview or install context."],
  ["先放宽当前目录筛选，再重新查询。", "Relax the current catalog filters first, then query again."],
  ["有关键词时最容易出现零结果，其次是渠道不匹配或选错了市场类型。", "Keywords are the most common reason for zero catalog results, followed by channel mismatches or the wrong market kind."],
  ["当前市场插件还没有可用的可信源。", "There are currently no trusted sources available for the market plugin."],
  ["把 source_id 切换到一个已配置可信源，再刷新当前上下文。", "Switch source_id to a configured trusted source and then refresh the current context."],
  ["当前 source_id 不存在于插件配置中，所以目录查询和详情查询都会失败，必须先修正源上下文。", "The current source_id does not exist in plugin config, so catalog and detail queries will fail until the source context is corrected."],
  ["先决定进入包模式还是模板模式，再从目录查询或源详情开始。", "Choose package mode or template mode first, then start from catalog query or source detail."],
  ["当前已经成功解析到可信源，接下来应该先明确是在处理插件包、支付包，还是宿主管理模板。", "A trusted source is already resolved. The next step is deciding whether you are working with plugin packages, payment packages, or host-managed templates."],
  ["先查看已固定的任务 ${state.task_id}。", "Inspect the pinned task first."],
  ["当前上下文已经固定了任务 ID，先解析任务阶段是最快的判断方式，可以立即知道这个包到底成功、失败还是仍在运行。", "A task ID is already pinned in context, so resolving its phase is the fastest way to see whether this package succeeded, failed, or is still running."],
  ["先查询${formatMarketKindLabel(state.kind)}目录。", "Query the package catalog first."],
  ["当前还没有选中制品，先做目录检索是缩小范围的最快方式，之后再预览或安装会更顺手。", "No artifact is selected yet, so catalog search is the fastest way to narrow the scope before previewing or installing."],
  ["下一步查看制品详情，或直接预览最新版本。", "Next, inspect artifact detail or preview the latest version directly."],
  ["当前已经选中了制品，但还没有固定版本。现在最有价值的动作就是决定先看版本列表，还是直接让预览去解析最新版本。", "An artifact is already selected, but no version is pinned yet. The next useful step is deciding whether to inspect the version list or let preview resolve the latest version directly."],
  ["先预览 ${state.name}@${state.version}，确认通过后再安装或导入。", "Preview the selected release first, then install or import after the checks pass."],
  ["支付包最终仍会在原生导入器里完成，但预览是确认桥接兼容性和默认解析结果的最佳位置。", "Payment packages still complete in the native importer, but preview is the best place to confirm bridge compatibility and resolved defaults."],
  ["插件包在预览确认兼容性和权限影响之后，就可以直接从当前页提交安装。", "After preview confirms compatibility and permission impact, plugin packages can be submitted directly from this page."],
  ["模板导入必须先绑定到明确的邮件模板键或落地页 slug，宿主才能比较内容并写入管理历史。", "Template imports must be bound to a concrete email template key or landing-page slug before the host can compare content and write managed history."],
  ["下一步先查询模板目录。", "Next, query the template catalog."],
  ["当前目标已经确定，接下来最有价值的动作是决定哪个模板制品和哪个版本要映射到这个原生目标上。", "The target is already selected. The next most valuable action is deciding which template artifact and release should map to that native target."],
  ["下一步查看制品详情，或预览最新模板版本。", "Next, inspect artifact detail or preview the latest template version."],
  ["当前已经选中了模板制品，但还没有固定版本。现在应该在版本列表和直接预览最新版本之间做选择。", "A template artifact is already selected, but no version is pinned yet. Decide whether to inspect the version list or preview the latest version directly."],
  ["先对 ${targetKey} 预览 ${state.name}@${state.version}，确认差异正确后再导入。", "Preview the selected release against the target first, then import after confirming the diff."],
  ["模板预览会由宿主确认目标解析、摘要变化，以及这个版本是否应成为下一个受管修订。", "Template preview is where the host confirms target resolution, digest changes, and whether the selected release should become the next managed revision."],
  ["插件包走宿主安装桥接，可能产生异步安装任务。", "Plugin packages use the host install bridge and may create asynchronous install tasks."],
  ["模板的导入、历史查询和回滚都会立即通过宿主管理的模板历史执行。", "Template import, history lookup, and rollback all execute immediately through host-managed template history."],
  ["宿主返回的原始任务载荷，用于更细粒度的安装排查。", "Raw host task payload for low-level install diagnostics."],
  ["宿主返回的原始预览载荷，用于包兼容性和安装排查。", "Raw host preview payload for package compatibility and installer troubleshooting."],
  ["宿主桥接返回的原始插件安装结果。", "Raw plugin install payload returned by the host bridge."],
  ["宿主桥接返回的原始模板导入结果。", "Raw template import payload returned by the host bridge."],
  ["宿主桥接返回的原始回滚结果。", "Raw rollback payload returned by the host bridge."],
  ["Plugin.market.source.get() 返回的原始可信源配置。", "Raw trusted-source payload returned by Plugin.market.source.get()."],
  ["支付包导入预览返回的原始载荷，用于桥接排查和字段确认。", "Raw payment import preview payload for bridge debugging and field verification."],
  ["模板预览返回的原始载荷，用于目标解析和差异排查。", "Raw template preview payload for target resolution and diff troubleshooting."],
  ["这个版本没有申请额外宿主权限。", "This release does not request any additional host permissions."],
  ["这个版本的宿主预览没有产生任何警告。", "No host preview warnings were raised for this release."],
  ["这个支付包的宿主预览没有产生任何警告。", "No host preview warnings were raised for this payment package."],
  ["这个模板版本的宿主预览没有产生任何警告。", "No host preview warnings were raised for this template release."],
  ["支付包会通过原生支付导入器导入。当前已存在匹配的支付方式，建议先确认解析出的默认值，再决定是否覆盖。", "Payment packages import through the native payment importer. A matching payment method already exists, so review the resolved defaults before deciding whether to overwrite."],
  ["支付包会通过原生支付导入器导入。请打开管理端导入页选择目标支付方式并应用。", "Payment packages import through the native payment importer. Open the admin importer to choose the target method and apply the package."],
  ["模板回滚是即时完成的，所以下一步最有价值的是确认当前目标内容和历史位置。", "Template rollback completes immediately, so the next useful step is confirming the current target content and history position."],
  ["宿主任务已进入失败状态。请先检查原始载荷，再结合安装历史决定是重试还是回滚。", "The host task reported a failure state. Inspect the raw payload and review install history before retrying or rolling back."],
  ["宿主任务已成功结束。可以直接用下方快捷入口继续检查结果版本和历史记录。", "The host task reached a terminal success state. Use the shortcuts below to inspect the resulting version and history."],
  ["宿主任务仍在进行中。可以刷新当前任务，或返回任务列表观察阶段和进度变化。", "The host task is still running. Refresh the task or return to task list to observe phase and progress changes."],
  ["先检查失败任务，再决定重试还是回滚。", "Inspect a failed task first, then decide whether to retry or roll back."],
  ["优先检查最近仍在运行的任务，并保持版本上下文固定。", "Inspect the most recent running task first and keep the version context pinned."],
  ["检查最新成功版本上下文，或切换到历史记录。", "Inspect the latest successful version context or switch to history."],
  ["固定版本筛选后的任务查询通常会过窄，尤其是在刚切换上下文之后。", "Task queries scoped to a fixed version are often too narrow right after switching context."],
  ["如果当前上下文里还没有任务，通常需要先提交一次安装，任务跟踪才会有意义。", "If no task exists in this context yet, you usually need to submit an install first before task tracking becomes useful."],
  ["先打开当前激活版本上下文，再对比页面里的其他旧版本。", "Open the active version context first, then compare any older versions in view."],
  ["先打开最新可见历史项，确认预期版本是否已经激活。", "Open the newest visible history item and confirm whether the expected version is active."],
  ["模板历史还会按目标键做额外限定，所以当前选择的邮件模板键或落地页 slug 可能和你预期的版本不一致。", "Template history is also scoped by target key, so the selected email key or landing-page slug may not match the release you expected."],
  ["payment_package 与所有宿主管理模板/页面规则制品都是即时执行，不会产生异步宿主任务。", "payment_package and all host-managed template/page-rule actions execute immediately and do not create async host tasks."],
  ["没有宿主任务", "No Host Task"],
  ["当前包上下文下没有匹配到任何宿主任务。可以先提交一次安装，或清空版本筛选以查看更早的执行记录。", "No host tasks matched the current package context. Submit an install first or clear the version filter to inspect older runs."],
  ["当前上下文下没有匹配到任何宿主管理安装历史。可以切换制品或版本筛选来查看更早记录。", "No host-managed install history matched the current context. Change the artifact or version filter to inspect older records."],
  ["当前目标下没有找到宿主管理模板历史。", "No host-managed template history was found for the current target."],
  ["当前配置中未找到该源", "Not found in current config"],
  ["先查看上方版本详情，再切换到其他兼容版本。", "Inspect release detail above and switch to another compatible version."],
  ["先查看上方版本详情，再切换到其他兼容模板版本。", "Inspect release detail above and switch to another compatible template version."],
  ["先查看上方版本详情，再选择其他兼容的支付包版本。", "Inspect release detail above and choose another compatible payment-package version."],
  ["如果当前预览确认无误，就直接使用上方安装 / 导入动作。", "Use the Install / Import action above when this preview looks correct."],
  ["确认目标键和内容差异无误后，直接使用上方模板导入动作。", "Use the Import Template action above after confirming the target key and content diff."],
  ["先检查导入后的版本上下文，再回看宿主管理历史。", "Inspect the imported version context first and then review host-managed history."],
  ["持续检查任务 ${taskID}，直到它进入终态。", "Inspect the task until it reaches a terminal state."],
  ["检查已安装版本上下文和宿主历史。", "Inspect the installed version context and host history."],
  ["检查已恢复模板版本，并确认目标状态。", "Inspect the restored template version and confirm the target state."],
  ["检查已恢复的包上下文，并确认当前激活历史或任务状态。", "Inspect the restored package context and confirm active history or task status."],
  ["先检查失败版本上下文，再结合历史决定重试还是回滚。", "Inspect the failed version context, then review history before retrying or rolling back."],
  ["继续检查结果版本上下文和宿主历史。", "Inspect the resulting version context and host history."],
  ["稍后再次刷新这个任务，或切换到任务列表观察整体进度。", "Refresh this task again after a short delay or switch to task list for broader progress tracking."],
  ["任务失败后，下一步通常就是判断要不要重试同一版本，或从历史执行回滚。", "A failed task usually means the next decision is whether to retry the same release or roll back from history."],
  ["任务已经结束，最有价值的后续动作是验证结果版本和当前激活历史记录。", "The task has finished, so the most valuable follow-up is validating the resulting version and active history record."],
  ["任务还在运行，保持任务上下文固定是观察进度变化最直接的方法。", "The task is still running, so keeping the task context pinned is the most direct way to observe progress changes."],
  ["回滚结果现在已经可用，建议先确认恢复后的版本，并保留历史视图以便后续再次回滚。", "The rollback result is available now, so confirm the restored version in context and keep history visible for another rollback if needed."],
  ["包回滚可能还会继续经过宿主编排，所以继续操作前应先验证任务状态或已恢复版本上下文。", "Package rollback may continue through host orchestration, so verify the task or restored version context before taking further action."],
  ["如果当前没有返回任务 ID，因此已安装版本和历史视图就是验证结果的最佳位置。", "No task ID was returned, so the installed version and history views are the best places to verify the outcome."],
  ["宿主预览检查已通过，这个版本会升级现有插件安装。", "Host preview checks passed. This release will upgrade an existing plugin installation."],
  ["宿主预览检查已通过，这个版本已经可以通过宿主桥接安装。", "Host preview checks passed. The release is ready to install through the host bridge."],
  ["直接从当前预览打开原生支付导入页。", "Open the native payment importer from this preview."],
  ["支付包最终仍会在原生导入器里完成，但预览是确认桥接兼容性和默认解析结果的最佳位置。", "Payment packages still complete through the native importer, but preview is the right place to confirm bridge compatibility and resolved defaults first."],
  ["当前选中的源在插件配置中已被禁用。", "The selected source is currently disabled in plugin configuration."],
  ["当前源已启用，并且允许当前所选市场类型。", "The selected source is enabled and currently allows the active market kind."],
  ["当前源已启用，但暂不允许 ${state.kind}。继续之前请先切换类型，或更新插件源配置。", "The selected source is enabled, but it does not currently allow the active market kind. Switch kind or update the plugin source configuration before continuing."],
  ["已加载制品详情，但当前源没有暴露版本列表。若你已经知道版本号，可以继续查看版本详情。", "Artifact detail loaded, but the source did not expose version records. Use release detail if you already know the version you want to inspect."],
  ["已加载制品详情，共返回 ${versions.length} 条版本记录。若你要确认单个版本的宿主影响，请继续查看版本详情或直接预览。", "Artifact detail loaded with version records. Use release detail or preview when you want full metadata and host impact for a single version."],
  ["已从可信源加载版本元数据。若要在安装或导入前确认宿主兼容性和目标影响，请继续预览。", "Loaded raw release metadata from the trusted source. Use preview when you need host compatibility and target impact checks before install or import."],
  ["已加载版本元数据，但源返回的渠道是 ${channel}，当前表单上下文是 ${selectedChannel}。若要确认宿主兼容性和影响，请继续预览。", "Loaded raw release metadata, but the source reported a different channel than the current form context. Use preview when you need host compatibility and target impact checks."],
  ["当前制品没有暴露版本列表。", "This artifact did not expose a version list."],
  ["插件包安装通常会异步继续执行，所以任务跟踪是确认激活结果或失败细节的最快方式。", "Plugin package installs usually continue asynchronously, so task tracking is the fastest way to see activation or failure details."],
  ["宿主已经写入了这次模板修订，下一步最有价值的是确认当前激活目标状态，并把回滚点保留在视野内。", "The host has already written the template revision, so the next useful step is validating the active target state and keeping a rollback point in view."],
  ["已加载宿主任务。可以直接用下方快捷入口跳到最新任务或安装历史。", "Loaded host tasks. Reuse the context links below to jump into the latest task or installed version history."],
] as const;

const MACHINE_TEXTS: Record<string, readonly [string, string]> = {
  plugin_package: ["插件包", "Plugin Package"],
  payment_package: ["支付包", "Payment Package"],
  email_template: ["邮件模板", "Email Template"],
  landing_page_template: ["落地页模板", "Landing Page Template"],
  invoice_template: ["账单模板", "Invoice Template"],
  auth_branding_template: ["认证页品牌模板", "Auth Branding Template"],
  page_rule_pack: ["页面规则包", "Page Rule Pack"],
  payment_method: ["支付方式", "Payment Method"],
  true: ["是", "true"],
  false: ["否", "false"],
  ready: ["就绪", "ready"],
  rejected: ["已拒绝", "rejected"],
  rolled_back: ["已回滚", "rolled_back"],
  rollback_failed: ["回滚失败", "rollback_failed"],
  already_active: ["已是当前版本", "already_active"]
};

const GENERIC_TEXT_KEYS = new Set([
  "title",
  "label",
  "description",
  "summary",
  "placeholder",
  "empty_text",
  "recent_title",
  "content",
  "message",
  "notice",
  "load_label",
  "save_label",
  "reset_label"
]);

const KEYED_EXACT_PAIRS: ReadonlyArray<readonly [string, string]> = Object.values(KEYED_MESSAGES);
const KEYED_EXACT_PAIR_KEYS = new Set(KEYED_EXACT_PAIRS.map((pair) => `${pair[0]}\u0000${pair[1]}`));

function isKeyedExactPair(pair: readonly [string, string]): boolean {
  return KEYED_EXACT_PAIR_KEYS.has(`${pair[0]}\u0000${pair[1]}`);
}

const LEGACY_EXACT_PAIRS: Array<readonly [string, string]> = [
  ...PHRASE_PAIRS,
  ...EXACT_TEXTS,
  ...ADDITIONAL_PHRASE_PAIRS,
  ...ADDITIONAL_EXACT_TEXTS
].filter((pair) => !isKeyedExactPair(pair));

const EXACT_LOOKUP = new Map<string, readonly [string, string]>();

for (const pair of KEYED_EXACT_PAIRS.concat(LEGACY_EXACT_PAIRS)) {
  // Keep the earliest/canonical definition and treat later tables as additive compatibility aliases.
  if (!EXACT_LOOKUP.has(pair[0])) {
    EXACT_LOOKUP.set(pair[0], pair);
  }
  if (!EXACT_LOOKUP.has(pair[1])) {
    EXACT_LOOKUP.set(pair[1], pair);
  }
}

function isRecord(value: unknown): value is UnknownMap {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function translateExact(locale: MarketLocale, value: string): string | null {
  const pair = EXACT_LOOKUP.get(value);
  if (!pair) {
    return null;
  }
  return locale === "zh" ? pair[0] : pair[1];
}

function translateMachine(locale: MarketLocale, value: string): string | null {
  const pair = MACHINE_TEXTS[value];
  if (!pair) {
    return null;
  }
  return locale === "zh" ? pair[0] : pair[1];
}

function normalizeLocaleCandidate(value: unknown): MarketLocale | null {
  const raw = String(value || "").trim().toLowerCase();
  if (!raw) {
    return null;
  }
  const candidate = raw.split(",")[0].trim();
  if (candidate.startsWith("zh")) {
    return "zh";
  }
  if (candidate.startsWith("en")) {
    return "en";
  }
  return null;
}

export function resolveMarketLocale(
  context?: PluginExecutionContext,
  params?: UnknownMap
): MarketLocale {
  const metadata = (context?.metadata || {}) as Record<string, string>;
  return (
    normalizeLocaleCandidate(params?.locale) ||
    normalizeLocaleCandidate(params?.lang) ||
    normalizeLocaleCandidate(metadata.locale) ||
    normalizeLocaleCandidate(metadata.user_locale) ||
    normalizeLocaleCandidate(metadata.accept_language) ||
    "zh"
  );
}

export function translateMarketText(locale: MarketLocale, value: unknown): string {
  if (typeof value !== "string" || value.trim() === "") {
    return typeof value === "string" ? value : "";
  }
  const keyed = translateKeyed(locale, value);
  if (keyed !== null) {
    return keyed;
  }
  const exact = translateExact(locale, value);
  if (exact) {
    return exact;
  }
  const machine = translateMachine(locale, value);
  if (machine) {
    return machine;
  }
  const loadedSource = value.match(/^Loaded source detail for (.+)\.$/);
  if (loadedSource) {
    return locale === "zh" ? `已加载源 ${loadedSource[1]} 的详情。` : value;
  }
  const loadedArtifact = value.match(/^Loaded artifact detail for (.+)\.$/);
  if (loadedArtifact) {
    return locale === "zh" ? `已加载制品 ${loadedArtifact[1]} 的详情。` : value;
  }
  const loadedRelease = value.match(/^Loaded release metadata for (.+)\.$/);
  if (loadedRelease) {
    return locale === "zh" ? `已加载版本元数据：${loadedRelease[1]}。` : value;
  }
  const loadedTask = value.match(/^Loaded task (.+)\.$/);
  if (loadedTask) {
    return locale === "zh" ? `已加载任务 ${loadedTask[1]}。` : value;
  }
  const loadedCatalog = value.match(/^Loaded (\d+) catalog item\(s\)\.$/);
  if (loadedCatalog) {
    return locale === "zh" ? `已加载 ${loadedCatalog[1]} 个目录项。` : value;
  }
  const loadedHostTasks = value.match(/^Loaded (\d+) host task\(s\)\.$/);
  if (loadedHostTasks) {
    return locale === "zh" ? `已加载 ${loadedHostTasks[1]} 个宿主任务。` : value;
  }
  const loadedHistory = value.match(/^Loaded (\d+) host install history item\(s\)\.$/);
  if (loadedHistory) {
    return locale === "zh" ? `已加载 ${loadedHistory[1]} 条宿主安装历史。` : value;
  }
  const imported = value.match(/^Imported market (.+) through the host bridge\.$/);
  if (imported) {
    return locale === "zh"
      ? `已通过宿主桥接导入市场资源：${translateMarketText(locale, imported[1])}。`
      : value;
  }
  const failedArtifact = value.match(/^Failed to load artifact detail: (.+)$/);
  if (failedArtifact) {
    return locale === "zh" ? `加载制品详情失败：${failedArtifact[1]}` : value;
  }
  const failedNativeSnapshot = value.match(/^Failed to load native target snapshot: (.+)$/);
  if (failedNativeSnapshot) {
    return locale === "zh" ? `加载原生目标快照失败：${failedNativeSnapshot[1]}` : value;
  }
  const failedTemplateHistory = value.match(/^Failed to load template history for the current target: (.+)$/);
  if (failedTemplateHistory) {
    return locale === "zh" ? `加载当前目标的模板历史失败：${failedTemplateHistory[1]}` : value;
  }
  const rollbackStatus = value.match(/^Rollback status: (.+)\.$/);
  if (rollbackStatus) {
    return locale === "zh"
      ? `回滚状态：${translateMarketText(locale, rollbackStatus[1])}。`
      : value;
  }
  const inspectTask = value.match(/^Inspect Task (.+)$/);
  if (inspectTask) {
    return locale === "zh" ? `查看任务 ${inspectTask[1]}` : value;
  }
  const reuseContext = value.match(/^Reuse (.+)$/);
  if (reuseContext) {
    return locale === "zh" ? `复用 ${reuseContext[1]}` : value;
  }
  const openLabel = value.match(/^Open (.+)$/);
  if (openLabel) {
    return locale === "zh" ? `打开${translateMarketText(locale, openLabel[1])}` : value;
  }
  const inspectLabel = value.match(/^Inspect (.+)$/);
  if (inspectLabel) {
    return locale === "zh" ? `查看${translateMarketText(locale, inspectLabel[1])}` : value;
  }
  const clearLabel = value.match(/^Clear (.+)$/);
  if (clearLabel) {
    return locale === "zh" ? `清除${translateMarketText(locale, clearLabel[1])}` : value;
  }
  const resetLabel = value.match(/^Reset (.+)$/);
  if (resetLabel) {
    return locale === "zh" ? `重置${translateMarketText(locale, resetLabel[1])}` : value;
  }
  const useLabel = value.match(/^Use (.+)$/);
  if (useLabel) {
    return locale === "zh" ? `使用${translateMarketText(locale, useLabel[1])}` : value;
  }
  const importLabel = value.match(/^Import (.+)$/);
  if (importLabel) {
    return locale === "zh" ? `导入 ${importLabel[1]}` : value;
  }
  const installLabel = value.match(/^Install (.+)$/);
  if (installLabel) {
    return locale === "zh" ? `安装 ${installLabel[1]}` : value;
  }
  const previewLabel = value.match(/^Preview (.+)$/);
  if (previewLabel) {
    return locale === "zh" ? `预览 ${previewLabel[1]}` : value;
  }
  const activeSuffix = value.match(/^(.+) \(active\)$/);
  if (activeSuffix) {
    return locale === "zh" ? `${activeSuffix[1]}（当前生效）` : value;
  }
  const freshInstall = value.match(/^Fresh install$/);
  if (freshInstall) {
    return locale === "zh" ? "首次安装" : value;
  }
  const upgradeInstall = value.match(/^Upgrade existing install$/);
  if (upgradeInstall) {
    return locale === "zh" ? "升级现有安装" : value;
  }
  const reapplyInstall = value.match(/^Reapply existing version$/);
  if (reapplyInstall) {
    return locale === "zh" ? "重新应用当前版本" : value;
  }
  const noDigest = value.match(/^Target exists but digest is unavailable$/);
  if (noDigest) {
    return locale === "zh" ? "目标已存在，但摘要不可用" : value;
  }
  const newTarget = value.match(/^New native target$/);
  if (newTarget) {
    return locale === "zh" ? "新的原生目标" : value;
  }
  const noDiff = value.match(/^No content diff detected$/);
  if (noDiff) {
    return locale === "zh" ? "未检测到内容差异" : value;
  }
  const contentChanged = value.match(/^Content will change$/);
  if (contentChanged) {
    return locale === "zh" ? "内容将发生变化" : value;
  }
  const loadedSourceZh = value.match(/^已加载源 (.+) 的详情。$/);
  if (loadedSourceZh) {
    return locale === "en" ? `Loaded source detail for ${loadedSourceZh[1]}.` : value;
  }
  const loadedArtifactZh = value.match(/^已加载制品 (.+) 的详情。$/);
  if (loadedArtifactZh) {
    return locale === "en" ? `Loaded artifact detail for ${loadedArtifactZh[1]}.` : value;
  }
  const loadedReleaseZh = value.match(/^已加载版本 (.+) 的元数据。$/);
  if (loadedReleaseZh) {
    return locale === "en" ? `Loaded release metadata for ${loadedReleaseZh[1]}.` : value;
  }
  const loadedTaskZh = value.match(/^已加载任务 (.+)。$/);
  if (loadedTaskZh) {
    return locale === "en" ? `Loaded task ${loadedTaskZh[1]}.` : value;
  }
  const loadedCatalogZh = value.match(/^已加载 (\d+) 个目录项。$/);
  if (loadedCatalogZh) {
    return locale === "en" ? `Loaded ${loadedCatalogZh[1]} catalog item(s).` : value;
  }
  const loadedCandidateZh = value.match(/^已加载 (\d+) 个候选版本。可以直接用下方上下文快捷入口去预览或安装 (.+)。$/);
  if (loadedCandidateZh) {
    return locale === "en"
      ? `Loaded ${loadedCandidateZh[1]} candidate release(s). Reuse the context links below to preview or install ${loadedCandidateZh[2]}.`
      : value;
  }
  const loadedHostTasksZh = value.match(/^已加载 (\d+) 个宿主任务。$/);
  if (loadedHostTasksZh) {
    return locale === "en" ? `Loaded ${loadedHostTasksZh[1]} host task(s).` : value;
  }
  const loadedHostTasksFailedZh = value.match(/^已加载 (\d+) 个宿主任务，其中有 (\d+) 个失败任务。建议先排查失败任务，再决定重试或回滚。$/);
  if (loadedHostTasksFailedZh) {
    return locale === "en"
      ? `Loaded ${loadedHostTasksFailedZh[1]} host task(s), including ${loadedHostTasksFailedZh[2]} failed task(s). Inspect the failed task context before retrying or rolling back.`
      : value;
  }
  const loadedHostTasksRunningZh = value.match(/^已加载 (\d+) 个宿主任务，其中 (\d+) 个仍在运行或等待中。$/);
  if (loadedHostTasksRunningZh) {
    return locale === "en"
      ? `Loaded ${loadedHostTasksRunningZh[1]} host task(s). ${loadedHostTasksRunningZh[2]} task(s) are still running or pending.`
      : value;
  }
  const loadedHistoryZh = value.match(/^已加载 (\d+) 条宿主安装历史。$/);
  if (loadedHistoryZh) {
    return locale === "en" ? `Loaded ${loadedHistoryZh[1]} host install history item(s).` : value;
  }
  const loadedHistoryActiveZh = value.match(/^已加载 (\d+) 条历史记录，其中 (\d+) 条为当前激活版本。$/);
  if (loadedHistoryActiveZh) {
    return locale === "en"
      ? `Loaded ${loadedHistoryActiveZh[1]} history item(s), including ${loadedHistoryActiveZh[2]} active version record(s).`
      : value;
  }
  const loadedHistoryNoActiveZh = value.match(/^已加载 (\d+) 条历史记录，但返回页里没有标记当前激活版本。建议放宽上下文，或返回任务结果继续确认。$/);
  if (loadedHistoryNoActiveZh) {
    return locale === "en"
      ? `Loaded ${loadedHistoryNoActiveZh[1]} history item(s). No active version is marked in the returned page, so expand the context or inspect task results.`
      : value;
  }
  const importedZh = value.match(/^已通过宿主桥接导入 (.+)。$/);
  if (importedZh) {
    return locale === "en"
      ? `Imported market ${translateMarketText(locale, importedZh[1])} through the host bridge.`
      : value;
  }
  const failedArtifactZh = value.match(/^加载制品详情失败：(.+)$/);
  if (failedArtifactZh) {
    return locale === "en" ? `Failed to load artifact detail: ${failedArtifactZh[1]}` : value;
  }
  const failedNativeSnapshotZh = value.match(/^加载原生目标快照失败：(.+)$/);
  if (failedNativeSnapshotZh) {
    return locale === "en" ? `Failed to load native target snapshot: ${failedNativeSnapshotZh[1]}` : value;
  }
  const failedTemplateHistoryZh = value.match(/^加载当前目标的模板历史失败：(.+)$/);
  if (failedTemplateHistoryZh) {
    return locale === "en" ? `Failed to load template history for the current target: ${failedTemplateHistoryZh[1]}` : value;
  }
  const failedEmailTargetsZh = value.match(/^加载邮件模板目标失败：(.+)$/);
  if (failedEmailTargetsZh) {
    return locale === "en" ? `Failed to load email targets: ${failedEmailTargetsZh[1]}` : value;
  }
  const hostImportStatusZh = value.match(/^宿主导入状态：(.+)。$/);
  if (hostImportStatusZh) {
    return locale === "en" ? `Host import status: ${translateMarketText(locale, hostImportStatusZh[1])}.` : value;
  }
  const hostInstallStatusZh = value.match(/^宿主安装状态：(.+)。$/);
  if (hostInstallStatusZh) {
    return locale === "en" ? `Host install status: ${translateMarketText(locale, hostInstallStatusZh[1])}.` : value;
  }
  const rollbackStatusZh = value.match(/^回滚状态：(.+)。$/);
  if (rollbackStatusZh) {
    return locale === "en" ? `Rollback status: ${translateMarketText(locale, rollbackStatusZh[1])}.` : value;
  }
  const pinnedTaskZh = value.match(/^先查看已固定的任务 (.+)。$/);
  if (pinnedTaskZh) {
    return locale === "en" ? `Inspect pinned task ${pinnedTaskZh[1]} first.` : value;
  }
  const inspectTaskZh = value.match(/^持续检查任务 (.+)，直到它进入终态。$/);
  if (inspectTaskZh) {
    return locale === "en" ? `Inspect task ${inspectTaskZh[1]} until it reaches a terminal state.` : value;
  }
  const previewReleaseZh = value.match(/^先预览 (.+)，确认通过后再安装或导入。$/);
  if (previewReleaseZh) {
    return locale === "en" ? `Preview ${previewReleaseZh[1]}, then install or import if the host checks pass.` : value;
  }
  const previewTargetZh = value.match(/^先对 (.+) 预览 (.+)，确认差异正确后再导入。$/);
  if (previewTargetZh) {
    return locale === "en" ? `Preview ${previewTargetZh[2]} against ${previewTargetZh[1]}, then import if the diff is correct.` : value;
  }
  const openTaskZh = value.match(/^打开任务 (.+)$/);
  if (openTaskZh) {
    return locale === "en" ? `Open Task ${openTaskZh[1]}` : value;
  }
  const reuseContextZh = value.match(/^复用 (.+)$/);
  if (reuseContextZh) {
    return locale === "en" ? `Reuse ${reuseContextZh[1]}` : value;
  }
  const openLabelZh = value.match(/^打开(.+)$/);
  if (openLabelZh) {
    return locale === "en" ? `Open ${translateMarketText(locale, openLabelZh[1])}` : value;
  }
  const inspectLabelZh = value.match(/^查看(.+)$/);
  if (inspectLabelZh) {
    return locale === "en" ? `Inspect ${translateMarketText(locale, inspectLabelZh[1])}` : value;
  }
  const clearLabelZh = value.match(/^清除(.+)$/);
  if (clearLabelZh) {
    return locale === "en" ? `Clear ${translateMarketText(locale, clearLabelZh[1])}` : value;
  }
  const resetLabelZh = value.match(/^重置(.+)$/);
  if (resetLabelZh) {
    return locale === "en" ? `Reset ${translateMarketText(locale, resetLabelZh[1])}` : value;
  }
  const useLabelZh = value.match(/^使用 (.+)$/);
  if (useLabelZh) {
    return locale === "en" ? `Use ${translateMarketText(locale, useLabelZh[1])}` : value;
  }
  const importLabelZh = value.match(/^导入 (.+)$/);
  if (importLabelZh) {
    return locale === "en" ? `Import ${importLabelZh[1]}` : value;
  }
  const installLabelZh = value.match(/^安装 (.+)$/);
  if (installLabelZh) {
    return locale === "en" ? `Install ${installLabelZh[1]}` : value;
  }
  const previewLabelZh = value.match(/^预览 (.+)$/);
  if (previewLabelZh) {
    return locale === "en" ? `Preview ${previewLabelZh[1]}` : value;
  }
  const activeSuffixZh = value.match(/^(.+)（当前(?:激活|生效)）$/);
  if (activeSuffixZh) {
    return locale === "en" ? `${activeSuffixZh[1]} (active)` : value;
  }
  const freshInstallZh = value.match(/^首次安装$/);
  if (freshInstallZh) {
    return locale === "en" ? "Fresh install" : value;
  }
  const upgradeInstallZh = value.match(/^升级现有安装$/);
  if (upgradeInstallZh) {
    return locale === "en" ? "Upgrade existing install" : value;
  }
  const reapplyInstallZh = value.match(/^重新应用当前版本$/);
  if (reapplyInstallZh) {
    return locale === "en" ? "Reapply existing version" : value;
  }
  const noDigestZh = value.match(/^目标已存在，但摘要不可用$/);
  if (noDigestZh) {
    return locale === "en" ? "Target exists but digest is unavailable" : value;
  }
  const newTargetZh = value.match(/^新的原生目标$/);
  if (newTargetZh) {
    return locale === "en" ? "New native target" : value;
  }
  const noDiffZh = value.match(/^未检测到内容差异$/);
  if (noDiffZh) {
    return locale === "en" ? "No content diff detected" : value;
  }
  const contentChangedZh = value.match(/^内容将发生变化$/);
  if (contentChangedZh) {
    return locale === "en" ? "Content will change" : value;
  }
  const unhandledHook = value.match(/^Unhandled hook (.+)\.$/);
  if (unhandledHook) {
    return locale === "zh" ? `未处理的 Hook：${unhandledHook[1]}。` : value;
  }
  const unhandledHookZh = value.match(/^未处理的 Hook：(.+)。$/);
  if (unhandledHookZh) {
    return locale === "en" ? `Unhandled hook ${unhandledHookZh[1]}.` : value;
  }
  return value;
}

function localizeActionFormData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (typeof data.recent_title === "string") {
    next.recent_title = translateMarketText(locale, data.recent_title);
  }
  if (isRecord(data.actions)) {
    const actions: UnknownMap = {
      ...data.actions
    };
    if (typeof actions.load_label === "string") {
      actions.load_label = translateMarketText(locale, actions.load_label);
    }
    if (typeof actions.save_label === "string") {
      actions.save_label = translateMarketText(locale, actions.save_label);
    }
    if (typeof actions.reset_label === "string") {
      actions.reset_label = translateMarketText(locale, actions.reset_label);
    }
    if (Array.isArray(actions.extra)) {
      actions.extra = actions.extra.map((item) =>
        isRecord(item) && typeof item.label === "string"
          ? {
              ...item,
              label: translateMarketText(locale, item.label)
            }
          : item
      );
    }
    next.actions = actions;
  }
  if (Array.isArray(data.presets)) {
    next.presets = data.presets.map((item) =>
      isRecord(item)
        ? {
            ...item,
            label:
              typeof item.label === "string" ? translateMarketText(locale, item.label) : item.label,
            description:
              typeof item.description === "string"
                ? translateMarketText(locale, item.description)
                : item.description
          }
        : item
    );
  }
  if (Array.isArray(data.fields)) {
    next.fields = data.fields.map((item) => {
      if (!isRecord(item)) {
        return item;
      }
      const field: UnknownMap = {
        ...item
      };
      if (typeof item.label === "string") {
        field.label = translateMarketText(locale, item.label);
      }
      if (typeof item.description === "string") {
        field.description = translateMarketText(locale, item.description);
      }
      if (typeof item.placeholder === "string") {
        field.placeholder = translateMarketText(locale, item.placeholder);
      }
      if (Array.isArray(item.options)) {
        field.options = item.options.map((option) =>
          isRecord(option) && typeof option.label === "string"
            ? {
                ...option,
                label: translateMarketText(locale, option.label)
              }
            : option
        );
      }
      return field;
    });
  }
  return next;
}

function localizeDisplayValue(value: unknown, locale: MarketLocale): unknown {
  if (typeof value === "string") {
    return translateMarketText(locale, value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => localizeDisplayValue(item, locale));
  }
  return value;
}

function localizeGenericTextData(value: unknown, locale: MarketLocale): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => localizeGenericTextData(item, locale));
  }
  if (!isRecord(value)) {
    return value;
  }
  const next: UnknownMap = {};
  Object.entries(value).forEach(([key, item]) => {
    if (typeof item === "string" && GENERIC_TEXT_KEYS.has(key)) {
      next[key] = translateMarketText(locale, item);
      return;
    }
    if (Array.isArray(item) || isRecord(item)) {
      next[key] = localizeGenericTextData(item, locale);
      return;
    }
    next[key] = item;
  });
  return next;
}

function localizeKeyValueData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (Array.isArray(data.items)) {
    next.items = data.items.map((item) =>
      isRecord(item)
        ? {
            ...item,
            label: typeof item.label === "string" ? translateMarketText(locale, item.label) : item.label,
            description:
              typeof item.description === "string"
                ? translateMarketText(locale, item.description)
                : item.description,
            value: localizeDisplayValue(item.value, locale)
          }
        : item
    );
  }
  return next;
}

function localizeStatsGridData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (Array.isArray(data.items)) {
    next.items = data.items.map((item) =>
      isRecord(item)
        ? {
            ...item,
            label: typeof item.label === "string" ? translateMarketText(locale, item.label) : item.label,
            description:
              typeof item.description === "string"
                ? translateMarketText(locale, item.description)
                : item.description,
            value: localizeDisplayValue(item.value, locale)
          }
        : item
    );
  }
  return next;
}

function localizeLinkListData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (Array.isArray(data.links)) {
    next.links = data.links.map((item) =>
      isRecord(item)
        ? {
            ...item,
            label: typeof item.label === "string" ? translateMarketText(locale, item.label) : item.label
          }
        : item
    );
  }
  return next;
}

function localizeBadgeListData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (Array.isArray(data.items)) {
    next.items = data.items.map((item) => localizeDisplayValue(item, locale));
  }
  return next;
}

function localizeJSONViewData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  if (typeof data.summary === "string") {
    next.summary = translateMarketText(locale, data.summary);
  }
  return next;
}

function localizeTableData(data: UnknownMap, locale: MarketLocale): UnknownMap {
  const next: UnknownMap = {
    ...data
  };
  const columns = Array.isArray(data.columns) ? data.columns.map((item) => String(item)) : [];
  const localizedColumns = columns.map((item) => translateMarketText(locale, item));
  if (localizedColumns.length > 0) {
    next.columns = localizedColumns;
  }
  if (typeof data.empty_text === "string") {
    next.empty_text = translateMarketText(locale, data.empty_text);
  }
  if (Array.isArray(data.rows) && columns.length > 0) {
    next.rows = data.rows.map((row) => {
      if (!isRecord(row)) {
        return row;
      }
      const localizedRow: UnknownMap = {};
      columns.forEach((key, index) => {
        localizedRow[localizedColumns[index] || key] = localizeDisplayValue(row[key], locale);
      });
      Object.keys(row).forEach((key) => {
        if (!columns.includes(key)) {
          localizedRow[key] = localizeDisplayValue(row[key], locale);
        }
      });
      return localizedRow;
    });
  }
  return next;
}

function localizeMarketBlock(block: PluginPageBlock, locale: MarketLocale): PluginPageBlock {
  const next: PluginPageBlock = {
    ...block,
    title: typeof block.title === "string" ? translateMarketText(locale, block.title) : block.title
  };
  if (typeof block.content === "string") {
    if (block.type === "html") {
      next.content = translateEmbeddedMarketTokens(locale, block.content);
    } else {
      next.content = translateMarketText(locale, block.content);
    }
  }
  if (isRecord(block.data)) {
    let data: UnknownMap = {
      ...block.data
    };
    switch (block.type) {
      case "table":
        data = localizeTableData(data, locale);
        break;
      case "action_form":
        data = localizeActionFormData(data, locale);
        break;
      case "key_value":
        data = localizeKeyValueData(data, locale);
        break;
      case "stats_grid":
        data = localizeStatsGridData(data, locale);
        break;
      case "link_list":
        data = localizeLinkListData(data, locale);
        break;
      case "badge_list":
        data = localizeBadgeListData(data, locale);
        break;
      case "json_view":
        data = localizeJSONViewData(data, locale);
        break;
      default:
        break;
    }
    next.data = localizeGenericTextData(data, locale) as UnknownMap;
  }
  return next;
}

export function localizeMarketBlocks(
  blocks: PluginPageBlock[] | undefined,
  locale: MarketLocale
): PluginPageBlock[] | undefined {
  if (!Array.isArray(blocks)) {
    return blocks;
  }
  return blocks.map((block) => localizeMarketBlock(block, locale));
}

function localizeMarketPageSchema(page: PluginPageSchema, locale: MarketLocale): PluginPageSchema {
  return {
    ...page,
    title: typeof page.title === "string" ? translateMarketText(locale, page.title) : page.title,
    description:
      typeof page.description === "string"
        ? translateMarketText(locale, page.description)
        : page.description,
    blocks: localizeMarketBlocks(page.blocks, locale)
  };
}

export function localizeMarketFrontendExtensions(
  extensions: PluginFrontendExtension[] | undefined,
  locale: MarketLocale
): PluginFrontendExtension[] | undefined {
  if (!Array.isArray(extensions)) {
    return extensions;
  }
  return extensions.map((extension) => {
    const next: PluginFrontendExtension = {
      ...extension,
      title:
        typeof extension.title === "string"
          ? translateMarketText(locale, extension.title)
          : extension.title,
      content:
        typeof extension.content === "string"
          ? translateMarketText(locale, extension.content)
          : extension.content
    };
    if (isRecord(extension.data)) {
      const data: UnknownMap = {
        ...extension.data
      };
      if (typeof data.title === "string") {
        data.title = translateMarketText(locale, data.title);
      }
      if (typeof data.label === "string") {
        data.label = translateMarketText(locale, data.label);
      }
      if (isRecord(data.page)) {
        data.page = localizeMarketPageSchema(data.page as PluginPageSchema, locale);
      }
      next.data = localizeGenericTextData(data, locale) as UnknownMap;
    }
    return next;
  });
}

function localizeMarketResultData(
  data: UnknownMap | undefined,
  locale: MarketLocale
): UnknownMap | undefined {
  if (!isRecord(data)) {
    return data;
  }
  const next: UnknownMap = {
    ...data
  };
  if (typeof data.source === "string") {
    next.source = translateMarketText(locale, data.source);
  }
  if (typeof data.message === "string") {
    next.message = translateMarketText(locale, data.message);
  }
  if (typeof data.notice === "string") {
    next.notice = translateMarketText(locale, data.notice);
  }
  if (typeof data.title === "string") {
    next.title = translateMarketText(locale, data.title);
  }
  if (typeof data.description === "string") {
    next.description = translateMarketText(locale, data.description);
  }
  if (Array.isArray(data.blocks)) {
    next.blocks = localizeMarketBlocks(data.blocks as PluginPageBlock[], locale);
  }
  if (Array.isArray(data.frontend_extensions)) {
    next.frontend_extensions = localizeMarketFrontendExtensions(
      data.frontend_extensions as PluginFrontendExtension[],
      locale
    );
  }
  if (isRecord(data.page)) {
    next.page = localizeMarketPageSchema(data.page as PluginPageSchema, locale);
  }
  return next;
}

export function localizeMarketExecuteResult(
  result: PluginExecuteResult,
  locale: MarketLocale
): PluginExecuteResult {
  if (result.success) {
    const localized: PluginExecuteResult = {
      ...result
    };
    if (typeof result.message === "string") {
      localized.message = translateMarketText(locale, result.message);
    }
    if (typeof result.block_reason === "string") {
      localized.block_reason = translateMarketText(locale, result.block_reason);
    }
    if (typeof result.reason === "string") {
      localized.reason = translateMarketText(locale, result.reason);
    }
    const localizedData = localizeMarketResultData(result.data as UnknownMap | undefined, locale);
    if (localizedData !== undefined) {
      localized.data = localizedData;
    } else {
      delete localized.data;
    }
    const localizedExtensions = localizeMarketFrontendExtensions(result.frontend_extensions, locale);
    if (localizedExtensions !== undefined) {
      localized.frontend_extensions = localizedExtensions;
    } else {
      delete localized.frontend_extensions;
    }
    return localized;
  }
  const localizedError: PluginExecuteResult = {
    ...result,
    error: translateMarketText(locale, result.error)
  };
  const localizedData = localizeMarketResultData(result.data as UnknownMap | undefined, locale);
  if (localizedData !== undefined) {
    localizedError.data = localizedData;
  } else {
    delete localizedError.data;
  }
  return localizedError;
}
