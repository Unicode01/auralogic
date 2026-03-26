import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");
const repoRoot = path.resolve(sdkRoot, "..", "..");
const syncScriptPath = path.join(scriptDir, "sync-plugin-manifest.mjs");

const samples = [
  {
    pluginRoot: "plugins/js-worker-debugger",
    profile: "debugger"
  },
  {
    pluginRoot: "plugins/js-worker-template",
    profile: "template"
  },
  {
    pluginRoot: "plugins/js_market",
    profile: "market"
  }
];

for (const sample of samples) {
  const result = spawnSync(process.execPath, [syncScriptPath, sample.pluginRoot, `--profile=${sample.profile}`, "--check"], {
    cwd: repoRoot,
    stdio: "inherit"
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(
      `sample manifest check failed for ${sample.pluginRoot} (${sample.profile}), exit ${result.status ?? "unknown"}`
    );
  }
}

console.log("all sample plugin manifests are in sync");
