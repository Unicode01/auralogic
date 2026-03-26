import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(scriptDir, "..");
const sourcePath = path.join(packageRoot, "src", "index.js");
const distRoot = path.join(packageRoot, "dist");
const targetPath = path.join(distRoot, "index.js");

await fs.mkdir(distRoot, { recursive: true });
await fs.copyFile(sourcePath, targetPath);
console.log("built payment-js-template dist/index.js");
