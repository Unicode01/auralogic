export const PLUGIN_IDENTITY = "js-market";
export const PLUGIN_DISPLAY_NAME = "AuraLogic Market";
export const ADMIN_PLUGIN_PAGE_PATH = "/admin/plugin-pages/market";
export const PACKAGE_MARKET_KINDS = [
  "plugin_package",
  "payment_package"
] as const;
export const TEMPLATE_MARKET_KINDS = [
  "email_template",
  "landing_page_template",
  "invoice_template",
  "auth_branding_template",
  "page_rule_pack"
] as const;
export const SUPPORTED_MARKET_KINDS = [
  ...PACKAGE_MARKET_KINDS,
  ...TEMPLATE_MARKET_KINDS
] as const;

export type PackageMarketKind = (typeof PACKAGE_MARKET_KINDS)[number];
export type TemplateMarketKind = (typeof TEMPLATE_MARKET_KINDS)[number];
export type SupportedMarketKind = (typeof SUPPORTED_MARKET_KINDS)[number];
