import {
  buildPluginActionFormBlock,
  buildPluginPageBootstrap,
  type PluginActionFormConditionMatcher,
  type PluginFrontendExtension,
  type PluginPageBlock,
  type PluginPageSchema,
  type UnknownMap,
} from "@auralogic/plugin-sdk";

import {
  ADMIN_PLUGIN_PAGE_PATH,
  PLUGIN_DISPLAY_NAME,
  type PackageMarketKind,
  type SupportedMarketKind,
  type TemplateMarketKind,
} from "./constants";
import { marketMessage } from "./i18n";
import { buildTrustedMarketWorkspaceBlock } from "./trusted-workspace";

export type MarketConsoleState = {
  source_id: string;
  kind: SupportedMarketKind;
  channel: string;
  q: string;
  name: string;
  version: string;
  workflow_stage: string;
  activate: boolean;
  auto_start: boolean;
  granted_permissions_json: string;
  note: string;
  email_key: string;
  landing_slug: string;
  task_id: string;
};

const t = marketMessage;
const MARKET_PAGE_EXECUTE_ACTIONS = [
  "market.console.load",
  "market.package.load",
  "market.package.reset",
  "market.template.load",
  "market.template.reset",
  "market.trusted.asset",
  "market.source.detail",
  "market.catalog.query",
  "market.artifact.detail",
  "market.release.detail",
  "market.release.preview",
  "market.install.execute",
  "market.install.task.get",
  "market.install.task.list",
  "market.install.history.list",
  "market.install.rollback",
] as const;

function allOf(...matchers: PluginActionFormConditionMatcher[]): PluginActionFormConditionMatcher[] {
  return matchers;
}

function stageIn(...stages: string[]): PluginActionFormConditionMatcher {
  return {
    field: "workflow_stage",
    in: stages,
  };
}

export function buildDefaultMarketConsoleState(config: UnknownMap): MarketConsoleState {
  const sourceID = String(config.source_id || "official").trim() || "official";
  const defaultChannel = String(config.default_channel || "stable").trim() || "stable";
  return {
    source_id: sourceID,
    kind: "plugin_package",
    channel: defaultChannel,
    q: "",
    name: "",
    version: "",
    workflow_stage: "source",
    activate: true,
    auto_start: false,
    granted_permissions_json: "[]",
    note: "",
    email_key: "order_paid",
    landing_slug: "home",
    task_id: "",
  };
}

function buildPackageFormState(
  defaults: MarketConsoleState,
  kind: PackageMarketKind,
): MarketConsoleState {
  return {
    ...defaults,
    kind,
    q: "",
    name: "",
    version: "",
    task_id: "",
    note: "",
    activate: true,
    auto_start: false,
    granted_permissions_json: "[]",
  };
}

function buildTemplateFormState(
  defaults: MarketConsoleState,
  kind: TemplateMarketKind,
): MarketConsoleState {
  return {
    ...defaults,
    kind,
    q: "",
    name: "",
    version: "",
    task_id: "",
    note: "",
    email_key: kind === "email_template" ? defaults.email_key : "",
    landing_slug: kind === "landing_page_template" ? defaults.landing_slug : "",
  };
}

function escapeHTML(value: unknown): string {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function markActionFormAsTrustedFallback(block: PluginPageBlock): PluginPageBlock {
  const baseData =
    block.data && typeof block.data === "object" && !Array.isArray(block.data)
      ? (block.data as UnknownMap)
      : {};
  return {
    ...block,
    data: {
      ...baseData,
      fallback_when_untrusted: true,
      hide_after_trusted_boot: true,
    },
  };
}

function buildMarketOverviewBlock(defaults: MarketConsoleState): PluginPageBlock {
  const overviewURL = ADMIN_PLUGIN_PAGE_PATH;
  const pluginPackageURL = `${ADMIN_PLUGIN_PAGE_PATH}?kind=plugin_package`;
  const paymentPackageURL = `${ADMIN_PLUGIN_PAGE_PATH}?kind=payment_package`;
  const emailTemplateURL =
    `${ADMIN_PLUGIN_PAGE_PATH}?kind=email_template&email_key=${encodeURIComponent(defaults.email_key)}`;
  const landingTemplateURL =
    `${ADMIN_PLUGIN_PAGE_PATH}?kind=landing_page_template&landing_slug=${encodeURIComponent(defaults.landing_slug)}`;
  const invoiceTemplateURL = `${ADMIN_PLUGIN_PAGE_PATH}?kind=invoice_template`;
  const authBrandingTemplateURL = `${ADMIN_PLUGIN_PAGE_PATH}?kind=auth_branding_template`;
  const pageRulePackURL = `${ADMIN_PLUGIN_PAGE_PATH}?kind=page_rule_pack`;

  return {
    type: "html",
    content: t("frontend.overview.html", {
      displayName: escapeHTML(PLUGIN_DISPLAY_NAME),
      adminRoute: escapeHTML(ADMIN_PLUGIN_PAGE_PATH),
      sourceId: escapeHTML(defaults.source_id),
      channel: escapeHTML(defaults.channel),
      emailKey: escapeHTML(defaults.email_key),
      landingSlug: escapeHTML(defaults.landing_slug),
      overviewUrl: escapeHTML(overviewURL),
      pluginUrl: escapeHTML(pluginPackageURL),
      paymentUrl: escapeHTML(paymentPackageURL),
      emailUrl: escapeHTML(emailTemplateURL),
      landingUrl: escapeHTML(landingTemplateURL),
      invoiceUrl: escapeHTML(invoiceTemplateURL),
      authBrandingUrl: escapeHTML(authBrandingTemplateURL),
      pageRuleUrl: escapeHTML(pageRulePackURL),
    }),
    data: {
      theme: "host",
      chrome: "bare",
      fallback_when_untrusted: true,
    },
  };
}

function buildTrustedWorkspaceBlock(defaults: MarketConsoleState): PluginPageBlock {
  return buildTrustedMarketWorkspaceBlock(defaults);
}

export function buildBootstrapExtensions(config: UnknownMap, queryParams: UnknownMap = {}): PluginFrontendExtension[] {
  const page = {
    title: PLUGIN_DISPLAY_NAME,
    description: t("frontend.page.description"),
    host_header: "hide",
    host_market_workspace: true,
    execute_actions: [...MARKET_PAGE_EXECUTE_ACTIONS],
    blocks: buildMarketPageBlocks(config, queryParams),
  } as PluginPageSchema;

  return buildPluginPageBootstrap({
    area: "admin",
    path: ADMIN_PLUGIN_PAGE_PATH,
    title: PLUGIN_DISPLAY_NAME,
    priority: 96,
    icon: "package",
    required_permissions: ["market.view"],
    guest_visible: false,
    mobile_visible: false,
    page,
  });
}

function buildMarketPageBlocks(config: UnknownMap, queryParams: UnknownMap = {}): PluginPageBlock[] {
  const defaults = buildDefaultMarketConsoleState(config);
  const selectedKind = String(queryParams.kind || "").trim().toLowerCase();
  const currentState: MarketConsoleState = {
    ...defaults,
    source_id: String(queryParams.source_id || defaults.source_id).trim() || defaults.source_id,
    kind: (
      [
        "plugin_package",
        "payment_package",
        "email_template",
        "landing_page_template",
        "invoice_template",
        "auth_branding_template",
        "page_rule_pack",
      ] as const
    ).includes(selectedKind as SupportedMarketKind)
      ? (selectedKind as SupportedMarketKind)
      : defaults.kind,
    channel: String(queryParams.channel || defaults.channel).trim() || defaults.channel,
    workflow_stage:
      String(queryParams.workflow_stage || defaults.workflow_stage).trim() || defaults.workflow_stage,
    email_key: String(queryParams.email_key || defaults.email_key).trim() || defaults.email_key,
    landing_slug:
      String(queryParams.landing_slug || defaults.landing_slug).trim() || defaults.landing_slug,
  };
  const pluginPackageDefaults = buildPackageFormState(defaults, "plugin_package");
  const paymentPackageDefaults = buildPackageFormState(defaults, "payment_package");
  const emailTemplateDefaults = buildTemplateFormState(defaults, "email_template");
  const landingTemplateDefaults = buildTemplateFormState(defaults, "landing_page_template");
  const invoiceTemplateDefaults = buildTemplateFormState(defaults, "invoice_template");
  const authBrandingTemplateDefaults = buildTemplateFormState(defaults, "auth_branding_template");
  const pageRulePackDefaults = buildTemplateFormState(defaults, "page_rule_pack");
  const internalFieldHidden = allOf({ field: "workflow_stage", equals: "__hidden__" });
  const packageSourceContext = allOf(
    stageIn("source", "catalog"),
    { field: "task_id", falsy: true },
    { field: "name", falsy: true }
  );
  const packageArtifactContext = allOf(
    stageIn("artifact", "catalog"),
    { field: "task_id", falsy: true },
    { field: "name", truthy: true },
    { field: "version", falsy: true }
  );
  const packageReleaseContext = allOf(
    stageIn("release", "artifact", "history", "rollback"),
    { field: "task_id", falsy: true },
    { field: "name", truthy: true },
    { field: "version", truthy: true }
  );
  const packagePreviewContext = allOf(
    stageIn("artifact", "release", "history"),
    { field: "task_id", falsy: true },
    { field: "name", truthy: true }
  );
  const packageInstallContext = allOf(
    stageIn("preview", "release"),
    { field: "task_id", falsy: true },
    { field: "name", truthy: true }
  );
  const packageHistoryContext = allOf(
    stageIn("artifact", "release", "preview", "install", "task", "history", "rollback"),
    { field: "name", truthy: true }
  );
  const packageTaskContext = allOf(
    stageIn("task", "install"),
    { field: "kind", equals: "plugin_package" },
    { field: "task_id", truthy: true }
  );
  const packageRollbackContext = allOf(
    stageIn("history", "task", "rollback"),
    { field: "name", truthy: true },
    { field: "version", truthy: true }
  );
  const templateSourceContext = allOf(stageIn("source", "catalog"), { field: "name", falsy: true });
  const templateArtifactContext = allOf(
    stageIn("artifact", "catalog"),
    { field: "name", truthy: true },
    { field: "version", falsy: true }
  );
  const templateReleaseContext = allOf(
    stageIn("release", "artifact", "history", "rollback"),
    { field: "name", truthy: true },
    { field: "version", truthy: true }
  );
  const templatePreviewContext = allOf(
    stageIn("artifact", "release", "history"),
    { field: "name", truthy: true }
  );
  const templateInstallContext = allOf(
    stageIn("preview", "release"),
    { field: "name", truthy: true }
  );
  const templateHistoryContext = allOf(
    stageIn("artifact", "release", "preview", "history", "rollback", "install"),
    { field: "name", truthy: true }
  );
  const templateRollbackContext = allOf(
    stageIn("history", "rollback", "install"),
    { field: "name", truthy: true },
    { field: "version", truthy: true }
  );

  const blocks: PluginPageBlock[] = [
    buildMarketOverviewBlock(currentState),
    buildTrustedWorkspaceBlock(currentState),
  ];
  const packageWorkspace = markActionFormAsTrustedFallback(buildPluginActionFormBlock({
      title: t("frontend.packageWorkspace.title"),
      initial: pluginPackageDefaults as unknown as Record<string, unknown>,
      autoload: true,
      autoload_include_fields: false,
      presets: [
        {
          key: "plugin-package",
          label: t("frontend.packageWorkspace.presets.plugin.label"),
          description: t("frontend.packageWorkspace.presets.plugin.description"),
          values: pluginPackageDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "payment-package",
          label: t("frontend.packageWorkspace.presets.payment.label"),
          description: t("frontend.packageWorkspace.presets.payment.description"),
          values: paymentPackageDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "task-inspect",
          label: t("frontend.packageWorkspace.presets.task.label"),
          description: t("frontend.packageWorkspace.presets.task.description"),
          values: {
            ...pluginPackageDefaults,
            workflow_stage: "task",
          } as unknown as Record<string, unknown>,
        },
      ],
      remember_recent: true,
      recent_key: "market.package",
      recent_title: t("frontend.packageWorkspace.recentTitle"),
      recent_limit: 6,
      recent_label_fields: ["kind", "source_id", "name", "version", "task_id"],
      load: "market.package.load",
      loadLabel: t("frontend.packageWorkspace.loadLabel"),
      loadVisibleWhen: stageIn("source", "catalog"),
      reset: "market.package.reset",
      resetLabel: t("frontend.packageWorkspace.resetLabel"),
      resetVisibleWhen: stageIn("catalog", "artifact", "release", "preview", "task", "history", "rollback"),
      save: "market.catalog.query",
      saveLabel: t("frontend.packageWorkspace.saveLabel"),
      saveVisibleWhen: stageIn("source", "catalog"),
      extra: [
        {
          key: "source-detail",
          label: t("frontend.actions.sourceDetail"),
          action: "market.source.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["source_id"],
          visible_when: packageSourceContext,
        },
        {
          key: "artifact-detail",
          label: t("frontend.actions.artifactDetail"),
          action: "market.artifact.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packageArtifactContext,
        },
        {
          key: "release-detail",
          label: t("frontend.actions.releaseDetail"),
          action: "market.release.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packageReleaseContext,
        },
        {
          key: "preview",
          label: t("frontend.actions.previewVersion"),
          action: "market.release.preview",
          variant: "secondary",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packagePreviewContext,
        },
        {
          key: "install",
          label: t("frontend.actions.installOrImport"),
          action: "market.install.execute",
          variant: "default",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packageInstallContext,
        },
        {
          key: "task-get",
          label: t("frontend.actions.queryTask"),
          action: "market.install.task.get",
          variant: "outline",
          include_fields: true,
          required_fields: ["task_id"],
          visible_when: packageTaskContext,
        },
        {
          key: "task-list",
          label: t("frontend.actions.listTasks"),
          action: "market.install.task.list",
          variant: "outline",
          include_fields: true,
          visible_when: packageTaskContext,
        },
        {
          key: "history",
          label: t("frontend.actions.viewHistory"),
          action: "market.install.history.list",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packageHistoryContext,
        },
        {
          key: "rollback",
          label: t("frontend.actions.executeRollback"),
          action: "market.install.rollback",
          variant: "destructive",
          include_fields: true,
          required_fields: ["name"],
          visible_when: packageRollbackContext,
        },
      ],
      fields: [
        {
          key: "source_id",
          type: "string",
          label: t("frontend.fields.sourceId.label"),
          description: t("frontend.fields.sourceId.description"),
          placeholder: t("frontend.fields.sourceId.placeholder"),
          required: true,
        },
        {
          key: "kind",
          type: "select",
          label: t("frontend.packageFields.kind.label"),
          description: t("frontend.packageFields.kind.description"),
          required: true,
          options: [
            { label: t("frontend.packageFields.kind.options.pluginPackage"), value: "plugin_package" },
            { label: t("frontend.packageFields.kind.options.paymentPackage"), value: "payment_package" },
          ],
        },
        {
          key: "channel",
          type: "select",
          label: t("frontend.packageFields.channel.label"),
          description: t("frontend.packageFields.channel.description"),
          options: [
            { label: "stable", value: "stable" },
            { label: "beta", value: "beta" },
            { label: "alpha", value: "alpha" },
          ],
        },
        {
          key: "q",
          type: "string",
          label: t("frontend.packageFields.q.label"),
          description: t("frontend.packageFields.q.description"),
          placeholder: t("frontend.packageFields.q.placeholder"),
        },
        {
          key: "name",
          type: "string",
          label: t("frontend.packageFields.name.label"),
          description: t("frontend.packageFields.name.description"),
          placeholder: t("frontend.packageFields.name.placeholder"),
        },
        {
          key: "version",
          type: "string",
          label: t("frontend.packageFields.version.label"),
          description: t("frontend.packageFields.version.description"),
          placeholder: t("frontend.packageFields.version.placeholder"),
        },
        {
          key: "workflow_stage",
          type: "string",
          visible_when: internalFieldHidden,
        },
        {
          key: "task_id",
          type: "string",
          label: t("frontend.packageFields.taskId.label"),
          description: t("frontend.packageFields.taskId.description"),
          placeholder: t("frontend.packageFields.taskId.placeholder"),
          visible_when: { kind: "plugin_package" },
        },
        {
          key: "activate",
          type: "boolean",
          label: t("frontend.packageFields.activate.label"),
          description: t("frontend.packageFields.activate.description"),
          visible_when: { kind: "plugin_package" },
        },
        {
          key: "auto_start",
          type: "boolean",
          label: t("frontend.packageFields.autoStart.label"),
          description: t("frontend.packageFields.autoStart.description"),
          visible_when: { kind: "plugin_package" },
        },
        {
          key: "granted_permissions_json",
          type: "json",
          label: t("frontend.packageFields.permissions.label"),
          description: t("frontend.packageFields.permissions.description"),
          rows: 5,
          visible_when: { kind: "plugin_package" },
        },
        {
          key: "note",
          type: "textarea",
          label: t("frontend.packageFields.note.label"),
          description: t("frontend.packageFields.note.description"),
          rows: 3,
          placeholder: t("frontend.packageFields.note.placeholder"),
        },
      ],
    }));
  const templateWorkspace = markActionFormAsTrustedFallback(buildPluginActionFormBlock({
      title: t("frontend.templateWorkspace.title"),
      initial: emailTemplateDefaults as unknown as Record<string, unknown>,
      autoload: true,
      autoload_include_fields: false,
      presets: [
        {
          key: "email-template",
          label: t("frontend.templateWorkspace.presets.email.label"),
          description: t("frontend.templateWorkspace.presets.email.description"),
          values: emailTemplateDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "landing-page-template",
          label: t("frontend.templateWorkspace.presets.landing.label"),
          description: t("frontend.templateWorkspace.presets.landing.description"),
          values: landingTemplateDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "invoice-template",
          label: t("frontend.templateWorkspace.presets.invoice.label"),
          description: t("frontend.templateWorkspace.presets.invoice.description"),
          values: invoiceTemplateDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "auth-branding-template",
          label: t("frontend.templateWorkspace.presets.authBranding.label"),
          description: t("frontend.templateWorkspace.presets.authBranding.description"),
          values: authBrandingTemplateDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "page-rule-pack",
          label: t("frontend.templateWorkspace.presets.pageRulePack.label"),
          description: t("frontend.templateWorkspace.presets.pageRulePack.description"),
          values: pageRulePackDefaults as unknown as Record<string, unknown>,
        },
        {
          key: "template-history",
          label: t("frontend.templateWorkspace.presets.history.label"),
          description: t("frontend.templateWorkspace.presets.history.description"),
          values: {
            workflow_stage: "history",
            q: "",
            version: "",
            note: "",
          },
        },
      ],
      remember_recent: true,
      recent_key: "market.template",
      recent_title: t("frontend.templateWorkspace.recentTitle"),
      recent_limit: 6,
      recent_label_fields: ["kind", "source_id", "name", "email_key", "landing_slug", "version"],
      load: "market.template.load",
      loadLabel: t("frontend.templateWorkspace.loadLabel"),
      loadVisibleWhen: stageIn("source", "catalog"),
      reset: "market.template.reset",
      resetLabel: t("frontend.templateWorkspace.resetLabel"),
      resetVisibleWhen: stageIn("catalog", "artifact", "release", "preview", "history", "rollback", "install"),
      save: "market.catalog.query",
      saveLabel: t("frontend.templateWorkspace.saveLabel"),
      saveVisibleWhen: stageIn("source", "catalog"),
      extra: [
        {
          key: "source-detail",
          label: t("frontend.actions.sourceDetail"),
          action: "market.source.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["source_id"],
          visible_when: templateSourceContext,
        },
        {
          key: "artifact-detail",
          label: t("frontend.actions.artifactDetail"),
          action: "market.artifact.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templateArtifactContext,
        },
        {
          key: "release-detail",
          label: t("frontend.actions.releaseDetail"),
          action: "market.release.detail",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templateReleaseContext,
        },
        {
          key: "preview",
          label: t("frontend.actions.previewTemplate"),
          action: "market.release.preview",
          variant: "secondary",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templatePreviewContext,
        },
        {
          key: "install",
          label: t("frontend.actions.importTemplate"),
          action: "market.install.execute",
          variant: "default",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templateInstallContext,
        },
        {
          key: "history",
          label: t("frontend.actions.viewHistory"),
          action: "market.install.history.list",
          variant: "outline",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templateHistoryContext,
        },
        {
          key: "rollback",
          label: t("frontend.actions.rollbackTemplate"),
          action: "market.install.rollback",
          variant: "destructive",
          include_fields: true,
          required_fields: ["name"],
          visible_when: templateRollbackContext,
        },
      ],
      fields: [
        {
          key: "source_id",
          type: "string",
          label: t("frontend.fields.sourceId.label"),
          description: t("frontend.fields.sourceId.description"),
          placeholder: t("frontend.fields.sourceId.placeholder"),
          required: true,
        },
        {
          key: "kind",
          type: "select",
          label: t("frontend.templateFields.kind.label"),
          description: t("frontend.templateFields.kind.description"),
          required: true,
          options: [
            { label: t("frontend.templateFields.kind.options.emailTemplate"), value: "email_template" },
            { label: t("frontend.templateFields.kind.options.landingPageTemplate"), value: "landing_page_template" },
            { label: t("frontend.templateFields.kind.options.invoiceTemplate"), value: "invoice_template" },
            { label: t("frontend.templateFields.kind.options.authBrandingTemplate"), value: "auth_branding_template" },
            { label: t("frontend.templateFields.kind.options.pageRulePack"), value: "page_rule_pack" },
          ],
        },
        {
          key: "channel",
          type: "select",
          label: t("frontend.templateFields.channel.label"),
          description: t("frontend.templateFields.channel.description"),
          options: [
            { label: "stable", value: "stable" },
            { label: "beta", value: "beta" },
            { label: "alpha", value: "alpha" },
          ],
        },
        {
          key: "q",
          type: "string",
          label: t("frontend.templateFields.q.label"),
          description: t("frontend.templateFields.q.description"),
          placeholder: t("frontend.templateFields.q.placeholder"),
        },
        {
          key: "name",
          type: "string",
          label: t("frontend.templateFields.name.label"),
          description: t("frontend.templateFields.name.description"),
          placeholder: t("frontend.templateFields.name.placeholder"),
        },
        {
          key: "version",
          type: "string",
          label: t("frontend.templateFields.version.label"),
          description: t("frontend.templateFields.version.description"),
          placeholder: t("frontend.templateFields.version.placeholder"),
        },
        {
          key: "workflow_stage",
          type: "string",
          visible_when: internalFieldHidden,
        },
        {
          key: "email_key",
          type: "string",
          label: t("frontend.templateFields.emailKey.label"),
          description: t("frontend.templateFields.emailKey.description"),
          placeholder: t("frontend.templateFields.emailKey.placeholder"),
          visible_when: { kind: "email_template" },
          required_when: { kind: "email_template" },
        },
        {
          key: "landing_slug",
          type: "string",
          label: t("frontend.templateFields.landingSlug.label"),
          description: t("frontend.templateFields.landingSlug.description"),
          placeholder: t("frontend.templateFields.landingSlug.placeholder"),
          visible_when: { kind: "landing_page_template" },
          required_when: { kind: "landing_page_template" },
        },
        {
          key: "note",
          type: "textarea",
          label: t("frontend.templateFields.note.label"),
          description: t("frontend.templateFields.note.description"),
          rows: 3,
          placeholder: t("frontend.templateFields.note.placeholder"),
        },
      ],
    }));

  if (selectedKind === "plugin_package" || selectedKind === "payment_package") {
    blocks.push(packageWorkspace);
    return blocks;
  }

  if (
    selectedKind === "email_template" ||
    selectedKind === "landing_page_template" ||
    selectedKind === "invoice_template" ||
    selectedKind === "auth_branding_template" ||
    selectedKind === "page_rule_pack"
  ) {
    blocks.push(templateWorkspace);
    return blocks;
  }

  blocks.push(packageWorkspace);
  return blocks;
}
