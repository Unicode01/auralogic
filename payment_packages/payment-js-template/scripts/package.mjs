import { createWriteStream } from "node:fs";
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import yazl from "yazl";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(scriptDir, "..");
const manifestPath = path.join(packageRoot, "manifest.json");
const distEntryPath = path.join(packageRoot, "dist", "index.js");
const packageDir = path.join(packageRoot, ".artifacts", "package");
const zipPath = path.join(packageRoot, "payment-js-template.zip");

await assertFile(manifestPath);
await assertFile(distEntryPath);

await fs.rm(packageDir, { recursive: true, force: true });
await fs.mkdir(packageDir, { recursive: true });
await fs.copyFile(manifestPath, path.join(packageDir, "manifest.json"));
await fs.copyFile(distEntryPath, path.join(packageDir, "index.js"));

await zipDirectory(packageDir, zipPath);
console.log(`packaged payment template: ${path.relative(packageRoot, zipPath)}`);

async function assertFile(targetPath) {
  const stat = await fs.stat(targetPath);
  if (!stat.isFile()) {
    throw new Error(`required file is not a file: ${targetPath}`);
  }
}

async function collectFiles(rootPath) {
  const output = [];
  const entries = await fs.readdir(rootPath, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(rootPath, entry.name);
    if (entry.isDirectory()) {
      const nested = await collectFiles(fullPath);
      output.push(...nested);
      continue;
    }
    if (entry.isFile()) {
      output.push(fullPath);
    }
  }
  output.sort((a, b) => a.localeCompare(b));
  return output;
}

async function zipDirectory(sourceRoot, targetZipPath) {
  await fs.rm(targetZipPath, { force: true });
  const files = await collectFiles(sourceRoot);
  if (files.length === 0) {
    throw new Error("payment template package is empty");
  }

  const zipFile = new yazl.ZipFile();
  for (const filePath of files) {
    const relativePath = path.relative(sourceRoot, filePath).split(path.sep).join("/");
    zipFile.addFile(filePath, relativePath);
  }

  await new Promise((resolve, reject) => {
    const output = createWriteStream(targetZipPath);
    output.on("error", reject);
    output.on("close", resolve);
    zipFile.outputStream.on("error", reject);
    zipFile.outputStream.pipe(output);
    zipFile.end();
  });
}
