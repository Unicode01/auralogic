import {
  asString,
  buildPluginAlertBlock,
  buildPluginActionFormBlock,
  buildPluginExecuteHTMLBridge,
  buildPluginExecuteTemplatePlaceholder,
  buildPluginHTMLBlock,
  buildPluginKeyValueBlock,
  buildPluginPageBootstrap,
  buildPluginTextBlock,
  prettyJSON,
  PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS,
  type PluginFrontendArea,
  type PluginFrontendExtension,
  type PluginPageBlock,
  type UnknownMap
} from "@auralogic/plugin-sdk";

import {
  ADMIN_PLUGIN_PAGE_PATH,
  DEFAULT_NOTE,
  PLUGIN_DISPLAY_NAME,
  USER_PLUGIN_PAGE_PATH
} from "./constants";

export type TemplatePageState = {
  greeting: string;
  note: string;
  payload_json: string;
};

export function buildDefaultTemplatePageState(config: UnknownMap): TemplatePageState {
  const greeting = asString(config.greeting) || "hello from template";
  return {
    greeting,
    note: DEFAULT_NOTE,
    payload_json: prettyJSON({
      source: "visual-form",
      area: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.area"],
      path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"],
      full_path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.full_path"],
      order_id: buildPluginExecuteTemplatePlaceholder("plugin.query.order_id"),
      order_no: buildPluginExecuteTemplatePlaceholder("plugin.route.orderNo"),
      greeting
    })
  };
}

export function buildBootstrapExtensions(
  area: PluginFrontendArea,
  state: TemplatePageState
): PluginFrontendExtension[] {
  const pagePath = area === "admin" ? ADMIN_PLUGIN_PAGE_PATH : USER_PLUGIN_PAGE_PATH;
  const requiredPermissions = area === "admin" ? ["system.config"] : [];

  return buildPluginPageBootstrap({
    area,
    path: pagePath,
    title: PLUGIN_DISPLAY_NAME,
    priority: area === "admin" ? 88 : 72,
    guest_visible: false,
    mobile_visible: area === "user",
    required_permissions: requiredPermissions,
    page: {
      title: PLUGIN_DISPLAY_NAME,
      description:
        "Minimal SDK example page injected by frontend.bootstrap. It demonstrates visual forms and HTML bridge calls against the plugin's own execute API.",
      blocks: buildTemplatePageBlocks(area, state)
    }
  });
}

function buildTemplatePageBlocks(
  area: PluginFrontendArea,
  state: TemplatePageState
): PluginPageBlock[] {
  const pagePath = area === "admin" ? ADMIN_PLUGIN_PAGE_PATH : USER_PLUGIN_PAGE_PATH;
  const workspaceExample = "template/context $PLUGIN_NAME $WORKSPACE_STATUS";
  const workspacePromptExample = "template/prompt";

  return [
    buildPluginAlertBlock({
      title: PLUGIN_DISPLAY_NAME,
      content:
        "This page is registered by the plugin itself. The form and HTML block below both call the route's execute API without host-specific plugin page code.",
      variant: "success"
    }),
    buildPluginKeyValueBlock({
      title: "SDK Surface",
      items: [
        { label: "Area", value: area },
        { label: "Route", value: pagePath },
        { label: "Bootstrap", value: "buildPluginPageBootstrap" },
        { label: "Visual Form", value: "buildPluginActionFormBlock" },
        { label: "HTML Bridge", value: "buildPluginExecuteHTMLBridge" },
        { label: "Typed Host Bridge", value: "Plugin.order / Plugin.user" },
        {
          label: "Workspace Commands",
          value: area === "admin" ? "template/context + template/prompt" : "admin only"
        }
      ]
    }),
    buildPluginActionFormBlock({
      title: "Native Host Lookup",
      remember_recent: true,
      recent_key: "template.host.lookup",
      recent_title: "Recent Host Lookups",
      recent_label_fields: ["order_no", "order_id", "user_email", "user_id"],
      initial: {
        order_id: buildPluginExecuteTemplatePlaceholder("plugin.query.order_id"),
        order_no: buildPluginExecuteTemplatePlaceholder("plugin.route.orderNo"),
        user_id: buildPluginExecuteTemplatePlaceholder("plugin.query.user_id"),
        user_email: buildPluginExecuteTemplatePlaceholder("plugin.query.user_email")
      },
      fields: [
        {
          key: "order_id",
          type: "string",
          label: "Order ID",
          description: "Optional. Auto-filled from ?order_id when present."
        },
        {
          key: "order_no",
          type: "string",
          label: "Order No",
          description: "Optional. Auto-filled from the route param :orderNo when present."
        },
        {
          key: "user_id",
          type: "string",
          label: "User ID",
          description: "Optional. Reads the target through Plugin.user.get."
        },
        {
          key: "user_email",
          type: "string",
          label: "User Email",
          description: "Optional. Useful when the account email is the only identifier you have."
        }
      ],
      extra: [
        {
          key: "lookup",
          label: "Run Host Lookup",
          action: "template.host.lookup",
          variant: "secondary",
          include_fields: true
        }
      ]
    }),
    buildPluginTextBlock(
      area === "admin"
        ? [
            "Open the admin plugin workspace dialog to use the template workspace console tools.",
            `Try: \`${workspaceExample}\``,
            `Interactive demo: \`${workspacePromptExample}\``
          ].join("\n")
        : "Workspace commands are available from the admin plugin workspace dialog.",
      "Workspace Quickstart"
    ),
    buildPluginActionFormBlock({
      title: "Template State",
      initial: state as unknown as Record<string, unknown>,
      autoload: true,
      autoload_include_fields: false,
      load: "template.page.get",
      save: "template.page.save",
      fields: [
        {
          key: "greeting",
          type: "string",
          label: "Greeting",
          description: "Used by exec responses and page feedback blocks."
        },
        {
          key: "note",
          type: "textarea",
          label: "Note",
          rows: 4,
          description: "Saved into Plugin.storage when storage demo is enabled."
        },
        {
          key: "payload_json",
          type: "json",
          label: "Payload JSON",
          rows: 8,
          description: "Round-tripped by the plugin execute API."
        }
      ],
      extra: [
        {
          key: "echo",
          label: "Echo Inputs",
          action: "template.echo",
          variant: "secondary",
          include_fields: true
        }
      ]
    }),
    buildPluginHTMLBlock(
      buildPluginExecuteHTMLBridge({
        action: "template.echo",
        target: `template-html-${area}`,
        intro:
          "This HTML block uses the execute bridge exported by the SDK. It relies on route metadata injected by the host, but the markup is fully authored by the plugin.",
        messageLabel: "Greeting",
        messageField: "greeting",
        messageValue: state.greeting,
        jsonField: "payload_json",
        jsonLabel: "Payload JSON",
        jsonValue: {
          source: "html-bridge",
          area: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.area"],
          path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"]
        },
        submitLabel: "Run Echo",
        quickActionLabel: "Quick Echo",
        quickAction: "template.echo",
        quickActionParams: {
          greeting: state.greeting,
          note: state.note,
          payload_json: state.payload_json
        }
      }),
      "HTML Exec Demo"
    ),
    buildPluginHTMLBlock(
      [
        "<div class=\"space-y-2 text-sm\">",
        "<div><strong>Resolved Full Path</strong></div>",
        `<div><code>${PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.full_path"]}</code></div>`,
        "<div><strong>Resolved Query / Route Params</strong></div>",
        `<div>order_id = <code>${buildPluginExecuteTemplatePlaceholder("plugin.query.order_id")}</code></div>`,
        `<div>orderNo = <code>${buildPluginExecuteTemplatePlaceholder("plugin.route.orderNo")}</code></div>`,
        `<div>query_json = <code>${PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.query_params_json"]}</code></div>`,
        `<div>route_json = <code>${PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.route_params_json"]}</code></div>`,
        "</div>"
      ].join(""),
      "Plugin Page Params",
      {
        chrome: "card",
        theme: "host"
      }
    )
  ];
}
