import type { CSSProperties, ReactNode } from "react";
import { Hr } from "@react-email/hr";
import { PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS } from "@auralogic/plugin-sdk";

import {
  ADMIN_PLUGIN_PAGE_PATH,
  PLUGIN_DISPLAY_NAME,
  USER_PLUGIN_PAGE_PATH
} from "./constants";
import type { DebuggerProfile } from "./types";

const { renderToStaticMarkup } = require(
  "../../node_modules/react-dom/cjs/react-dom-server-legacy.browser.production.js"
) as {
  renderToStaticMarkup: (node: ReactNode) => string;
};

const surfaceStyle: CSSProperties = {
  padding: "20px",
  border: "1px solid hsl(var(--border) / 0.8)",
  borderRadius: "14px",
  background: "linear-gradient(180deg, hsl(var(--card)) 0%, hsl(var(--muted) / 0.35) 100%)",
  color: "hsl(var(--foreground))",
  boxShadow: "0 10px 30px hsl(var(--foreground) / 0.06)"
};

const eyebrowStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  padding: "4px 10px",
  border: "1px solid hsl(var(--border))",
  borderRadius: "999px",
  background: "hsl(var(--muted) / 0.65)",
  color: "hsl(var(--muted-foreground))",
  fontSize: "11px",
  fontWeight: 700,
  letterSpacing: "0.08em",
  textTransform: "uppercase"
};

const titleStyle: CSSProperties = {
  margin: "12px 0 8px",
  color: "hsl(var(--foreground))",
  fontSize: "24px",
  lineHeight: "1.35",
  fontWeight: 700
};

const mutedTextStyle: CSSProperties = {
  margin: "0 0 16px",
  color: "hsl(var(--muted-foreground))",
  fontSize: "14px",
  lineHeight: "1.75"
};

const chipListStyle: CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  gap: "8px",
  marginBottom: "16px"
};

const chipStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  padding: "6px 10px",
  border: "1px solid hsl(var(--border))",
  borderRadius: "999px",
  background: "hsl(var(--muted) / 0.55)",
  color: "hsl(var(--foreground))",
  fontSize: "12px",
  lineHeight: "1.2",
  fontWeight: 600
};

const statsStyle: CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))",
  gap: "12px",
  marginBottom: "16px"
};

const statStyle: CSSProperties = {
  padding: "14px 16px",
  border: "1px solid hsl(var(--border) / 0.8)",
  borderRadius: "12px",
  background: "hsl(var(--muted) / 0.38)"
};

const labelStyle: CSSProperties = {
  display: "block",
  marginBottom: "6px",
  color: "hsl(var(--muted-foreground))",
  fontSize: "11px",
  lineHeight: "1.2",
  fontWeight: 600,
  letterSpacing: "0.06em",
  textTransform: "uppercase"
};

const statValueStyle: CSSProperties = {
  color: "hsl(var(--foreground))",
  fontSize: "16px",
  lineHeight: "1.5",
  fontWeight: 700,
  wordBreak: "break-word"
};

const metaStyle: CSSProperties = {
  display: "block",
  marginBottom: "16px",
  padding: "12px 14px",
  border: "1px solid hsl(var(--border) / 0.8)",
  borderRadius: "10px",
  background: "hsl(var(--muted) / 0.5)",
  color: "hsl(var(--foreground))",
  fontSize: "12px",
  lineHeight: "1.65",
  whiteSpace: "pre-wrap",
  wordBreak: "break-all"
};

const actionsStyle: CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  gap: "12px",
  alignItems: "center"
};

const primaryButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  minHeight: "40px",
  padding: "10px 14px",
  borderRadius: "10px",
  border: "1px solid transparent",
  background: "hsl(var(--primary))",
  color: "hsl(var(--primary-foreground))",
  fontSize: "14px",
  fontWeight: 600,
  lineHeight: "1.2",
  textDecoration: "none"
};

const secondaryButtonStyle: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  minHeight: "40px",
  padding: "10px 14px",
  borderRadius: "10px",
  border: "1px solid hsl(var(--border))",
  background: "hsl(var(--background))",
  color: "hsl(var(--foreground))",
  fontSize: "14px",
  fontWeight: 600,
  lineHeight: "1.2",
  textDecoration: "none"
};

const hrStyle: CSSProperties = {
  margin: "16px 0",
  borderColor: "hsl(var(--border))"
};

function renderStat(label: string, value: string | number) {
  return (
    <div data-plugin-stat="" style={statStyle}>
      <span data-plugin-label="" style={labelStyle}>{label}</span>
      <div data-plugin-stat-value="" style={statValueStyle}>{String(value)}</div>
    </div>
  );
}

function renderBadge(label: string, value: string | number) {
  return <span data-plugin-chip="" style={chipStyle}>{`${label}: ${value}`}</span>;
}

function DebuggerNpmComponentCard({
  area,
  profile
}: {
  area: "admin" | "user";
  profile: DebuggerProfile;
}) {
  const latestHook = profile.recent_events[0]?.hook || "no hook yet";
  const nextRoute = area === "admin" ? USER_PLUGIN_PAGE_PATH : ADMIN_PLUGIN_PAGE_PATH;
  const nextRouteLabel = area === "admin" ? "打开用户侧页面" : "打开管理侧页面";
  const currentPath = PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"];
  const executeURL = PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_api_url"];
  const fsValue = profile.fs.enabled ? profile.fs.usage?.file_count ?? 0 : "disabled";

  return (
    <div data-plugin-surface="card" style={surfaceStyle}>
      <span data-plugin-eyebrow="" style={eyebrowStyle}>NPM Renderer Demo</span>
      <h3 data-plugin-title="" style={titleStyle}>{`${PLUGIN_DISPLAY_NAME} 宿主风格 HTML 卡片`}</h3>
      <p data-plugin-text="muted" style={mutedTextStyle}>
        这块内容仍然是插件自己渲染出来的 HTML，但不再额外模拟一整套站点皮肤。它改为复用宿主
        的 card、border、text 视觉语义，所以看起来会更像系统原生模块，而不是页面里再嵌一个
        小页面。
      </p>

      <div data-plugin-chip-list="" style={chipListStyle}>
        {renderBadge("area", area)}
        {renderBadge("hooks", profile.enabled_hooks.length)}
        {renderBadge("granted", profile.sandbox.grantedPermissions.length)}
        {renderBadge("gaps", profile.capability_gaps.length)}
      </div>

      <div data-plugin-stats="" style={statsStyle}>
        {renderStat("Recent Events", profile.recent_events.length)}
        {renderStat("Latest Hook", latestHook)}
        {renderStat("Storage Keys", profile.storage.key_count)}
        {renderStat("Filesystem", fsValue)}
      </div>

      <Hr style={hrStyle} />

      <span data-plugin-label="" style={labelStyle}>Current Plugin Page</span>
      <code data-plugin-meta="" style={metaStyle}>{currentPath}</code>

      <span data-plugin-label="" style={labelStyle}>Execute API URL</span>
      <code data-plugin-meta="" style={metaStyle}>{executeURL}</code>

      <div data-plugin-actions="" style={actionsStyle}>
        <a href={nextRoute} data-plugin-button="primary" style={primaryButtonStyle}>
          {nextRouteLabel}
        </a>
        <a href={currentPath} data-plugin-button="secondary" style={secondaryButtonStyle}>
          刷新当前插件页
        </a>
      </div>
    </div>
  );
}

export function renderDebuggerNpmComponentCard(
  area: "admin" | "user",
  profile: DebuggerProfile
): string {
  return renderToStaticMarkup(<DebuggerNpmComponentCard area={area} profile={profile} />);
}
