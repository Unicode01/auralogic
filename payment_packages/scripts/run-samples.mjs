import { readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const workspaceRoot = path.resolve(scriptDir, "..");
const targetScript = process.argv[2] || "validate:manifest";
const packageRoots = discoverSamplePackages(workspaceRoot);

if (packageRoots.length === 0) {
  throw new Error("no payment package samples found");
}

for (const packageRoot of packageRoots) {
  const relativeRoot = path.relative(workspaceRoot, packageRoot).split(path.sep).join("/");
  console.log(`\n==> ${relativeRoot}: npm run ${targetScript}`);
  const result = runNpmScript(packageRoot, targetScript);
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`sample ${relativeRoot} failed while running npm run ${targetScript}`);
  }
}

console.log(`\ncompleted payment package samples for script: ${targetScript}`);

function runNpmScript(cwd, scriptName) {
  if (process.platform === "win32") {
    const command = process.env.ComSpec || "cmd.exe";
    return spawnSync(command, ["/d", "/s", "/c", `npm run ${scriptName}`], {
      cwd,
      stdio: "inherit"
    });
  }
  return spawnSync("npm", ["run", scriptName], {
    cwd,
    stdio: "inherit"
  });
}

function discoverSamplePackages(rootPath) {
  return readdirSync(rootPath)
    .map((name) => path.join(rootPath, name))
    .filter((entryPath) => {
      const entryName = path.basename(entryPath);
      if (entryName === "shared" || entryName === "scripts") {
        return false;
      }
      let stats;
      try {
        stats = statSync(entryPath);
      } catch (error) {
        return false;
      }
      if (!stats.isDirectory()) {
        return false;
      }
      try {
        return statSync(path.join(entryPath, "package.json")).isFile() && statSync(path.join(entryPath, "manifest.json")).isFile();
      } catch (error) {
        return false;
      }
    })
    .sort((left, right) => left.localeCompare(right));
}
