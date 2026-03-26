import {
  ADMIN_PLUGIN_PAGE_PATH,
  PLUGIN_DISPLAY_NAME,
  PLUGIN_IDENTITY,
  USER_PLUGIN_PAGE_PATH
} from "./constants";
import {
  buildPluginActionButtonExtension,
  buildPluginPageBootstrap,
  type PluginFrontendExtension,
  type PluginPageBlock,
  type PluginPageSchema
} from "@auralogic/plugin-sdk";
import type { DebuggerProfile, FrontendExtension, GenericRecord } from "./types";
import { asString } from "./utils";
import { buildDebuggerDashboardBlocks, buildSlotExtensionContent } from "./view";

export function buildSlotExtensions(
  hook: string,
  payload: GenericRecord,
  profile: DebuggerProfile
): FrontendExtension[] {
  if (!profile.config.emit_frontend_extensions) {
    return [];
  }
  const slot = asString(payload.slot).toLowerCase();
  if (!slot) {
    return [];
  }
  const path = asString(payload.path) || "/";
  const area = slot.startsWith("admin.") ? "admin" : "user";
  const targetPath = area === "admin" ? ADMIN_PLUGIN_PAGE_PATH : USER_PLUGIN_PAGE_PATH;
  const isActionSlot = slot.endsWith(".actions") || slot.endsWith("_actions");
  if (isActionSlot) {
    return [
      buildPluginActionButtonExtension({
        slot,
        title: `Open ${PLUGIN_DISPLAY_NAME}`,
        href: `${targetPath}?slot=${encodeURIComponent(slot)}&path=${encodeURIComponent(path)}`,
        priority: -900,
        icon: "wrench",
        variant: "outline",
        size: "sm"
      }) as unknown as FrontendExtension
    ];
  }
  return [
    {
      type: "text",
      slot,
      title: PLUGIN_DISPLAY_NAME,
      content: buildSlotExtensionContent(profile, hook, slot, path),
      priority: -900,
      data: {
        diagnostic: true,
        source: PLUGIN_IDENTITY
      }
    }
  ];
}

export function buildBootstrapExtensions(
  payload: GenericRecord,
  profile: DebuggerProfile
): FrontendExtension[] {
  const area = asString(payload.area).toLowerCase() === "admin" ? "admin" : "user";
  const pagePath = area === "admin" ? ADMIN_PLUGIN_PAGE_PATH : USER_PLUGIN_PAGE_PATH;
  const requiredPermissions = area === "admin" ? ["system.config"] : [];
  const pageBlocks = buildDebuggerDashboardBlocks(area, profile);
  const pageSchema: PluginPageSchema = {
    title: PLUGIN_DISPLAY_NAME,
    description:
      "Unified JS Worker debugger page injected by frontend.bootstrap. It shows live hook traffic, sandbox limits, storage, filesystem, and interactive demos.",
    blocks: pageBlocks as unknown as PluginPageBlock[]
  };
  return buildPluginPageBootstrap({
    area,
    path: pagePath,
    title: PLUGIN_DISPLAY_NAME,
    priority: 95,
    guest_visible: false,
    mobile_visible: true,
    required_permissions: requiredPermissions,
    super_admin_only: area === "admin",
    page: pageSchema
  }) as unknown as FrontendExtension[];
}
