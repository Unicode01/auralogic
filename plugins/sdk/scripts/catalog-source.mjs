import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");
const repoRoot = path.resolve(sdkRoot, "..", "..");
const slotPattern = /^(admin|user|auth|public)\.[a-z0-9_]+(?:\.[a-z0-9_]+)+$/;

export function readRepoText(relativePath) {
  return readFileSync(path.join(repoRoot, relativePath), "utf8");
}

export function readBackendHookRegistry() {
  const source = readRepoText("backend/internal/service/plugin_hook_registry.go");
  const matches = source.match(/"([a-z0-9_.]+)":\s+new/g) || [];
  const hooks = matches
    .map((entry) => {
      const match = entry.match(/"([a-z0-9_.]+)"/);
      return match ? match[1] : "";
    })
    .filter(Boolean);
  return Array.from(new Set(hooks)).sort();
}

export function readBackendPluginPermissions() {
  const source = readRepoText("backend/internal/service/plugin_permission_registry.go");
  const matches = Array.from(source.matchAll(/PluginPermission\w+\s+=\s+"([^"]+)"/g));
  return Array.from(new Set(matches.map((match) => match[1]).filter(Boolean))).sort();
}

export function readBackendHostPermissions() {
  const source = readRepoText("backend/internal/service/plugin_permission_registry.go");
  const matches = Array.from(source.matchAll(/PluginPermissionHost\w+\s+=\s+"([^"]+)"/g));
  return Array.from(new Set(matches.map((match) => match[1]).filter(Boolean))).sort();
}

export function walkFrontendFiles(rootDir) {
  const output = [];
  const stack = [rootDir];
  while (stack.length > 0) {
    const current = stack.pop();
    if (!current) {
      continue;
    }
    for (const entry of readdirSync(current)) {
      const fullPath = path.join(current, entry);
      const normalized = fullPath.replace(/\\/g, "/");
      if (
        normalized.includes("/node_modules/") ||
        normalized.includes("/.next/") ||
        normalized.includes("/dist/") ||
        normalized.includes("/build/") ||
        normalized.includes("/vendor/")
      ) {
        continue;
      }
      const stat = statSync(fullPath);
      if (stat.isDirectory()) {
        stack.push(fullPath);
        continue;
      }
      if (fullPath.endsWith(".ts") || fullPath.endsWith(".tsx")) {
        output.push(fullPath);
      }
    }
  }
  return output;
}

export function readFrontendSlots() {
  const frontendRoot = path.join(repoRoot, "frontend");
  const slots = new Set();

  for (const filePath of walkFrontendFiles(frontendRoot)) {
    const source = readFileSync(filePath, "utf8");

    for (const match of source.matchAll(/slot="([^"]+)"/g)) {
      addSlot(slots, match[1]);
    }
    for (const match of source.matchAll(/slot:\s*["']([^"']+)["']/g)) {
      addSlot(slots, match[1]);
    }

    for (const match of source.matchAll(/slots:\s*\[([\s\S]*?)\]/g)) {
      const block = match[1] || "";
      for (const entry of block.matchAll(/["']([^"']+)["']/g)) {
        addSlot(slots, entry[1]);
      }
    }
  }

  return Array.from(slots).sort();
}

export function groupPluginHooks(hooks) {
  const groups = {
    frontend: [],
    auth: [],
    platform: [],
    commerce: [],
    catalog: [],
    support: [],
    content: [],
    settings: []
  };

  hooks.forEach((hook) => {
    if (hook.startsWith("frontend.")) {
      groups.frontend.push(hook);
      return;
    }
    if (
      hook.startsWith("auth.") ||
      hook.startsWith("user.admin.") ||
      hook.startsWith("user.permissions.")
    ) {
      groups.auth.push(hook);
      return;
    }
    if (
      hook.startsWith("apikey.") ||
      hook.startsWith("log.email.retry.") ||
      hook.startsWith("plugin.") ||
      hook.startsWith("upload.")
    ) {
      groups.platform.push(hook);
      return;
    }
    if (
      hook.startsWith("cart.") ||
      hook.startsWith("order.") ||
      hook.startsWith("payment.") ||
      hook.startsWith("promo.") ||
      hook.startsWith("serial.")
    ) {
      groups.commerce.push(hook);
      return;
    }
    if (
      hook.startsWith("inventory.") ||
      hook.startsWith("product.") ||
      hook.startsWith("virtual_inventory.")
    ) {
      groups.catalog.push(hook);
      return;
    }
    if (hook.startsWith("ticket.")) {
      groups.support.push(hook);
      return;
    }
    if (
      hook.startsWith("announcement.") ||
      hook.startsWith("email.send.") ||
      hook.startsWith("knowledge.") ||
      hook.startsWith("marketing.") ||
      hook.startsWith("sms.send.")
    ) {
      groups.content.push(hook);
      return;
    }
    if (
      hook.startsWith("email_template.") ||
      hook.startsWith("landing_page.") ||
      hook.startsWith("settings.") ||
      hook.startsWith("template.package.")
    ) {
      groups.settings.push(hook);
    }
  });

  return groups;
}

export function buildCatalogSnapshot() {
  const pluginHooks = readBackendHookRegistry();
  return {
    plugin_hooks: pluginHooks,
    plugin_hook_groups: groupPluginHooks(pluginHooks),
    frontend_slots: readFrontendSlots(),
    plugin_permission_keys: readBackendPluginPermissions(),
    host_permission_keys: readBackendHostPermissions()
  };
}

function addSlot(target, value) {
  const normalized = String(value || "").trim();
  if (!slotPattern.test(normalized)) {
    return;
  }
  target.add(normalized);
}

if (process.argv.includes("--json")) {
  process.stdout.write(`${JSON.stringify(buildCatalogSnapshot(), null, 2)}\n`);
}
