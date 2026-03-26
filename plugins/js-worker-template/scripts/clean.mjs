import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const templateRoot = path.resolve(scriptDir, "..");

const targets = [
  path.join(templateRoot, "dist"),
  path.join(templateRoot, ".artifacts"),
  path.join(templateRoot, "js-worker-template.zip")
];

for (const target of targets) {
  await fs.rm(target, { recursive: true, force: true });
}

console.log("cleaned template artifacts");
