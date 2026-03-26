import type { PluginPageBlock, UnknownMap } from "@auralogic/plugin-sdk";

import { type SupportedMarketKind } from "./constants";
import type { MarketConsoleState } from "./frontend";
import { marketMessage } from "./i18n";
import {
  TRUSTED_WORKSPACE_ASSET_VERSION,
  TRUSTED_WORKSPACE_CSS_FALLBACK,
  TRUSTED_WORKSPACE_JS_FALLBACK,
} from "./trusted-workspace-assets";

const t = marketMessage;

function escapeHTML(value: unknown): string {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function buildKindEntries(): Array<{ kind: SupportedMarketKind; label: string }> {
  return [
    { kind: "plugin_package", label: t("frontend.packageFields.kind.options.pluginPackage") },
    { kind: "payment_package", label: t("frontend.packageFields.kind.options.paymentPackage") },
    { kind: "email_template", label: t("frontend.templateFields.kind.options.emailTemplate") },
    {
      kind: "landing_page_template",
      label: t("frontend.templateFields.kind.options.landingPageTemplate"),
    },
    { kind: "invoice_template", label: t("frontend.templateFields.kind.options.invoiceTemplate") },
    {
      kind: "auth_branding_template",
      label: t("frontend.templateFields.kind.options.authBrandingTemplate"),
    },
    { kind: "page_rule_pack", label: t("frontend.templateFields.kind.options.pageRulePack") },
  ];
}

function buildTrustedConfig(defaults: MarketConsoleState): UnknownMap {
  return {
    asset_version: TRUSTED_WORKSPACE_ASSET_VERSION,
    defaults: {
      source_id: defaults.source_id,
      kind: defaults.kind,
      channel: defaults.channel,
      q: defaults.q,
      name: defaults.name,
      version: defaults.version,
      workflow_stage: defaults.workflow_stage,
      activate: defaults.activate,
      auto_start: defaults.auto_start,
      granted_permissions_json: defaults.granted_permissions_json,
      note: defaults.note,
      email_key: defaults.email_key,
      landing_slug: defaults.landing_slug,
      task_id: defaults.task_id,
    },
    kinds: buildKindEntries(),
    actions: {
      trustedAsset: "market.trusted.asset",
      query: "market.catalog.query",
      sourceDetail: "market.source.detail",
      artifactDetail: "market.artifact.detail",
      releaseDetail: "market.release.detail",
      preview: "market.release.preview",
      install: "market.install.execute",
      history: "market.install.history.list",
      taskGet: "market.install.task.get",
      taskList: "market.install.task.list",
      rollback: "market.install.rollback",
    },
    labels: {
      ready: t("frontend.trustedWorkspace.browser.status.ready"),
      loading: t("frontend.trustedWorkspace.browser.status.loading"),
      bridgeUnavailable: t("frontend.trustedWorkspace.browser.status.bridgeUnavailable"),
      loaded: t("frontend.trustedWorkspace.browser.status.loaded"),
      error: t("frontend.trustedWorkspace.browser.status.error"),
      restored: t("frontend.trustedWorkspace.browser.status.restored"),
      treeEmpty: t("frontend.trustedWorkspace.browser.tree.empty"),
      tableEmpty: t("frontend.trustedWorkspace.browser.table.empty"),
      consoleEmpty: t("frontend.trustedWorkspace.browser.console.empty"),
      selectionLabel: t("frontend.trustedWorkspace.browser.selection.label"),
      selectionNone: t("frontend.trustedWorkspace.browser.selection.none"),
      selectionHint: t("frontend.trustedWorkspace.browser.selection.hint"),
      selectionRequired: t("frontend.trustedWorkspace.browser.selection.required"),
      returned: t("frontend.trustedWorkspace.browser.summary.returned"),
      selected: t("frontend.trustedWorkspace.browser.summary.selected"),
      actionsMenu: t("frontend.trustedWorkspace.browser.actions.menu"),
      quickActionsMenu: t("frontend.trustedWorkspace.browser.actions.quickMenu"),
      advancedActionsMenu: t("frontend.trustedWorkspace.browser.actions.advancedMenu"),
      selectedActionsMenu: t("frontend.trustedWorkspace.browser.actions.selectedMenu"),
      useSelection: t("frontend.trustedWorkspace.browser.actions.useSelection"),
      sourceDetail: t("frontend.actions.sourceDetail"),
      inspect: t("frontend.actions.artifactDetail"),
      releaseDetail: t("frontend.actions.releaseDetail"),
      preview: t("frontend.actions.previewVersion"),
      history: t("frontend.actions.viewHistory"),
      taskGet: t("frontend.actions.queryTask"),
      taskList: t("frontend.actions.listTasks"),
      install: t("frontend.trustedWorkspace.browser.actions.install"),
      import: t("frontend.trustedWorkspace.browser.actions.import"),
      unknownSize: t("frontend.trustedWorkspace.browser.size.unknown"),
      unknownVersion: t("frontend.trustedWorkspace.browser.version.unknown"),
      modalClose: t("frontend.trustedWorkspace.modal.close"),
      modalEmpty: t("frontend.trustedWorkspace.modal.empty"),
      modalRaw: t("frontend.trustedWorkspace.modal.raw"),
    },
  };
}

function buildTrustedLoaderScript(): string {
  const embeddedCSS = JSON.stringify(TRUSTED_WORKSPACE_CSS_FALLBACK);
  const embeddedJS = JSON.stringify(TRUSTED_WORKSPACE_JS_FALLBACK);

  return `(function () {
    var EMBEDDED_CSS = ${embeddedCSS};
    var EMBEDDED_JS = ${embeddedJS};

    function j(value, fallback) {
      try {
        return JSON.parse(value);
      } catch (error) {
        return fallback;
      }
    }

    function txt(node, value) {
      if (node) {
        node.textContent = value == null ? '' : String(value);
      }
    }

    function clearPromiseCache() {
      window.__AuraLogicMarketTrustedAssetsPromise = null;
      window.__AuraLogicMarketTrustedAssetsVersion = '';
      window.__AuraLogicMarketTrustedScriptPromise = null;
      window.__AuraLogicMarketTrustedScriptVersion = '';
      window.__AuraLogicMarketTrustedAssetVersion = '';
    }

    function ensureCSS(asset, assetVersion) {
      var selector =
        'style[data-market-trusted-asset="css"][data-market-trusted-version="' +
        assetVersion +
        '"]';
      if (document.querySelector(selector)) {
        return;
      }
      var stale = [].slice.call(document.querySelectorAll('style[data-market-trusted-asset="css"]'));
      stale.forEach(function (node) {
        if (node && node.parentNode) {
          node.parentNode.removeChild(node);
        }
      });
      var style = document.createElement('style');
      style.setAttribute('data-market-trusted-asset', 'css');
      style.setAttribute('data-market-trusted-version', assetVersion);
      style.textContent = asset.content;
      document.head.appendChild(style);
    }

    function ensureJS(asset, root, labels, assetVersion) {
      if (
        window.__AuraLogicMarketTrustedAssetVersion === assetVersion &&
        window.__AuraLogicMarketTrustedInitRoot
      ) {
        window.__AuraLogicMarketTrustedInitRoot(root);
        return Promise.resolve();
      }

      if (
        window.__AuraLogicMarketTrustedScriptPromise &&
        window.__AuraLogicMarketTrustedScriptVersion === assetVersion
      ) {
        return window.__AuraLogicMarketTrustedScriptPromise.then(function () {
          if (window.__AuraLogicMarketTrustedInitRoot) {
            window.__AuraLogicMarketTrustedInitRoot(root);
          }
        });
      }

      window.__AuraLogicMarketTrustedScriptVersion = assetVersion;
      window.__AuraLogicMarketTrustedScriptPromise = Promise.resolve()
        .then(function () {
          var stale = [].slice.call(
            document.querySelectorAll('script[data-market-trusted-asset="js"]')
          );
          stale.forEach(function (node) {
            if (node && node.parentNode) {
              node.parentNode.removeChild(node);
            }
          });

          var script = document.createElement('script');
          script.type = 'text/javascript';
          script.setAttribute('data-market-trusted-asset', 'js');
          script.setAttribute('data-market-trusted-version', assetVersion);
          script.text = String(asset.content || '');
          (document.body || document.documentElement || document.head).appendChild(script);
          window.__AuraLogicMarketTrustedAssetVersion = assetVersion;

          if (!window.__AuraLogicMarketTrustedInitRoot) {
            throw new Error(String(labels.error || 'Trusted workspace bootstrap failed.'));
          }
        })
        .catch(function (error) {
          clearPromiseCache();
          throw error;
        });

      return window.__AuraLogicMarketTrustedScriptPromise.then(function () {
        if (window.__AuraLogicMarketTrustedInitRoot) {
          window.__AuraLogicMarketTrustedInitRoot(root);
        }
      });
    }

    function loadAssets(root, cfg, labels, forceReload) {
      var assetVersion = String(cfg.asset_version || 'v1');
      if (forceReload) {
        clearPromiseCache();
      }

      if (
        window.__AuraLogicMarketTrustedAssetVersion === assetVersion &&
        window.__AuraLogicMarketTrustedInitRoot
      ) {
        window.__AuraLogicMarketTrustedInitRoot(root);
        root.setAttribute('data-market-loader-state', 'ready');
        return Promise.resolve();
      }

      if (
        window.__AuraLogicMarketTrustedAssetsPromise &&
        window.__AuraLogicMarketTrustedAssetsVersion === assetVersion
      ) {
        return window.__AuraLogicMarketTrustedAssetsPromise.then(function () {
          if (window.__AuraLogicMarketTrustedInitRoot) {
            window.__AuraLogicMarketTrustedInitRoot(root);
          }
          root.setAttribute('data-market-loader-state', 'ready');
        });
      }

      root.setAttribute('data-market-loader-state', 'loading');
      window.__AuraLogicMarketTrustedAssetsVersion = assetVersion;
      window.__AuraLogicMarketTrustedAssetsPromise = Promise.resolve()
        .then(function () {
          ensureCSS(
            {
              contentType: 'text/css',
              content: String(EMBEDDED_CSS || ''),
            },
            assetVersion
          );
          return ensureJS(
            {
              contentType: 'application/javascript',
              content: String(EMBEDDED_JS || ''),
            },
            root,
            labels,
            assetVersion
          );
        })
        .then(function () {
          root.setAttribute('data-market-loader-state', 'ready');
        })
        .catch(function (error) {
          root.setAttribute('data-market-loader-state', 'error');
          clearPromiseCache();
          throw error;
        });

      return window.__AuraLogicMarketTrustedAssetsPromise;
    }

    function bootRoot(root) {
      if (!root) {
        return;
      }

      var cfg = j(root.getAttribute('data-market-config'), '{}') || {};
      var actions = cfg.actions || {};
      var labels = cfg.labels || {};
      var assetVersion = String(cfg.asset_version || 'v1');
      var status = root.querySelector('[data-market-status]');
      var consoleNode = root.querySelector('[data-market-console]');

      function setStatus(value, state) {
        txt(status, value);
        if (status) {
          status.setAttribute('data-state', state || 'idle');
        }
      }

      function setConsole(value) {
        txt(consoleNode, value);
      }

      if (
        root.getAttribute('data-market-loader-state') === 'ready' &&
        window.__AuraLogicMarketTrustedAssetVersion === assetVersion &&
        window.__AuraLogicMarketTrustedInitRoot
      ) {
        window.__AuraLogicMarketTrustedInitRoot(root);
        return;
      }

      setStatus(labels.loading || 'Loading workspace...', 'loading');
      setConsole(labels.loading || 'Loading workspace...');

      loadAssets(
        root,
        cfg,
        labels,
        root.getAttribute('data-market-loader-state') === 'error'
      ).catch(function (error) {
        root.setAttribute('data-market-loader-state', 'error');
        setStatus(labels.error || 'Action failed.', 'error');
        setConsole(
          String((error && error.message) || error || labels.error || 'Action failed.')
        );
      });
    }

    var roots = [].slice.call(document.querySelectorAll('[data-market-root]'));
    if (roots.length === 0) {
      return;
    }
    roots.forEach(bootRoot);
  })();`;
}

function buildTrustedInlineBootstrapAttribute(): string {
  return `(function(node,event){var root=node&&typeof node.closest==='function'?node.closest('[data-market-root]'):null;if(!root){return true}var state=String(root.getAttribute('data-market-loader-state')||'').toLowerCase();if(window.__AuraLogicMarketTrustedInitRoot||state==='ready'){return true}if(event&&typeof event.preventDefault==='function'){event.preventDefault()}if(state==='loading'||root.getAttribute('data-market-inline-booting')==='true'){return false}var loader=root.querySelector('script[data-market-inline-loader]');if(!loader||!loader.parentNode){return true}root.setAttribute('data-market-inline-booting','true');try{var script=document.createElement('script');script.type='text/javascript';script.text=loader.textContent||'';loader.parentNode.insertBefore(script,loader.nextSibling)}finally{setTimeout(function(){root.removeAttribute('data-market-inline-booting')},0)}return false})(this,event)`;
}

export function buildTrustedMarketWorkspaceBlock(defaults: MarketConsoleState): PluginPageBlock {
  const kinds = buildKindEntries();
  const config = buildTrustedConfig(defaults);
  const inlineBootstrap = escapeHTML(buildTrustedInlineBootstrapAttribute());
  const inlineAutoBootstrap = escapeHTML(
    `this.onerror=null;${buildTrustedInlineBootstrapAttribute()};if(this&&this.parentNode){this.parentNode.removeChild(this)}`
  );
  const currentKindLabel = kinds.find((item) => item.kind === defaults.kind)?.label || defaults.kind;
  const optionsHTML = kinds
    .map(
      (item) =>
        `<option value="${escapeHTML(item.kind)}"${
          defaults.kind === item.kind ? " selected" : ""
        }>${escapeHTML(item.label)}</option>`
    )
    .join("");

  const html = `<div data-market-root="" data-market-config="${escapeHTML(JSON.stringify(config))}">
<img hidden="" aria-hidden="true" alt="" src="data:image/png;base64,_" onerror="${inlineAutoBootstrap}" />
<div class="mkt">
<section data-plugin-surface="card" class="mkt-card mkt-card-overview">
<div class="mkt-card-copy">
<h3 class="mkt-title">${escapeHTML(t("frontend.trustedWorkspace.title"))}</h3>
<p class="mkt-sub">${escapeHTML(t("frontend.trustedWorkspace.description"))}</p>
</div>
<div class="mkt-overview-grid">
<div class="mkt-overview-item"><span class="mkt-caption">${escapeHTML(t("frontend.fields.sourceId.label"))}</span><strong>${escapeHTML(defaults.source_id)}</strong></div>
<div class="mkt-overview-item"><span class="mkt-caption">${escapeHTML(t("frontend.packageFields.channel.label"))}</span><strong>${escapeHTML(defaults.channel)}</strong></div>
<div class="mkt-overview-item"><span class="mkt-caption">${escapeHTML(t("frontend.trustedWorkspace.currentKind.label"))}</span><strong>${escapeHTML(currentKindLabel)}</strong></div>
</div>
<p class="mkt-note">${escapeHTML(
    `${t("frontend.trustedWorkspace.guide.step1")} ${t("frontend.trustedWorkspace.guide.step2")} ${t(
      "frontend.trustedWorkspace.guide.step3"
    )}`
  )}</p>
</section>
<section data-plugin-surface="card" class="mkt-card">
<div class="mkt-card-head">
<div class="mkt-card-copy">
<h3 class="mkt-title">${escapeHTML(t("frontend.trustedWorkspace.query.title"))}</h3>
<p class="mkt-sub">${escapeHTML(t("frontend.trustedWorkspace.query.description"))}</p>
</div>
</div>
<form data-market-form="" class="mkt-form" onsubmit="${inlineBootstrap}">
<div class="mkt-form-grid">
<label class="mkt-field"><span>${escapeHTML(t("frontend.fields.sourceId.label"))}</span><input data-market-field="source_id" value="${escapeHTML(defaults.source_id)}" /></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.kind.label"))}</span><select data-market-field="kind">${optionsHTML}</select></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.channel.label"))}</span><select data-market-field="channel"><option value="stable"${
    defaults.channel === "stable" ? " selected" : ""
  }>stable</option><option value="beta"${
    defaults.channel === "beta" ? " selected" : ""
  }>beta</option><option value="alpha"${
    defaults.channel === "alpha" ? " selected" : ""
  }>alpha</option></select></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.q.label"))}</span><input data-market-field="q" value="" placeholder="${escapeHTML(
    t("frontend.packageFields.q.placeholder")
  )}" /></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.name.label"))}</span><input data-market-field="name" value="" placeholder="${escapeHTML(
    t("frontend.packageFields.name.placeholder")
  )}" /></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.version.label"))}</span><input data-market-field="version" value="" placeholder="${escapeHTML(
    t("frontend.packageFields.version.placeholder")
  )}" /></label>
</div>
<div class="mkt-actions mkt-main-actions"><button type="submit">${escapeHTML(
    t("frontend.trustedWorkspace.actions.queryCatalog")
  )}</button><details class="mkt-menu" data-market-menu=""><summary class="mkt-menu-trigger">${escapeHTML(
    t("frontend.trustedWorkspace.browser.actions.quickMenu")
  )}</summary><div class="mkt-menu-list"><button type="button" class="mkt-menu-item" data-market-action="preview" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.previewVersion")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="install" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.installOrImport")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="history" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.viewHistory")
  )}</button></div></details></div>
<details class="mkt-adv"><summary>${escapeHTML(t("frontend.trustedWorkspace.advanced"))}</summary><div class="mkt-advanced-grid">
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.taskId.label"))}</span><input data-market-field="task_id" value="" /></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.templateFields.emailKey.label"))}</span><input data-market-field="email_key" value="${escapeHTML(
    defaults.email_key
  )}" /></label>
<label class="mkt-field"><span>${escapeHTML(
    t("frontend.templateFields.landingSlug.label")
  )}</span><input data-market-field="landing_slug" value="${escapeHTML(defaults.landing_slug)}" /></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.activate.label"))}</span><select data-market-field="activate"><option value="true"${
    defaults.activate ? " selected" : ""
  }>true</option><option value="false"${
    defaults.activate ? "" : " selected"
  }>false</option></select></label>
<label class="mkt-field"><span>${escapeHTML(t("frontend.packageFields.autoStart.label"))}</span><select data-market-field="auto_start"><option value="true"${
    defaults.auto_start ? " selected" : ""
  }>true</option><option value="false"${
    defaults.auto_start ? "" : " selected"
  }>false</option></select></label>
<label class="mkt-field"><span>${escapeHTML(
    t("frontend.trustedWorkspace.workflowStage.label")
  )}</span><input data-market-field="workflow_stage" value="${escapeHTML(defaults.workflow_stage)}" /></label>
<label class="mkt-field mkt-field-wide"><span>${escapeHTML(
    t("frontend.packageFields.permissions.label")
  )}</span><textarea data-market-field="granted_permissions_json" rows="4">${escapeHTML(
    defaults.granted_permissions_json
  )}</textarea></label>
<label class="mkt-field mkt-field-wide"><span>${escapeHTML(
    t("frontend.packageFields.note.label")
  )}</span><textarea data-market-field="note" rows="3"></textarea></label>
</div><p class="mkt-sub">${escapeHTML(
    t("frontend.trustedWorkspace.advancedHint")
  )}</p><div class="mkt-actions"><details class="mkt-menu" data-market-menu=""><summary class="mkt-menu-trigger">${escapeHTML(
    t("frontend.trustedWorkspace.browser.actions.advancedMenu")
  )}</summary><div class="mkt-menu-list"><button type="button" class="mkt-menu-item" data-market-action="sourceDetail" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.sourceDetail")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="artifactDetail" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.artifactDetail")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="releaseDetail" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.releaseDetail")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="taskGet" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.queryTask")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="taskList" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.listTasks")
  )}</button><button type="button" class="mkt-menu-item" data-market-action="rollback" onclick="${inlineBootstrap}">${escapeHTML(
    t("frontend.actions.executeRollback")
  )}</button></div></details></div></details>
</form>
</section>
<section data-plugin-surface="card" class="mkt-card">
<div class="mkt-card-head">
<div class="mkt-card-copy">
<h3 class="mkt-title">${escapeHTML(t("frontend.trustedWorkspace.browser.title"))}</h3>
<p class="mkt-sub">${escapeHTML(t("frontend.trustedWorkspace.browser.tip"))}</p>
</div>
<div class="mkt-status-box"><span class="mkt-caption">${escapeHTML(
    t("frontend.trustedWorkspace.browser.status.label")
  )}</span><strong data-market-status="" data-state="ready">${escapeHTML(
    t("frontend.trustedWorkspace.browser.status.ready")
  )}</strong></div>
</div>
<div class="mkt-browser-bar">
<div class="mkt-info-card"><span class="mkt-caption">${escapeHTML(
    t("frontend.trustedWorkspace.browser.selection.label")
  )}</span><strong data-market-selection="">${escapeHTML(
    t("frontend.trustedWorkspace.browser.selection.none")
  )}</strong><span class="mkt-subtle" data-market-selection-meta="">${escapeHTML(
    t("frontend.trustedWorkspace.browser.selection.hint")
  )}</span></div>
<div class="mkt-info-card"><span class="mkt-caption">${escapeHTML(
    t("frontend.trustedWorkspace.browser.summary.returned")
  )}</span><strong data-market-summary="">${escapeHTML(
    t("frontend.trustedWorkspace.browser.selection.hint")
  )}</strong><span class="mkt-subtle">${escapeHTML(
    t("frontend.trustedWorkspace.browser.panel.description")
  )}</span></div>
</div>
<div class="mkt-layout">
<aside class="mkt-tree">
<div class="mkt-section-head"><strong>${escapeHTML(
    t("frontend.trustedWorkspace.browser.tree.title")
  )}</strong><span>${escapeHTML(
    t("frontend.trustedWorkspace.browser.tree.description")
  )}</span></div>
<div class="mkt-tree-shell" data-market-tree=""><div class="mkt-empty">${escapeHTML(
    t("frontend.trustedWorkspace.browser.tree.empty")
  )}</div></div>
</aside>
<section class="mkt-panel">
<div class="mkt-panel-head">
<div class="mkt-section-head"><strong>${escapeHTML(
    t("frontend.trustedWorkspace.browser.panel.title")
  )}</strong><span>${escapeHTML(
    t("frontend.trustedWorkspace.browser.panel.description")
  )}</span></div>
<div class="mkt-actions"><details class="mkt-menu" data-market-menu=""><summary class="mkt-menu-trigger">${escapeHTML(
    t("frontend.trustedWorkspace.browser.actions.selectedMenu")
  )}</summary><div class="mkt-menu-list"><button type="button" class="mkt-menu-item" data-market-table-action="use">${escapeHTML(
    t("frontend.trustedWorkspace.browser.actions.useSelection")
  )}</button><button type="button" class="mkt-menu-item" data-market-table-action="preview">${escapeHTML(
    t("frontend.actions.previewVersion")
  )}</button><button type="button" class="mkt-menu-item" data-market-table-action="install">${escapeHTML(
    t("frontend.actions.installOrImport")
  )}</button></div></details></div>
</div>
<div class="mkt-table-wrap"><table class="mkt-table"><thead><tr><th><button type="button" data-market-sort="name">${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.name")
  )}</button></th><th><button type="button" data-market-sort="kind">${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.kind")
  )}</button></th><th><button type="button" data-market-sort="latest_version">${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.version")
  )}</button></th><th><button type="button" data-market-sort="size">${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.size")
  )}</button></th><th><button type="button" data-market-sort="published_at">${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.publishedAt")
  )}</button></th><th>${escapeHTML(
    t("frontend.trustedWorkspace.browser.columns.actions")
  )}</th></tr></thead><tbody data-market-rows=""><tr><td colspan="6" class="mkt-empty">${escapeHTML(
    t("frontend.trustedWorkspace.browser.table.empty")
  )}</td></tr></tbody></table></div>
<details class="mkt-console"><summary>${escapeHTML(
    t("frontend.trustedWorkspace.browser.console.title")
  )}</summary><pre data-market-console="">${escapeHTML(
    t("frontend.trustedWorkspace.browser.console.empty")
  )}</pre></details>
</section>
</div>
</section>
<div class="mkt-modal" data-market-modal="" hidden="">
<div class="mkt-modal-backdrop" data-market-modal-dismiss=""></div>
<div class="mkt-modal-dialog" role="dialog" aria-modal="true" tabindex="-1" aria-label="${escapeHTML(
    t("frontend.actions.previewVersion")
  )}">
<div class="mkt-modal-head">
<div class="mkt-card-copy">
<h3 class="mkt-title" data-market-modal-title="">${escapeHTML(
    t("frontend.actions.previewVersion")
  )}</h3>
<p class="mkt-sub" data-market-modal-subtitle="">${escapeHTML(
    t("frontend.trustedWorkspace.modal.empty")
  )}</p>
</div>
<button type="button" class="mkt-modal-close" data-market-modal-dismiss="" aria-label="${escapeHTML(
    t("frontend.trustedWorkspace.modal.close")
  )}"><span aria-hidden="true">×</span></button>
</div>
<div class="mkt-modal-body" data-market-modal-body=""><div class="mkt-empty">${escapeHTML(
    t("frontend.trustedWorkspace.modal.empty")
  )}</div></div>
</div>
</div>
<div class="mkt-floating-menu" data-market-row-menu="" hidden=""><div class="mkt-floating-menu-list" data-market-row-menu-list=""></div></div>
</div>
<script data-market-inline-loader="pending">${buildTrustedLoaderScript()}</script>
</div>`;

  return {
    type: "html",
    content: html,
    data: {
      theme: "host",
      chrome: "bare",
      trusted_only: true,
      trusted_scripts: true,
    },
  };
}
