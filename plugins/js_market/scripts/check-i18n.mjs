import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = resolve(__dirname, "..");

const sourceFiles = [
  resolve(root, "src/index.ts"),
  resolve(root, "src/lib/frontend.ts")
];
const i18nFile = resolve(root, "src/lib/i18n.ts");

const cjkPattern = /[\u4e00-\u9fff]/;
const literalPattern = /(["'`])((?:\\.|(?!\1)[\s\S])*?)\1/gm;
const keyedPattern = /\b(?:marketMessage|t)\(\s*(["'`])([^"'`]+)\1/gm;

function collectLocalizedCandidates(content) {
  const values = new Set();
  let match;
  while ((match = literalPattern.exec(content)) !== null) {
    const text = match[2].trim();
    if (!text || !cjkPattern.test(text)) {
      continue;
    }
    if (text.includes("${")) {
      continue;
    }
    values.add(text);
  }
  return values;
}

function collectKeyedCandidates(content) {
  const keys = new Set();
  let match;
  while ((match = keyedPattern.exec(content)) !== null) {
    const key = match[2].trim();
    if (!key) {
      continue;
    }
    keys.add(key);
  }
  return keys;
}

const i18nSource = readFileSync(i18nFile, "utf8");
const missing = new Set();
const missingKeys = new Set();
const duplicatePairs = [];
const keyedLegacyOverlaps = [];

function extractConstExpression(content, name) {
  const marker = `const ${name} = `;
  const start = content.indexOf(marker);
  if (start < 0) {
    throw new Error(`Missing ${name} in i18n source`);
  }
  let index = start + marker.length;
  while (/\s/.test(content[index])) {
    index += 1;
  }
  const opener = content[index];
  const closer = opener === "{" ? "}" : opener === "[" ? "]" : null;
  if (!closer) {
    throw new Error(`Unsupported ${name} opener: ${opener}`);
  }
  let depth = 0;
  let inString = null;
  let escaped = false;
  for (; index < content.length; index += 1) {
    const ch = content[index];
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (ch === "\\") {
        escaped = true;
      } else if (ch === inString) {
        inString = null;
      }
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") {
      inString = ch;
      continue;
    }
    if (ch === opener) {
      depth += 1;
      continue;
    }
    if (ch === closer) {
      depth -= 1;
      if (depth === 0) {
        return content.slice(start + marker.length, index + 1).trim();
      }
    }
  }
  throw new Error(`Unterminated ${name} in i18n source`);
}

function collectDuplicatePairs(content, tableNames) {
  const seen = new Map();
  const duplicates = [];
  for (const tableName of tableNames) {
    const rows = runInNewContext(`(${extractConstExpression(content, tableName)})`);
    rows.forEach((pair, index) => {
      const key = JSON.stringify(pair);
      const location = `${tableName}[${index}]`;
      const existing = seen.get(key);
      if (existing) {
        duplicates.push(`${existing} | ${location} => ${key}`);
        return;
      }
      seen.set(key, location);
    });
  }
  return duplicates;
}

function collectKeyedLegacyOverlaps(content, keyedName, legacyTableNames) {
  const keyed = runInNewContext(`(${extractConstExpression(content, keyedName)})`);
  const keyedPairs = new Set(Object.values(keyed).map((pair) => JSON.stringify(pair)));
  const overlaps = [];
  for (const tableName of legacyTableNames) {
    const rows = runInNewContext(`(${extractConstExpression(content, tableName)})`);
    rows.forEach((pair, index) => {
      const key = JSON.stringify(pair);
      if (keyedPairs.has(key)) {
        overlaps.push(`${tableName}[${index}] => ${key}`);
      }
    });
  }
  return overlaps;
}

duplicatePairs.push(
  ...collectDuplicatePairs(i18nSource, [
    "PHRASE_PAIRS",
    "EXACT_TEXTS",
    "ADDITIONAL_PHRASE_PAIRS",
    "ADDITIONAL_EXACT_TEXTS"
  ])
);

keyedLegacyOverlaps.push(
  ...collectKeyedLegacyOverlaps(i18nSource, "KEYED_MESSAGES", [
    "PHRASE_PAIRS",
    "EXACT_TEXTS",
    "ADDITIONAL_PHRASE_PAIRS",
    "ADDITIONAL_EXACT_TEXTS"
  ])
);

for (const file of sourceFiles) {
  const content = readFileSync(file, "utf8");
  for (const text of collectLocalizedCandidates(content)) {
    if (!i18nSource.includes(JSON.stringify(text))) {
      missing.add(text);
    }
  }
  for (const key of collectKeyedCandidates(content)) {
    if (!i18nSource.includes(`${JSON.stringify(key)}:`)) {
      missingKeys.add(key);
    }
  }
}

if (missing.size > 0 || missingKeys.size > 0 || duplicatePairs.length > 0 || keyedLegacyOverlaps.length > 0) {
  console.error("js_market i18n check failed:");
  [...missing].sort((a, b) => a.localeCompare(b, "zh-Hans-CN")).forEach((text) => {
    console.error(`- ${text}`);
  });
  [...missingKeys].sort((a, b) => a.localeCompare(b)).forEach((key) => {
    console.error(`- [key] ${key}`);
  });
  duplicatePairs.forEach((entry) => {
    console.error(`- [duplicate] ${entry}`);
  });
  keyedLegacyOverlaps.forEach((entry) => {
    console.error(`- [keyed-overlap] ${entry}`);
  });
  process.exit(1);
}

console.log("js_market i18n check passed.");
