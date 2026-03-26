import { createWriteStream } from "node:fs";
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import yazl from "yazl";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const templateRoot = path.resolve(scriptDir, "..");
const distRoot = path.join(templateRoot, "dist");
const packageRoot = path.join(templateRoot, ".artifacts", "package");
const zipPath = path.join(templateRoot, "js-worker-template.zip");

await assertFile(path.join(templateRoot, "manifest.json"));
await assertFile(path.join(distRoot, "index.js"));

await fs.rm(packageRoot, { recursive: true, force: true });
await fs.mkdir(packageRoot, { recursive: true });

await copyFile(path.join(templateRoot, "manifest.json"), path.join(packageRoot, "manifest.json"));
await copyFile(path.join(distRoot, "index.js"), path.join(packageRoot, "index.js"));

const assetsRoot = path.join(templateRoot, "assets");
if (await pathExists(assetsRoot)) {
  await copyDir(assetsRoot, path.join(packageRoot, "assets"));
}

await zipDirectoryFlat(packageRoot, zipPath);
console.log(`packaged template: ${path.relative(templateRoot, zipPath)}`);

async function pathExists(targetPath) {
  try {
    await fs.stat(targetPath);
    return true;
  } catch (error) {
    if (isCode(error, "ENOENT")) {
      return false;
    }
    throw error;
  }
}

async function assertFile(targetPath) {
  let info;
  try {
    info = await fs.stat(targetPath);
  } catch (error) {
    if (isCode(error, "ENOENT")) {
      throw new Error(`required file not found: ${toSlash(path.relative(templateRoot, targetPath))}`);
    }
    throw error;
  }
  if (!info.isFile()) {
    throw new Error(`required file is not a regular file: ${toSlash(path.relative(templateRoot, targetPath))}`);
  }
}

async function copyFile(source, target) {
  await fs.mkdir(path.dirname(target), { recursive: true });
  await fs.copyFile(source, target);
}

async function copyDir(source, target) {
  await fs.mkdir(target, { recursive: true });
  const entries = await fs.readdir(source, { withFileTypes: true });
  for (const entry of entries) {
    const sourcePath = path.join(source, entry.name);
    const targetPath = path.join(target, entry.name);
    if (entry.isDirectory()) {
      await copyDir(sourcePath, targetPath);
      continue;
    }
    if (entry.isFile()) {
      await copyFile(sourcePath, targetPath);
    }
  }
}

async function collectFilesRecursive(rootPath) {
  const output = [];
  const entries = await fs.readdir(rootPath, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(rootPath, entry.name);
    if (entry.isDirectory()) {
      const nested = await collectFilesRecursive(fullPath);
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

async function zipDirectoryFlat(sourceRoot, targetZipPath) {
  await fs.rm(targetZipPath, { force: true });
  const files = await collectFilesRecursive(sourceRoot);
  if (files.length === 0) {
    throw new Error("template package is empty");
  }

  const zipFile = new yazl.ZipFile();
  for (const filePath of files) {
    const relPath = toSlash(path.relative(sourceRoot, filePath));
    zipFile.addFile(filePath, relPath);
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

function isCode(error, code) {
  return Boolean(error && typeof error === "object" && "code" in error && error.code === code);
}

function toSlash(rawPath) {
  return rawPath.split(path.sep).join("/");
}
