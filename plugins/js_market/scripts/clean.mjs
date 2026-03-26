import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = path.resolve(scriptDir, "..");

const targets = [
  path.join(pluginRoot, "dist"),
  path.join(pluginRoot, ".artifacts"),
  path.join(pluginRoot, "js-market.zip")
];

for (const target of targets) {
  await fs.rm(target, { recursive: true, force: true });
}

console.log("cleaned js-market artifacts");
