import { readFile } from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { ensureSDKDistBuilt } from "./ensure-dist.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(scriptDir, "..");
const sdkDistEntry = path.join(sdkRoot, "dist", "index.js");
const pluginRoot = path.resolve(process.cwd(), process.argv[2] || ".");
const manifestPath = path.join(pluginRoot, "manifest.json");

ensureSDKDistBuilt();

const require = createRequire(import.meta.url);
const {
  inspectPluginManifestCompatibility,
  validatePluginManifestCatalog,
  validatePluginManifestSchema
} = require(sdkDistEntry);

const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
const catalogValidation = validatePluginManifestCatalog(manifest);
const schemaValidation = validatePluginManifestSchema(manifest);
const compatibility = inspectPluginManifestCompatibility(manifest);

if (!catalogValidation.valid || !schemaValidation.valid || !compatibility.compatible) {
  const lines = [];
  if (!catalogValidation.valid) {
    lines.push(
      `catalog validation failed: ${JSON.stringify(
        {
          invalid_hooks: catalogValidation.invalid_hooks,
          invalid_disabled_hooks: catalogValidation.invalid_disabled_hooks,
          invalid_allowed_frontend_slots: catalogValidation.invalid_allowed_frontend_slots,
          invalid_requested_permissions: catalogValidation.invalid_requested_permissions,
          invalid_granted_permissions: catalogValidation.invalid_granted_permissions,
          invalid_declared_permissions: catalogValidation.invalid_declared_permissions,
          requested_permissions_missing_declaration:
            catalogValidation.requested_permissions_missing_declaration,
          granted_permissions_missing_declaration:
            catalogValidation.granted_permissions_missing_declaration,
          declared_permissions_missing_request:
            catalogValidation.declared_permissions_missing_request
        },
        null,
        2
      )}`
    );
  }
  if (!schemaValidation.valid) {
    lines.push(
      `schema validation failed: ${JSON.stringify(schemaValidation.issues, null, 2)}`
    );
  }
  if (!compatibility.compatible) {
    lines.push(
      `compatibility validation failed: ${compatibility.reason_code} - ${compatibility.reason}`
    );
  }
  throw new Error(lines.join("\n\n"));
}

console.log(
  `manifest validated for ${path.basename(pluginRoot)}: catalog ok, schema ok, compatibility ${compatibility.protocol_version || compatibility.host_protocol_version}`
);
