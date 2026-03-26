import { existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");
const distFiles = ["index.js", "index.d.ts"].map((name) => path.join(sdkRoot, "dist", name));
const sourceEntries = [
  path.join(sdkRoot, "package.json"),
  path.join(sdkRoot, "tsconfig.json"),
  path.join(sdkRoot, "src")
];

export function sdkDistNeedsBuild() {
  const missingDist = distFiles.some((filePath) => !existsSync(filePath));
  if (missingDist) {
    return true;
  }

  const distMtimeMs = Math.min(...distFiles.map((filePath) => statSync(filePath).mtimeMs));
  const sourceMtimeMs = Math.max(...sourceEntries.map((entryPath) => readLatestMtimeMs(entryPath)));
  return sourceMtimeMs > distMtimeMs;
}

export function ensureSDKDistBuilt() {
  if (!sdkDistNeedsBuild()) {
    console.log("plugin sdk dist is up to date");
    return;
  }

  console.log("plugin sdk dist is missing or stale, rebuilding...");

  const npmExecPath = process.env.npm_execpath;
  const command = npmExecPath
    ? process.execPath
    : process.platform === "win32"
      ? "npm.cmd"
      : "npm";
  const args = npmExecPath ? [npmExecPath, "run", "build"] : ["run", "build"];
  const result = spawnSync(command, args, {
    cwd: sdkRoot,
    stdio: "inherit"
  });

  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(
      `failed to build plugin sdk for dependent plugin workflow (exit ${result.status ?? "unknown"})`
    );
  }
}

function readLatestMtimeMs(entryPath) {
  if (!existsSync(entryPath)) {
    return 0;
  }
  const stat = statSync(entryPath);
  if (!stat.isDirectory()) {
    return stat.mtimeMs;
  }

  let latest = stat.mtimeMs;
  for (const entry of readdirSync(entryPath)) {
    const fullPath = path.join(entryPath, entry);
    const normalized = fullPath.replace(/\\/g, "/");
    if (
      normalized.includes("/node_modules/") ||
      normalized.includes("/dist/") ||
      normalized.includes("/dist-test/") ||
      normalized.includes("/.npm-cache/")
    ) {
      continue;
    }
    latest = Math.max(latest, readLatestMtimeMs(fullPath));
  }
  return latest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  ensureSDKDistBuilt();
}
