(function () {
  if (typeof window === "undefined") {
    return;
  }

  if (window.__AuraLogicMarketTrustedInitAll) {
    window.__AuraLogicMarketTrustedInitAll();
    return;
  }

  function parseJSON(value, fallback) {
    try {
      return JSON.parse(value);
    } catch (error) {
      return fallback;
    }
  }

  function cloneRecord(value) {
    return parseJSON(JSON.stringify(value || {}), {});
  }

  function setText(node, value) {
    if (node) {
      node.textContent = value == null ? "" : String(value);
    }
  }

  function escapeHTML(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function formatSize(value, unknownLabel) {
    var size = Number(value || 0);
    var units = ["B", "KB", "MB", "GB"];
    var index = 0;

    if (!size || !isFinite(size) || size < 0) {
      return unknownLabel;
    }

    while (size >= 1024 && index < units.length - 1) {
      size /= 1024;
      index += 1;
    }

    return size.toFixed(index === 0 ? 0 : 1) + " " + units[index];
  }

  function formatDate(value) {
    var text = String(value || "").trim();
    if (!text) {
      return "-";
    }

    var date = new Date(text);
    return isNaN(date.getTime()) ? text : date.toLocaleString();
  }

  function compareVersion(left, right) {
    var leftParts = String(left || "").split(".");
    var rightParts = String(right || "").split(".");
    var maxLength = Math.max(leftParts.length, rightParts.length);
    var index = 0;

    for (index = 0; index < maxLength; index += 1) {
      var leftValue = parseInt(leftParts[index] || "0", 10);
      var rightValue = parseInt(rightParts[index] || "0", 10);
      if (leftValue < rightValue) {
        return -1;
      }
      if (leftValue > rightValue) {
        return 1;
      }
    }

    return 0;
  }

  function bridge() {
    return window.AuraLogicPluginPage && typeof window.AuraLogicPluginPage.execute === "function"
      ? window.AuraLogicPluginPage
      : null;
  }

  function payload(response) {
    var directBody =
      response && typeof response === "object" && !Array.isArray(response) ? response : null;
    var nestedBody =
      directBody && directBody.data && typeof directBody.data === "object" && !Array.isArray(directBody.data)
        ? directBody.data
        : null;
    var body =
      directBody && Object.prototype.hasOwnProperty.call(directBody, "success")
        ? directBody
        : nestedBody && Object.prototype.hasOwnProperty.call(nestedBody, "success")
          ? nestedBody
          : directBody || {};
    var data =
      body && body.data && typeof body.data === "object" && !Array.isArray(body.data)
        ? body.data
        : body &&
            (Object.prototype.hasOwnProperty.call(body, "browser") ||
              Object.prototype.hasOwnProperty.call(body, "blocks") ||
              Object.prototype.hasOwnProperty.call(body, "values") ||
              Object.prototype.hasOwnProperty.call(body, "message") ||
              Object.prototype.hasOwnProperty.call(body, "asset"))
          ? body
          : {};
    return { body: body, data: data };
  }

  function kindLabel(config, kind) {
    var kinds = Array.isArray(config.kinds) ? config.kinds : [];
    var index = 0;
    for (index = 0; index < kinds.length; index += 1) {
      if (String(kinds[index].kind || "") === String(kind || "")) {
        return String(kinds[index].label || kind || "");
      }
    }
    return String(kind || "");
  }

  function isRecord(value) {
    return !!value && typeof value === "object" && !Array.isArray(value);
  }

  function normalizeBrowserItem(item) {
    var record = isRecord(item) ? item : {};
    return {
      name: String(record.name || ""),
      kind: String(record.kind || ""),
      title: String(record.title || ""),
      latest_version: String(record.latest_version || record.version || ""),
      channel: String(record.channel || ""),
      publisher: String(record.publisher || ""),
      published_at: String(record.published_at || ""),
      size: Number(record.size || 0),
    };
  }

  function extractBrowserItemsFromBlocks(blocks) {
    if (!Array.isArray(blocks)) {
      return [];
    }
    var index = 0;
    for (index = 0; index < blocks.length; index += 1) {
      var block = blocks[index];
      if (!isRecord(block) || String(block.type || "") !== "table") {
        continue;
      }
      var data = isRecord(block.data) ? block.data : {};
      var rows = Array.isArray(data.rows) ? data.rows : [];
      if (rows.length === 0) {
        continue;
      }
      return rows.map(function (row) {
        var record = isRecord(row) ? row : {};
        return normalizeBrowserItem({
          name: record.name,
          kind: record.kind,
          title: record.title,
          latest_version: record.latest_version || record.version,
          channel: record.channel,
          publisher: record.publisher,
          published_at: record.published_at,
          size: record.size,
        });
      });
    }
    return [];
  }

  function extractBrowserItems(out, response) {
    var candidates = [];
    if (out && isRecord(out.data)) {
      candidates.push(out.data);
      if (isRecord(out.data.browser)) {
        candidates.push(out.data.browser);
      }
    }
    if (out && isRecord(out.body)) {
      candidates.push(out.body);
      if (isRecord(out.body.browser)) {
        candidates.push(out.body.browser);
      }
      if (isRecord(out.body.data)) {
        candidates.push(out.body.data);
        if (isRecord(out.body.data.browser)) {
          candidates.push(out.body.data.browser);
        }
      }
    }
    if (isRecord(response)) {
      candidates.push(response);
      if (isRecord(response.browser)) {
        candidates.push(response.browser);
      }
      if (isRecord(response.data)) {
        candidates.push(response.data);
        if (isRecord(response.data.browser)) {
          candidates.push(response.data.browser);
        }
        if (isRecord(response.data.data)) {
          candidates.push(response.data.data);
          if (isRecord(response.data.data.browser)) {
            candidates.push(response.data.data.browser);
          }
        }
      }
    }

    var index = 0;
    for (index = 0; index < candidates.length; index += 1) {
      var candidate = candidates[index];
      if (isRecord(candidate) && Array.isArray(candidate.items)) {
        return candidate.items.map(normalizeBrowserItem);
      }
      if (isRecord(candidate) && isRecord(candidate.browser) && Array.isArray(candidate.browser.items)) {
        return candidate.browser.items.map(normalizeBrowserItem);
      }
    }

    var blockCandidates = [];
    if (out && isRecord(out.data) && Array.isArray(out.data.blocks)) {
      blockCandidates.push(out.data.blocks);
    }
    if (out && isRecord(out.body) && Array.isArray(out.body.blocks)) {
      blockCandidates.push(out.body.blocks);
    }
    if (isRecord(response) && Array.isArray(response.blocks)) {
      blockCandidates.push(response.blocks);
    }
    if (isRecord(response) && isRecord(response.data) && Array.isArray(response.data.blocks)) {
      blockCandidates.push(response.data.blocks);
    }
    for (index = 0; index < blockCandidates.length; index += 1) {
      var items = extractBrowserItemsFromBlocks(blockCandidates[index]);
      if (items.length > 0) {
        return items;
      }
    }

    return [];
  }

  function extractResultBlocks(out, response) {
    var candidates = [];
    if (out && isRecord(out.data)) {
      candidates.push(out.data);
      if (isRecord(out.data.view)) {
        candidates.push(out.data.view);
      }
    }
    if (out && isRecord(out.body)) {
      candidates.push(out.body);
      if (isRecord(out.body.view)) {
        candidates.push(out.body.view);
      }
      if (isRecord(out.body.data)) {
        candidates.push(out.body.data);
        if (isRecord(out.body.data.view)) {
          candidates.push(out.body.data.view);
        }
      }
    }
    if (isRecord(response)) {
      candidates.push(response);
      if (isRecord(response.view)) {
        candidates.push(response.view);
      }
      if (isRecord(response.data)) {
        candidates.push(response.data);
        if (isRecord(response.data.view)) {
          candidates.push(response.data.view);
        }
        if (isRecord(response.data.data)) {
          candidates.push(response.data.data);
          if (isRecord(response.data.data.view)) {
            candidates.push(response.data.data.view);
          }
        }
      }
    }

    var index = 0;
    for (index = 0; index < candidates.length; index += 1) {
      var candidate = candidates[index];
      if (isRecord(candidate) && Array.isArray(candidate.blocks)) {
        return candidate.blocks.slice();
      }
    }

    return [];
  }

  function formatDisplayValue(value) {
    if (value === null || value === undefined || value === "") {
      return "-";
    }
    if (Array.isArray(value)) {
      return value.map(formatDisplayValue).join(", ");
    }
    if (typeof value === "object") {
      try {
        return JSON.stringify(value);
      } catch (error) {
        return String(value);
      }
    }
    return String(value);
  }

  function humanizeKey(value) {
    var text = String(value || "").replace(/_/g, " ").trim();
    if (!text) {
      return "-";
    }
    return text.charAt(0).toUpperCase() + text.slice(1);
  }

  function renderModalJSON(value) {
    try {
      return JSON.stringify(value, null, 2);
    } catch (error) {
      return String(value == null ? "" : value);
    }
  }

  function renderModalBlock(block, labels, index) {
    var record = isRecord(block) ? block : {};
    var data = isRecord(record.data) ? record.data : {};
    var type = String(record.type || "").toLowerCase();
    var title = String(record.title || "").trim();
    var content = String(record.content || "").trim();
    var titleHTML = title ? '<h4 class="mkt-modal-block-title">' + escapeHTML(title) + "</h4>" : "";
    var contentHTML = content
      ? '<p class="mkt-modal-block-text">' + escapeHTML(content) + "</p>"
      : "";

    if (type === "alert") {
      return (
        '<section class="mkt-modal-block" data-variant="' +
        escapeHTML(String(data.variant || "info")) +
        '">' +
        titleHTML +
        contentHTML +
        "</section>"
      );
    }

    if (type === "stats_grid") {
      var statsItems = Array.isArray(data.items) ? data.items : [];
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        (statsItems.length > 0
          ? '<div class="mkt-modal-stats">' +
            statsItems
              .map(function (item) {
                var stat = isRecord(item) ? item : {};
                var description = String(stat.description || "").trim();
                return (
                  '<div class="mkt-modal-stat">' +
                  '<span class="mkt-modal-stat-label">' +
                  escapeHTML(String(stat.label || stat.key || "-")) +
                  "</span>" +
                  '<strong class="mkt-modal-stat-value">' +
                  escapeHTML(formatDisplayValue(stat.value)) +
                  "</strong>" +
                  (description
                    ? '<span class="mkt-modal-block-text">' + escapeHTML(description) + "</span>"
                    : "") +
                  "</div>"
                );
              })
              .join("") +
            "</div>"
          : '<div class="mkt-empty">' + escapeHTML(labels.modalEmpty || "No detail is available.") + "</div>") +
        "</section>"
      );
    }

    if (type === "key_value") {
      var kvItems = Array.isArray(data.items) ? data.items : [];
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        (kvItems.length > 0
          ? '<div class="mkt-modal-kv">' +
            kvItems
              .map(function (item) {
                var entry = isRecord(item) ? item : {};
                var description = String(entry.description || "").trim();
                return (
                  '<div class="mkt-modal-kv-row">' +
                  '<span class="mkt-modal-kv-label">' +
                  escapeHTML(String(entry.label || entry.key || "-")) +
                  "</span>" +
                  '<div class="mkt-modal-kv-value">' +
                  escapeHTML(formatDisplayValue(entry.value)) +
                  "</div>" +
                  (description
                    ? '<span class="mkt-modal-block-text">' + escapeHTML(description) + "</span>"
                    : "") +
                  "</div>"
                );
              })
              .join("") +
            "</div>"
          : '<div class="mkt-empty">' + escapeHTML(labels.modalEmpty || "No detail is available.") + "</div>") +
        "</section>"
      );
    }

    if (type === "badge_list") {
      var badgeItems = Array.isArray(data.items) ? data.items : [];
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        (badgeItems.length > 0
          ? '<div class="mkt-modal-badges">' +
            badgeItems
              .map(function (item) {
                return '<span class="mkt-modal-badge">' + escapeHTML(formatDisplayValue(item)) + "</span>";
              })
              .join("") +
            "</div>"
          : '<div class="mkt-empty">' + escapeHTML(labels.modalEmpty || "No detail is available.") + "</div>") +
        "</section>"
      );
    }

    if (type === "link_list") {
      var links = Array.isArray(data.links) ? data.links : [];
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        (links.length > 0
          ? '<div class="mkt-modal-links">' +
            links
              .map(function (item) {
                var link = isRecord(item) ? item : {};
                var href = String(link.url || "").trim();
                var target = String(link.target || "").trim();
                return href
                  ? '<a class="mkt-modal-link" href="' +
                      escapeHTML(href) +
                      '"' +
                      (target ? ' target="' + escapeHTML(target) + '"' : "") +
                      ">" +
                      escapeHTML(String(link.label || href)) +
                      "</a>"
                  : "";
              })
              .join("") +
            "</div>"
          : '<div class="mkt-empty">' + escapeHTML(labels.modalEmpty || "No detail is available.") + "</div>") +
        "</section>"
      );
    }

    if (type === "table") {
      var columns = Array.isArray(data.columns) ? data.columns.slice() : [];
      var rows = Array.isArray(data.rows) ? data.rows : [];
      if (columns.length === 0 && rows.length > 0 && isRecord(rows[0])) {
        columns = Object.keys(rows[0]);
      }
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        (rows.length > 0 && columns.length > 0
          ? '<div class="mkt-modal-table-wrap"><table class="mkt-modal-table"><thead><tr>' +
            columns
              .map(function (column) {
                return "<th>" + escapeHTML(humanizeKey(column)) + "</th>";
              })
              .join("") +
            "</tr></thead><tbody>" +
            rows
              .map(function (row) {
                var recordRow = isRecord(row) ? row : {};
                return (
                  "<tr>" +
                  columns
                    .map(function (column) {
                      return (
                        "<td>" + escapeHTML(formatDisplayValue(recordRow[column])) + "</td>"
                      );
                    })
                    .join("") +
                  "</tr>"
                );
              })
              .join("") +
            "</tbody></table></div>"
          : '<div class="mkt-empty">' +
              escapeHTML(String(data.empty_text || labels.modalEmpty || "No detail is available.")) +
              "</div>") +
        "</section>"
      );
    }

    if (type === "json_view") {
      var summary = String(data.summary || labels.modalRaw || "Raw Response").trim();
      var jsonText = renderModalJSON(data.value);
      if (data.collapsible === true) {
        return (
          '<section class="mkt-modal-block">' +
          titleHTML +
          contentHTML +
          '<details class="mkt-modal-json"' +
          (data.collapsed === true ? "" : " open") +
          "><summary>" +
          escapeHTML(summary) +
          "</summary><pre>" +
          escapeHTML(jsonText) +
          "</pre></details></section>"
        );
      }
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        contentHTML +
        '<pre class="mkt-modal-code">' +
        escapeHTML(jsonText) +
        "</pre></section>"
      );
    }

    if (type === "text") {
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        (contentHTML || '<p class="mkt-modal-block-text">' + escapeHTML(labels.modalEmpty || "No detail is available.") + "</p>") +
        "</section>"
      );
    }

    if (type === "html") {
      return (
        '<section class="mkt-modal-block">' +
        titleHTML +
        '<pre class="mkt-modal-code">' +
        escapeHTML(String(record.content || "")) +
        "</pre></section>"
      );
    }

    return (
      '<section class="mkt-modal-block">' +
      (titleHTML || '<h4 class="mkt-modal-block-title">Block ' + String(index + 1) + "</h4>") +
      '<pre class="mkt-modal-code">' +
      escapeHTML(renderModalJSON(record)) +
      "</pre></section>"
    );
  }

  function renderModalBlocks(blocks, labels, rawBody) {
    var parts = [];
    var list = Array.isArray(blocks) ? blocks : [];

    if (list.length === 0) {
      parts.push(
        '<section class="mkt-modal-block"><div class="mkt-empty">' +
          escapeHTML(labels.modalEmpty || "No detail is available.") +
          "</div></section>"
      );
    } else {
      parts = list.map(function (block, index) {
        return renderModalBlock(block, labels, index);
      });
    }

    parts.push(
      '<section class="mkt-modal-block"><details class="mkt-modal-json"><summary>' +
        escapeHTML(labels.modalRaw || "Raw Response") +
        "</summary><pre>" +
        escapeHTML(rawBody || "{}") +
        "</pre></details></section>"
    );

    return parts.join("");
  }

  function resolveActionLabel(actionKey, labels) {
    if (actionKey === "sourceDetail") {
      return String(labels.sourceDetail || actionKey);
    }
    if (actionKey === "artifactDetail" || actionKey === "inspect") {
      return String(labels.inspect || actionKey);
    }
    if (actionKey === "releaseDetail") {
      return String(labels.releaseDetail || actionKey);
    }
    if (actionKey === "preview") {
      return String(labels.preview || actionKey);
    }
    if (actionKey === "history") {
      return String(labels.history || actionKey);
    }
    if (actionKey === "taskGet") {
      return String(labels.taskGet || actionKey);
    }
    if (actionKey === "taskList") {
      return String(labels.taskList || actionKey);
    }
    return String(actionKey || "");
  }

  function shouldOpenModalForAction(actionKey) {
    return (
      actionKey === "sourceDetail" ||
      actionKey === "artifactDetail" ||
      actionKey === "releaseDetail" ||
      actionKey === "preview" ||
      actionKey === "history" ||
      actionKey === "taskGet" ||
      actionKey === "taskList"
    );
  }

  function closestTarget(eventTarget, selector) {
    var element =
      eventTarget && eventTarget.nodeType === 1
        ? eventTarget
        : eventTarget && eventTarget.parentElement
          ? eventTarget.parentElement
          : null;

    if (element && typeof element.closest === "function") {
      return element.closest(selector);
    }

    if (eventTarget && typeof eventTarget.composedPath === "function") {
      var path = eventTarget.composedPath();
      var index = 0;
      for (index = 0; index < path.length; index += 1) {
        var item = path[index];
        if (item && item.nodeType === 1 && typeof item.closest === "function") {
          var matched = item.closest(selector);
          if (matched) {
            return matched;
          }
        }
      }
    }

    return null;
  }

  function initRoot(root) {
    if (!root || root.__AuraLogicMarketTrustedBound === true) {
      return;
    }

    root.__AuraLogicMarketTrustedBound = true;
    root.setAttribute("data-market-ready", "true");
    var pageRoot = root.closest("[data-plugin-page-root]");
    var fallbackObserver = null;
    if (pageRoot) {
      pageRoot.setAttribute("data-market-trusted-ready", "true");
    }

    function hideFallbackWorkspaces() {
      if (!pageRoot) {
        return;
      }
      [].slice
        .call(pageRoot.querySelectorAll("[data-plugin-action-form-fallback='true']"))
        .forEach(function (node) {
          if (!node || node === root) {
            return;
          }
          node.setAttribute("hidden", "hidden");
          node.setAttribute("aria-hidden", "true");
          node.style.display = "none";
        });
    }

    hideFallbackWorkspaces();
    if (
      pageRoot &&
      typeof MutationObserver !== "undefined" &&
      !pageRoot.__AuraLogicMarketFallbackObserver
    ) {
      fallbackObserver = new MutationObserver(function () {
        hideFallbackWorkspaces();
      });
      fallbackObserver.observe(pageRoot, {
        childList: true,
        subtree: true,
      });
      pageRoot.__AuraLogicMarketFallbackObserver = fallbackObserver;
    }

    var config = parseJSON(root.getAttribute("data-market-config"), "{}") || {};
    var labels = config.labels || {};
    var actions = config.actions || {};
    var defaults = cloneRecord(config.defaults || {});
    var storageKey = [
      "AuraLogicMarketTrustedWorkspace",
      window.location && window.location.pathname ? window.location.pathname : "market",
      String(defaults.source_id || ""),
      String(defaults.kind || ""),
      String(defaults.channel || ""),
      String(defaults.workflow_stage || ""),
      String(defaults.name || ""),
      String(defaults.version || ""),
      String(defaults.task_id || ""),
      String(defaults.email_key || ""),
      String(defaults.landing_slug || ""),
    ].join(":");
    var state = {
      items: [],
      selectedName: String(defaults.name || ""),
      sortKey: "published_at",
      sortDir: "desc",
    };

    var fields = [].slice.call(root.querySelectorAll("[data-market-field]"));
    var tree = root.querySelector("[data-market-tree]");
    var rows = root.querySelector("[data-market-rows]");
    var tableWrap = root.querySelector(".mkt-table-wrap");
    var status = root.querySelector("[data-market-status]");
    var summary = root.querySelector("[data-market-summary]");
    var selection = root.querySelector("[data-market-selection]");
    var selectionMeta = root.querySelector("[data-market-selection-meta]");
    var consoleNode = root.querySelector("[data-market-console]");
    var modal = root.querySelector("[data-market-modal]");
    var modalDialog = root.querySelector(".mkt-modal-dialog");
    var modalTitle = root.querySelector("[data-market-modal-title]");
    var modalSubtitle = root.querySelector("[data-market-modal-subtitle]");
    var modalBody = root.querySelector("[data-market-modal-body]");
    var rowMenu = root.querySelector("[data-market-row-menu]");
    var rowMenuList = root.querySelector("[data-market-row-menu-list]");

    function setStatus(value, tone) {
      setText(status, value);
      if (status) {
        status.setAttribute("data-state", tone || "idle");
      }
    }

    function closeMenus(exceptMenu) {
      [].slice.call(root.querySelectorAll("[data-market-menu][open]")).forEach(function (menu) {
        if (menu !== exceptMenu) {
          menu.removeAttribute("open");
        }
      });
    }

    function snapshotRect(node) {
      if (!node || typeof node.getBoundingClientRect !== "function") {
        return null;
      }
      var rect = node.getBoundingClientRect();
      return {
        top: Number(rect.top || 0),
        right: Number(rect.right || 0),
        bottom: Number(rect.bottom || 0),
        left: Number(rect.left || 0),
      };
    }

    function ensureRowMenu() {
      if (!rowMenu) {
        return null;
      }
      if (!rowMenu.__AuraLogicClickBound) {
        rowMenu.__AuraLogicClickBound = true;
        rowMenu.addEventListener("click", function (event) {
          var rowActionButton = closestTarget(event.target, "[data-market-row-action]");
          if (!rowActionButton) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          closeMenus();
          closeRowMenu();
          handleRowAction(
            rowActionButton.getAttribute("data-market-row-action"),
            rowActionButton.getAttribute("data-market-row-name")
          );
        });
      }
      rowMenu.removeAttribute("data-market-owner");
      return rowMenu;
    }

    function closeRowMenu() {
      if (!rowMenu) {
        return;
      }
      rowMenu.setAttribute("hidden", "hidden");
      rowMenu.removeAttribute("data-open-name");
      rowMenu.style.top = "";
      rowMenu.style.left = "";
      rowMenu.style.visibility = "";
      if (rowMenuList) {
        rowMenuList.innerHTML = "";
      }
    }

    function findRowMenuTrigger(name) {
      var triggers = [].slice.call(root.querySelectorAll("[data-market-row-menu-trigger]"));
      var index = 0;
      for (index = 0; index < triggers.length; index += 1) {
        if (String(triggers[index].getAttribute("data-market-row-name") || "") === String(name || "")) {
          return triggers[index];
        }
      }
      return null;
    }

    function positionRowMenu(anchorRect) {
      var menu = ensureRowMenu();
      if (!menu || !anchorRect) {
        return;
      }
      var viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0;
      var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
      var menuRect = menu.getBoundingClientRect();
      var top = Number(anchorRect.bottom || 0) + 8;
      var left = Number(anchorRect.right || 0) - menuRect.width;
      var maxLeft = Math.max(8, viewportWidth - menuRect.width - 8);
      left = Math.min(Math.max(8, left), maxLeft);
      if (top + menuRect.height > viewportHeight - 8) {
        top = Math.max(8, Number(anchorRect.top || 0) - menuRect.height - 8);
      }
      menu.style.top = String(Math.round(top)) + "px";
      menu.style.left = String(Math.round(left)) + "px";
    }

    function openRowMenu(name, trigger) {
      var item = findByName(name);
      var menu = ensureRowMenu();
      if (!item || !menu || !rowMenuList) {
        return;
      }
      var anchorRect = snapshotRect(trigger);
      useItem(item);
      render(readFields());
      saveCache();
      anchorRect = snapshotRect(findRowMenuTrigger(name)) || anchorRect;
      rowMenuList.innerHTML =
        '<button type="button" class="mkt-floating-menu-item" data-market-row-action="use" data-market-row-name="' +
        escapeHTML(item.name) +
        '">' +
        escapeHTML(labels.useSelection) +
        "</button>" +
        '<button type="button" class="mkt-floating-menu-item" data-market-row-action="inspect" data-market-row-name="' +
        escapeHTML(item.name) +
        '">' +
        escapeHTML(labels.inspect) +
        "</button>" +
        '<button type="button" class="mkt-floating-menu-item" data-market-row-action="preview" data-market-row-name="' +
        escapeHTML(item.name) +
        '">' +
        escapeHTML(labels.preview) +
        "</button>" +
        '<button type="button" class="mkt-floating-menu-item" data-market-row-action="install" data-market-row-name="' +
        escapeHTML(item.name) +
        '">' +
        escapeHTML(installLabel(readFields().kind)) +
        "</button>";
      menu.style.visibility = "hidden";
      menu.removeAttribute("hidden");
      menu.setAttribute("data-open-name", String(item.name || ""));
      requestAnimationFrame(function () {
        var latestAnchor = snapshotRect(findRowMenuTrigger(name)) || anchorRect;
        positionRowMenu(latestAnchor);
        menu.style.visibility = "";
      });
    }

    function closeModal() {
      if (!modal) {
        return;
      }
      modal.setAttribute("hidden", "hidden");
      modal.setAttribute("aria-hidden", "true");
      root.removeAttribute("data-market-modal-open");
    }

    function openModal(actionKey, item, params, out, blocks) {
      if (!modal || !modalBody) {
        return;
      }

      var bodyText = renderModalJSON(out && out.body ? out.body : {});
      var titleText = "";
      var index = 0;
      var list = Array.isArray(blocks) ? blocks : [];
      for (index = 0; index < list.length; index += 1) {
        if (isRecord(list[index]) && String(list[index].title || "").trim()) {
          titleText = String(list[index].title || "").trim();
          break;
        }
      }
      if (!titleText) {
        var subject = "";
        if (item && String(item.name || "").trim()) {
          subject =
            String(item.name || "") +
            (String(item.latest_version || params.version || "").trim()
              ? "@" + String(item.latest_version || params.version || "").trim()
              : "");
        } else if (String(params.name || "").trim()) {
          subject =
            String(params.name || "") +
            (String(params.version || "").trim() ? "@" + String(params.version || "").trim() : "");
        }
        titleText = resolveActionLabel(actionKey, labels);
        if (subject) {
          titleText += ": " + subject;
        }
      }

      setText(modalTitle, titleText || resolveActionLabel(actionKey, labels));
      setText(
        modalSubtitle,
        String((out && out.data && out.data.message) || (out && out.body && out.body.message) || "")
      );
      modalBody.innerHTML = renderModalBlocks(list, labels, bodyText);
      modal.removeAttribute("hidden");
      modal.removeAttribute("aria-hidden");
      root.setAttribute("data-market-modal-open", "true");
      if (modalDialog && typeof modalDialog.focus === "function") {
        setTimeout(function () {
          modalDialog.focus();
        }, 0);
      }
    }

    function saveCache() {
      try {
        if (!window.sessionStorage) {
          return;
        }
        window.sessionStorage.setItem(
          storageKey,
          JSON.stringify({
            fields: readFields(),
            items: state.items.slice(),
            selectedName: state.selectedName,
            sortKey: state.sortKey,
            sortDir: state.sortDir,
            console: consoleNode ? String(consoleNode.textContent || "") : "",
            saved_at: Date.now(),
          })
        );
      } catch (error) {}
    }

    function restoreCache() {
      try {
        if (!window.sessionStorage) {
          return false;
        }
        var raw = window.sessionStorage.getItem(storageKey);
        if (!raw) {
          return false;
        }
        var snapshot = parseJSON(raw, null);
        if (!isRecord(snapshot)) {
          return false;
        }
        if (isRecord(snapshot.fields)) {
          syncFields(snapshot.fields);
        }
        state.items = Array.isArray(snapshot.items)
          ? snapshot.items.map(normalizeBrowserItem)
          : [];
        state.selectedName = String(snapshot.selectedName || defaults.name || "");
        state.sortKey = String(snapshot.sortKey || "published_at");
        state.sortDir = String(snapshot.sortDir || "desc") === "asc" ? "asc" : "desc";
        root.setAttribute("data-market-browser-count", String(state.items.length));
        render(readFields());
        renderConsole(String(snapshot.console || labels.consoleEmpty));
        setStatus(labels.restored || labels.ready, "ready");
        return true;
      } catch (error) {
        return false;
      }
    }

    function readFields() {
      var next = cloneRecord(defaults);
      fields.forEach(function (field) {
        var key = String(field.getAttribute("data-market-field") || "").trim();
        if (!key) {
          return;
        }
        next[key] =
          key === "activate" || key === "auto_start" ? String(field.value) === "true" : field.value;
      });
      return next;
    }

    function writeField(key, value) {
      var field = root.querySelector('[data-market-field="' + key + '"]');
      if (field) {
        field.value = value == null ? "" : String(value);
      }
    }

    function syncFields(values) {
      if (!values || typeof values !== "object") {
        return;
      }
      Object.keys(values).forEach(function (key) {
        writeField(key, values[key]);
      });
    }

    function selectedItem() {
      var index = 0;
      for (index = 0; index < state.items.length; index += 1) {
        if (String(state.items[index].name || "") === state.selectedName) {
          return state.items[index];
        }
      }
      return null;
    }

    function ensureSelection() {
      var item = selectedItem();
      if (item) {
        return item;
      }
      if (state.items.length === 0) {
        state.selectedName = "";
        return null;
      }
      state.selectedName = String(state.items[0].name || "");
      return state.items[0];
    }

    function renderConsole(value) {
      setText(consoleNode, value || labels.consoleEmpty);
    }

    function sortedItems() {
      var items = state.items.slice();
      items.sort(function (left, right) {
        var key = state.sortKey;
        var direction = state.sortDir === "asc" ? 1 : -1;

        if (key === "size") {
          return ((Number(left.size || 0) - Number(right.size || 0)) || 0) * direction;
        }

        if (key === "published_at") {
          return (
            (new Date(left.published_at || 0).getTime() - new Date(right.published_at || 0).getTime() ||
              0) * direction
          );
        }

        if (key === "latest_version") {
          return compareVersion(left.latest_version, right.latest_version) * direction;
        }

        var leftValue = String(left[key] || "").toLowerCase();
        var rightValue = String(right[key] || "").toLowerCase();
        if (leftValue < rightValue) {
          return -1 * direction;
        }
        if (leftValue > rightValue) {
          return 1 * direction;
        }
        return 0;
      });
      return items;
    }

    function renderSelection(current) {
      var item = ensureSelection();
      if (!item) {
        setText(selection, labels.selectionNone);
        setText(selectionMeta, labels.selectionHint);
      } else {
        setText(selection, item.name || labels.selectionNone);
        setText(
          selectionMeta,
          String(current.source_id || "") +
            " / " +
            kindLabel(config, current.kind) +
            " / " +
            String(item.latest_version || labels.unknownVersion)
        );
      }

      setText(
        summary,
        String(labels.returned || "Returned") +
          ": " +
          state.items.length +
          " | " +
          String(labels.selected || "Selected") +
          ": " +
          (item ? String(item.name || "-") : "-")
      );
    }

    function renderTree(current) {
      if (!tree) {
        return;
      }

      var contextHTML =
        '<div class="mkt-tree-context">' +
        '<div class="mkt-tree-static">' +
        escapeHTML(current.source_id || "") +
        "</div>" +
        '<div class="mkt-tree-static">' +
        escapeHTML(kindLabel(config, current.kind)) +
        "</div>" +
        "</div>";

      if (state.items.length === 0) {
        tree.innerHTML = contextHTML + '<div class="mkt-empty">' + escapeHTML(labels.treeEmpty) + "</div>";
        return;
      }

      var html = contextHTML + '<div class="mkt-tree-list">';
      sortedItems().forEach(function (item) {
        var active = String(item.name || "") === state.selectedName ? "true" : "false";
        var treeMeta = [String(item.latest_version || labels.unknownVersion)];
        if (String(item.channel || "").trim()) {
          treeMeta.push(String(item.channel || "").trim());
        }
        html +=
          '<button type="button" class="mkt-tree-btn" data-market-select="' +
          escapeHTML(item.name) +
          '" data-active="' +
          active +
          '">' +
          "<strong>" +
          escapeHTML(item.name) +
          "</strong>" +
          "<span>" +
          escapeHTML(treeMeta.join(" · ")) +
          "</span>" +
          "</button>";
      });
      html += "</div>";
      tree.innerHTML = html;
    }

    function installLabel(kind) {
      return kind === "plugin_package" ? labels.install : labels.import;
    }

    function renderTable(current) {
      if (!rows) {
        return;
      }

      var items = sortedItems();
      if (items.length === 0) {
        rows.innerHTML =
          '<tr><td colspan="6" class="mkt-empty">' + escapeHTML(labels.tableEmpty) + "</td></tr>";
        return;
      }

      rows.innerHTML = items
        .map(function (item) {
          var selectedRow = String(item.name || "") === state.selectedName ? "true" : "false";
          var version = String(item.latest_version || "") || labels.unknownVersion;
          var title = String(item.title || "").trim();
          var meta = [];
          if (String(item.channel || "").trim()) {
            meta.push(
              '<span class="mkt-chip" data-tone="channel">' + escapeHTML(String(item.channel || "").trim()) + "</span>"
            );
          }
          if (String(item.publisher || "").trim()) {
            meta.push(
              '<span class="mkt-chip" data-tone="publisher">' +
                escapeHTML(String(item.publisher || "").trim()) +
                "</span>"
            );
          }
          return (
            '<tr data-market-row="' +
            escapeHTML(item.name) +
            '" data-selected="' +
            selectedRow +
            '">' +
            "<td>" +
            '<div class="mkt-name">' +
            "<strong>" +
            escapeHTML(item.name) +
            "</strong>" +
            "<span>" +
            escapeHTML(title || kindLabel(config, item.kind)) +
            "</span>" +
            (meta.length > 0 ? '<div class="mkt-name-meta">' + meta.join("") + "</div>" : "") +
            "</div>" +
            "</td>" +
            "<td>" +
            escapeHTML(kindLabel(config, item.kind)) +
            "</td>" +
            "<td>" +
            escapeHTML(version) +
            "</td>" +
            "<td>" +
            escapeHTML(formatSize(item.size, labels.unknownSize)) +
            "</td>" +
            "<td>" +
            escapeHTML(formatDate(item.published_at)) +
            "</td>" +
            "<td>" +
            '<div class="mkt-row-actions">' +
            '<button type="button" class="mkt-menu-trigger mkt-row-menu-trigger" data-market-row-menu-trigger="true" data-market-row-name="' +
            escapeHTML(item.name) +
            '">' +
            escapeHTML(labels.actionsMenu || "Actions") +
            "</button>" +
            "</div>" +
            "</td>" +
            "</tr>"
          );
        })
        .join("");
    }

    function render(current) {
      renderTree(current);
      renderTable(current);
      renderSelection(current);

      [].slice.call(root.querySelectorAll("[data-market-sort]")).forEach(function (button) {
        var key = String(button.getAttribute("data-market-sort") || "");
        button.setAttribute("data-active", key === state.sortKey ? "true" : "false");
        button.setAttribute("data-dir", key === state.sortKey ? state.sortDir : "");
      });
    }

    function useItem(item) {
      if (!item) {
        setStatus(labels.selectionRequired, "error");
        return null;
      }

      state.selectedName = String(item.name || "");
      writeField("name", item.name || "");
      writeField("version", item.latest_version || "");
      saveCache();
      return item;
    }

    function findByName(name) {
      var index = 0;
      for (index = 0; index < state.items.length; index += 1) {
        if (String(state.items[index].name || "") === String(name || "")) {
          return state.items[index];
        }
      }
      return null;
    }

    function buildParams(action, item) {
      var current = readFields();
      if ((!current.name || !String(current.name).trim()) && item) {
        current.name = item.name || "";
      }
      if ((!current.version || !String(current.version).trim()) && item) {
        current.version = item.latest_version || "";
      }
      if (
        (action === actions.preview ||
          action === actions.install ||
          action === actions.artifactDetail ||
          action === actions.releaseDetail ||
          action === actions.history ||
          action === actions.rollback) &&
        !String(current.name || "").trim()
      ) {
        throw new Error(labels.selectionRequired);
      }
      return current;
    }

    function run(actionKey, item) {
      var api = bridge();
      if (!api) {
        setStatus(labels.bridgeUnavailable, "error");
        return Promise.resolve();
      }

      var action = actions[actionKey] || actionKey;
      var params;
      try {
        params = buildParams(action, item || selectedItem());
      } catch (error) {
        renderConsole(String((error && error.message) || error || labels.selectionRequired));
        setStatus(labels.error, "error");
        return Promise.resolve();
      }

      if (!shouldOpenModalForAction(actionKey)) {
        closeModal();
      }
      setStatus(labels.loading, "loading");
      return api
        .execute({ action: action, params: params })
        .then(function (response) {
          var out = payload(response);
          var browserItems = extractBrowserItems(out, response);
          var blocks = extractResultBlocks(out, response);
          if (out.body && out.body.success === false) {
            throw new Error(String(out.body.error || labels.error));
          }

          if (out.data && out.data.values) {
            syncFields(out.data.values);
          }

          if (action === actions.query) {
            state.items = browserItems.slice();
            root.setAttribute("data-market-browser-count", String(state.items.length));
            if (params.name) {
              state.selectedName = String(params.name);
            } else if (!selectedItem() && state.items.length > 0) {
              state.selectedName = String(state.items[0].name || "");
            }
            render(readFields());
            saveCache();
          } else if (item) {
            useItem(item);
            render(readFields());
            saveCache();
          }

          saveCache();
          if (shouldOpenModalForAction(actionKey)) {
            openModal(actionKey, item, params, out, blocks);
          }
          renderConsole(JSON.stringify(out.body, null, 2));
          setStatus(String((out.data && out.data.message) || labels.loaded), "ready");
        })
        .catch(function (error) {
          if (shouldOpenModalForAction(actionKey)) {
            closeModal();
          }
          renderConsole(String((error && error.message) || error || labels.error));
          setStatus(labels.error, "error");
        });
    }

    function handleRowAction(action, name) {
      var item = findByName(name);
      if (!item) {
        renderConsole(labels.selectionRequired);
        return;
      }

      useItem(item);
      render(readFields());

      if (action === "use") {
        renderConsole(String(labels.useSelection || "Use Selection") + ": " + String(item.name || ""));
        setStatus(labels.ready, "ready");
        saveCache();
        return;
      }
      if (action === "inspect") {
        run("artifactDetail", item);
        return;
      }
      if (action === "preview") {
        run("preview", item);
        return;
      }
      if (action === "install") {
        run("install", item);
      }
    }

    root.addEventListener("submit", function (event) {
      var form = event.target;
      if (!form || !form.hasAttribute("data-market-form")) {
        return;
      }
      event.preventDefault();
      run("query", null);
    });

    root.addEventListener("click", function (event) {
      var dismissButton = closestTarget(event.target, "[data-market-modal-dismiss]");
      if (dismissButton) {
        event.preventDefault();
        closeModal();
        closeRowMenu();
        return;
      }

      if (!closestTarget(event.target, "[data-market-menu], [data-market-row-menu], [data-market-row-menu-trigger]")) {
        closeMenus();
        closeRowMenu();
      }

      var rowMenuTrigger = closestTarget(event.target, "[data-market-row-menu-trigger]");
      if (rowMenuTrigger) {
        event.preventDefault();
        closeMenus();
        var openName = rowMenu ? String(rowMenu.getAttribute("data-open-name") || "") : "";
        var triggerName = String(rowMenuTrigger.getAttribute("data-market-row-name") || "");
        if (!rowMenu || rowMenu.hasAttribute("hidden") || openName !== triggerName) {
          openRowMenu(triggerName, rowMenuTrigger);
        } else {
          closeRowMenu();
        }
        return;
      }

      var actionButton = closestTarget(event.target, "[data-market-action]");
      if (actionButton) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        run(String(actionButton.getAttribute("data-market-action") || ""), null);
        return;
      }

      var sortButton = closestTarget(event.target, "[data-market-sort]");
      if (sortButton) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        var key = String(sortButton.getAttribute("data-market-sort") || "");
        if (key) {
          state.sortDir = state.sortKey === key && state.sortDir === "asc" ? "desc" : "asc";
          state.sortKey = key;
          render(readFields());
          saveCache();
        }
        return;
      }

      var treeButton = closestTarget(event.target, "[data-market-select]");
      if (treeButton) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        handleRowAction("use", treeButton.getAttribute("data-market-select"));
        return;
      }

      var tableRow = closestTarget(event.target, "[data-market-row]");
      if (
        tableRow &&
        !closestTarget(event.target, "button, a, input, select, textarea, summary, details")
      ) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        handleRowAction("use", tableRow.getAttribute("data-market-row"));
        return;
      }

      var rowActionButton = closestTarget(event.target, "[data-market-row-action]");
      if (rowActionButton) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        handleRowAction(
          rowActionButton.getAttribute("data-market-row-action"),
          rowActionButton.getAttribute("data-market-row-name")
        );
        return;
      }

      var tableActionButton = closestTarget(event.target, "[data-market-table-action]");
      if (tableActionButton) {
        event.preventDefault();
        closeMenus();
        closeRowMenu();
        handleRowAction(
          tableActionButton.getAttribute("data-market-table-action"),
          state.selectedName
        );
      }
    });

    root.addEventListener("toggle", function (event) {
      var menu = event.target;
      if (!menu || !menu.hasAttribute || !menu.hasAttribute("data-market-menu")) {
        return;
      }
      if (menu.open) {
        closeMenus(menu);
        closeRowMenu();
      }
    });

    if (!document.__AuraLogicMarketTrustedOutsideClickBound) {
      document.__AuraLogicMarketTrustedOutsideClickBound = true;
      document.addEventListener("click", function (event) {
        if (
          closestTarget(
            event.target,
            "[data-market-root], [data-market-row-menu], [data-market-row-menu-trigger], [data-market-modal]"
          )
        ) {
          return;
        }
        [].slice.call(document.querySelectorAll("[data-market-row-menu]")).forEach(function (menu) {
          menu.setAttribute("hidden", "hidden");
          menu.removeAttribute("data-open-name");
          menu.style.top = "";
          menu.style.left = "";
          menu.style.visibility = "";
          var list = menu.querySelector("[data-market-row-menu-list]");
          if (list) {
            list.innerHTML = "";
          }
        });
      });
    }

    if (!root.__AuraLogicMarketRowMenuScrollBound) {
      root.__AuraLogicMarketRowMenuScrollBound = true;
      window.addEventListener(
        "scroll",
        function () {
          closeRowMenu();
        },
        true
      );
      window.addEventListener("resize", closeRowMenu);
      if (tableWrap) {
        tableWrap.addEventListener("scroll", closeRowMenu);
      }
    }

    if (modalDialog) {
      modalDialog.addEventListener("keydown", function (event) {
        if (String(event.key || "") === "Escape") {
          event.preventDefault();
          closeModal();
          closeRowMenu();
        }
      });
    }

    render(readFields());
    renderConsole(labels.consoleEmpty);
    closeModal();
    closeRowMenu();

    var tries = 0;
    (function waitForBridge() {
      if (bridge()) {
        if (restoreCache()) {
          return;
        }
        run("query", null);
        return;
      }

      tries += 1;
      if (tries > 40) {
        setStatus(labels.bridgeUnavailable, "error");
        return;
      }
      setTimeout(waitForBridge, 120);
    })();
  }

  function initAll() {
    [].slice.call(document.querySelectorAll("[data-market-root]")).forEach(initRoot);
  }

  window.__AuraLogicMarketTrustedInitRoot = initRoot;
  window.__AuraLogicMarketTrustedInitAll = initAll;
  initAll();
})();
