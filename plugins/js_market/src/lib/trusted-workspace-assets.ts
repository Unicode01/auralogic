import { getPluginFS } from "@auralogic/plugin-sdk";

import {
  TRUSTED_WORKSPACE_ASSET_VERSION,
  TRUSTED_WORKSPACE_CSS_FALLBACK,
  TRUSTED_WORKSPACE_JS_FALLBACK,
} from "./trusted-workspace-assets-fallback.generated";

export type TrustedWorkspaceAsset = {
  content_type: string;
  content: string;
};

const TRUSTED_WORKSPACE_ASSET_META: Record<
  string,
  { path: string; content_type: string; fallback: string }
> = {
  css: {
    path: "assets/trusted-workspace.css",
    content_type: "text/css",
    fallback: TRUSTED_WORKSPACE_CSS_FALLBACK,
  },
  js: {
    path: "assets/trusted-workspace.js",
    content_type: "application/javascript",
    fallback: TRUSTED_WORKSPACE_JS_FALLBACK,
  },
};

const trustedWorkspaceAssetCache = new Map<string, TrustedWorkspaceAsset>();

export { TRUSTED_WORKSPACE_ASSET_VERSION };
export { TRUSTED_WORKSPACE_CSS_FALLBACK, TRUSTED_WORKSPACE_JS_FALLBACK };

function readTrustedWorkspaceAsset(asset: string): TrustedWorkspaceAsset | null {
  const meta = TRUSTED_WORKSPACE_ASSET_META[asset];
  if (!meta) {
    return null;
  }

  const fs = getPluginFS();
  if (fs && fs.enabled && fs.exists(meta.path)) {
    return {
      content_type: meta.content_type,
      content: fs.readText(meta.path),
    };
  }

  return {
    content_type: meta.content_type,
    content: meta.fallback,
  };
}

export function getTrustedWorkspaceAsset(asset: string): TrustedWorkspaceAsset | null {
  const normalized = String(asset || "").trim().toLowerCase();
  if (!normalized) {
    return null;
  }

  const cached = trustedWorkspaceAssetCache.get(normalized);
  if (cached) {
    return cached;
  }

  const resolved = readTrustedWorkspaceAsset(normalized);
  if (!resolved || !resolved.content) {
    return null;
  }

  trustedWorkspaceAssetCache.set(normalized, resolved);
  return resolved;
}
