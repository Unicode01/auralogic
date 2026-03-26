import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");

await fs.rm(path.join(sdkRoot, "dist"), { recursive: true, force: true });
await fs.rm(path.join(sdkRoot, "dist-test"), { recursive: true, force: true });
console.log("cleaned sdk dist and dist-test");
