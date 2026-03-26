import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(scriptDir, "..");

await fs.rm(path.join(packageRoot, "dist"), { recursive: true, force: true });
await fs.rm(path.join(packageRoot, ".artifacts"), { recursive: true, force: true });
console.log("cleaned payment-js-template artifacts");
