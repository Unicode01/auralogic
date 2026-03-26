import { writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { buildCatalogSnapshot } from "./catalog-source.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");
const outputPath = path.join(sdkRoot, "src", "generated-catalog.ts");

const snapshot = buildCatalogSnapshot();
const pluginHooks = snapshot.plugin_hooks;
const hookGroups = snapshot.plugin_hook_groups;
const frontendSlots = snapshot.frontend_slots;
const pluginPermissions = snapshot.plugin_permission_keys;
const hostPermissions = snapshot.host_permission_keys;

const fileBody = [
  "// Code generated from the host hook/slot/permission registry. DO NOT EDIT.",
  "",
  renderStringArray("OFFICIAL_PLUGIN_HOOKS", pluginHooks),
  "",
  renderGroupObject("OFFICIAL_PLUGIN_HOOK_GROUPS", hookGroups),
  "",
  renderStringArray("OFFICIAL_FRONTEND_SLOTS", frontendSlots),
  "",
  renderStringArray("OFFICIAL_PLUGIN_PERMISSION_KEYS", pluginPermissions),
  "",
  renderStringArray("OFFICIAL_HOST_PERMISSION_KEYS", hostPermissions),
  ""
].join("\n");

await writeFile(outputPath, fileBody, "utf8");
console.log(
  `generated plugin catalog: hooks=${pluginHooks.length}, slots=${frontendSlots.length}, permissions=${pluginPermissions.length}, host_permissions=${hostPermissions.length}`
);

function renderStringArray(name, values) {
  const lines = values.map((value) => `  ${JSON.stringify(value)},`);
  return `export const ${name} = [\n${lines.join("\n")}\n] as const;`;
}

function renderGroupObject(name, groups) {
  const keys = Object.keys(groups);
  const body = keys
    .map((key) => {
      const values = groups[key];
      const lines = values.map((value) => `    ${JSON.stringify(value)},`);
      return `  ${key}: [\n${lines.join("\n")}\n  ],`;
    })
    .join("\n");
  return `export const ${name} = {\n${body}\n} as const;`;
}
