import { readFile } from "node:fs/promises";
import path from "node:path";

const packageRoot = path.resolve(process.cwd(), process.argv[2] || ".");
const manifestPath = path.join(packageRoot, "manifest.json");
const raw = await readFile(manifestPath, "utf8");
const manifest = JSON.parse(raw);
const issues = [];

validateStringOrLocalizedText(manifest.display_name, "display_name", true);
validateStringOrLocalizedText(manifest.description, "description", false);
validateNonEmptyString(manifest.name, "name");
validateOptionalLiteral(manifest.kind, "kind", ["payment_package"]);
validateLiteral(manifest.runtime, "runtime", ["payment_js"]);
validateAtLeastOneString([manifest.entry, manifest.address], "entry", "address");
validateNonEmptyString(manifest.version, "version");
validateSemver(manifest.manifest_version, "manifest_version");
validateSemver(manifest.protocol_version, "protocol_version");
validateSemver(manifest.min_host_protocol_version, "min_host_protocol_version");
validateSemver(manifest.max_host_protocol_version, "max_host_protocol_version");
validatePollInterval(manifest.poll_interval, "poll_interval");
validateConfigObject(manifest.config, "config");
validateConfigSchema(manifest.config_schema, "config_schema");
validateWebhooks(manifest.webhooks, "webhooks");

if (issues.length > 0) {
  const lines = issues.map((issue) => `- ${issue.path}: ${issue.reason}`);
  throw new Error(`payment package manifest validation failed\n${lines.join("\n")}`);
}

console.log(`payment package manifest validated for ${path.basename(packageRoot)}`);

function validateNonEmptyString(value, fieldPath) {
  if (typeof value !== "string" || value.trim() === "") {
    pushIssue(fieldPath, "must be a non-empty string");
  }
}

function validateLiteral(value, fieldPath, allowed) {
  const normalized = typeof value === "string" ? value.trim() : "";
  if (!allowed.includes(normalized)) {
    pushIssue(fieldPath, `must be one of ${allowed.join("/")}`);
  }
}

function validateOptionalLiteral(value, fieldPath, allowed) {
  if (value === undefined || value === null || String(value).trim() === "") {
    return;
  }
  validateLiteral(value, fieldPath, allowed);
}

function validateAtLeastOneString(values, firstFieldPath, secondFieldPath) {
  const matched = values.some((value) => typeof value === "string" && value.trim() !== "");
  if (!matched) {
    pushIssue(`${firstFieldPath}|${secondFieldPath}`, "at least one non-empty string is required");
  }
}

function validateSemver(value, fieldPath) {
  if (typeof value !== "string" || !/^\d+\.\d+\.\d+$/.test(value.trim())) {
    pushIssue(fieldPath, "must be a semantic version like 1.0.0");
  }
}

function validatePollInterval(value, fieldPath) {
  if (value === undefined || value === null || value === "") {
    return;
  }
  if (!Number.isFinite(Number(value)) || Number(value) < 0) {
    pushIssue(fieldPath, "must be a non-negative number");
  }
}

function validateConfigObject(value, fieldPath) {
  if (value === undefined || value === null) {
    return;
  }
  if (!isObject(value)) {
    pushIssue(fieldPath, "must be an object");
  }
}

function validateStringOrLocalizedText(value, fieldPath, required) {
  if (value === undefined || value === null || value === "") {
    if (required) {
      pushIssue(fieldPath, "is required");
    }
    return;
  }
  if (typeof value === "string") {
    if (value.trim() === "") {
      pushIssue(fieldPath, "must not be empty");
    }
    return;
  }
  if (!isObject(value)) {
    pushIssue(fieldPath, "must be a string or localized text object");
    return;
  }
  const entries = Object.entries(value).filter(([, item]) => typeof item === "string" && item.trim() !== "");
  if (entries.length === 0) {
    pushIssue(fieldPath, "must contain at least one localized string value");
  }
}

function validateConfigSchema(value, fieldPath) {
  if (value === undefined || value === null) {
    return;
  }
  if (!isObject(value)) {
    pushIssue(fieldPath, "must be an object");
    return;
  }
  const fields = value.fields;
  if (!Array.isArray(fields) || fields.length === 0) {
    pushIssue(`${fieldPath}.fields`, "must be a non-empty array");
    return;
  }

  const seen = new Set();
  for (let index = 0; index < fields.length; index += 1) {
    const item = fields[index];
    const itemPath = `${fieldPath}.fields[${index}]`;
    if (!isObject(item)) {
      pushIssue(itemPath, "must be an object");
      continue;
    }
    const key = typeof item.key === "string" ? item.key.trim() : "";
    if (!key) {
      pushIssue(`${itemPath}.key`, "is required");
    } else if (seen.has(key)) {
      pushIssue(`${itemPath}.key`, `duplicates ${JSON.stringify(key)}`);
    } else {
      seen.add(key);
    }

    const fieldType = normalizeFieldType(item.type);
    if (!fieldType.valid) {
      pushIssue(`${itemPath}.type`, "must be one of string/textarea/number/boolean/select/json/secret");
      continue;
    }

    validateSchemaDefault(item.default, fieldType.value, `${itemPath}.default`);
    const optionResult = validateSchemaOptions(item.options, `${itemPath}.options`);
    if (fieldType.value === "select") {
      if (!optionResult.present || optionResult.values.length === 0) {
        pushIssue(`${itemPath}.options`, "must be a non-empty array for select fields");
      } else if (Object.prototype.hasOwnProperty.call(item, "default")) {
        const defaultFingerprint = valueFingerprint(item.default);
        if (!optionResult.values.includes(defaultFingerprint)) {
          pushIssue(`${itemPath}.default`, "must match one of the select options");
        }
      }
    }
  }
}

function validateSchemaDefault(value, fieldType, fieldPath) {
  if (value === undefined || value === null) {
    return;
  }
  if ((fieldType === "string" || fieldType === "textarea" || fieldType === "secret") && typeof value !== "string") {
    pushIssue(fieldPath, "must be a string");
  }
  if (fieldType === "number" && !isNumberLike(value)) {
    pushIssue(fieldPath, "must be a number");
  }
  if (fieldType === "boolean" && typeof value !== "boolean") {
    pushIssue(fieldPath, "must be a boolean");
  }
}

function validateSchemaOptions(value, fieldPath) {
  if (value === undefined || value === null) {
    return { present: false, values: [] };
  }
  if (!Array.isArray(value)) {
    pushIssue(fieldPath, "must be an array");
    return { present: true, values: [] };
  }

  const out = [];
  for (let index = 0; index < value.length; index += 1) {
    const item = value[index];
    const itemPath = `${fieldPath}[${index}]`;
    if (item === null || item === undefined) {
      continue;
    }
    if (isObject(item)) {
      const optionValue = item.value !== undefined ? item.value : item.key;
      if (!isScalar(optionValue)) {
        pushIssue(`${itemPath}.value`, "is required");
        continue;
      }
      out.push(valueFingerprint(optionValue));
      continue;
    }
    if (!isScalar(item)) {
      pushIssue(itemPath, "must be a scalar or object");
      continue;
    }
    out.push(valueFingerprint(item));
  }
  return { present: true, values: out };
}

function validateWebhooks(value, fieldPath) {
  if (value === undefined || value === null) {
    return;
  }
  if (!Array.isArray(value)) {
    pushIssue(fieldPath, "must be an array");
    return;
  }
  const seen = new Set();
  for (let index = 0; index < value.length; index += 1) {
    const item = value[index];
    const itemPath = `${fieldPath}[${index}]`;
    if (!isObject(item)) {
      pushIssue(itemPath, "must be an object");
      continue;
    }
    const key = typeof item.key === "string" ? item.key.trim() : "";
    if (!key) {
      pushIssue(`${itemPath}.key`, "is required");
    } else if (seen.has(key)) {
      pushIssue(`${itemPath}.key`, `duplicates ${JSON.stringify(key)}`);
    } else {
      seen.add(key);
    }

    validateStringOrLocalizedText(item.description, `${itemPath}.description`, false);

    const method = normalizeWebhookMethod(item.method);
    if (!method.valid) {
      pushIssue(`${itemPath}.method`, "must be one of GET/POST/PUT/PATCH/DELETE/*");
    }
    const authMode = normalizeWebhookAuthMode(item.auth_mode);
    if (!authMode.valid) {
      pushIssue(`${itemPath}.auth_mode`, "must be one of none/query/header/hmac_sha256");
    }

    if (authMode.valid && authMode.value !== "none") {
      const secretKey = typeof item.secret_key === "string" ? item.secret_key.trim() : "";
      if (!secretKey) {
        pushIssue(`${itemPath}.secret_key`, `is required when auth_mode is ${JSON.stringify(authMode.value)}`);
      }
    }
  }
}

function normalizeFieldType(value) {
  const normalized = typeof value === "string" && value.trim() !== "" ? value.trim().toLowerCase() : "string";
  const allowed = ["string", "textarea", "number", "boolean", "select", "json", "secret"];
  return { valid: allowed.includes(normalized), value: normalized };
}

function normalizeWebhookMethod(value) {
  const normalized = typeof value === "string" && value.trim() !== "" ? value.trim().toUpperCase() : "POST";
  const allowed = ["GET", "POST", "PUT", "PATCH", "DELETE", "*", "ANY"];
  return { valid: allowed.includes(normalized), value: normalized === "ANY" ? "*" : normalized };
}

function normalizeWebhookAuthMode(value) {
  const normalized = typeof value === "string" && value.trim() !== "" ? value.trim().toLowerCase() : "none";
  const allowed = ["none", "query", "header", "hmac_sha256"];
  return { valid: allowed.includes(normalized), value: normalized };
}

function isObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isScalar(value) {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean";
}

function isNumberLike(value) {
  return typeof value === "number" || (typeof value === "string" && value.trim() !== "" && Number.isFinite(Number(value)));
}

function valueFingerprint(value) {
  return JSON.stringify(value);
}

function pushIssue(pathValue, reason) {
  issues.push({ path: pathValue, reason });
}
