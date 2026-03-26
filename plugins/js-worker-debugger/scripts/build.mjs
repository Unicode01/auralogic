import path from "node:path";
import { fileURLToPath } from "node:url";
import { build, context } from "esbuild";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = path.resolve(scriptDir, "..");
const entryFile = path.join(pluginRoot, "src", "index.ts");
const outFile = path.join(pluginRoot, "dist", "index.js");
const sdkEntryFile = path.resolve(pluginRoot, "..", "sdk", "src", "index.ts");

const isWatch = process.argv.includes("--watch");
const isMinify = process.argv.includes("--minify");

const options = {
  entryPoints: [entryFile],
  outfile: outFile,
  bundle: true,
  format: "cjs",
  platform: "neutral",
  target: "es2019",
  charset: "utf8",
  minify: isMinify,
  sourcemap: false,
  treeShaking: true,
  logLevel: "info",
  legalComments: "none",
  alias: {
    "@auralogic/plugin-sdk": sdkEntryFile
  },
  packages: "bundle",
  mainFields: ["main", "module"],
  conditions: ["default"],
  define: {
    "process.env.NODE_ENV": "\"production\""
  },
  banner: {
    js: "\"use strict\";var process=typeof globalThis!==\"undefined\"&&globalThis.process?globalThis.process:{env:{NODE_ENV:\"production\"}};if(typeof globalThis!==\"undefined\"&&!globalThis.process){globalThis.process=process;}"
  }
};

if (isWatch) {
  const buildContext = await context(options);
  await buildContext.watch();
  console.log("watching js-worker-debugger bundle...");
} else {
  await build(options);
}
