/**
 * AI REFERENCE — Setting Domain
 * Full type: src/types/proto-es/v1/setting_service_pb.d.ts (DO NOT read directly)
 */

/** Common setting resource names */
export const SETTING_NAMES = {
  WORKSPACE_PROFILE: "settings/bb.workspace.profile",
  WORKSPACE_APPROVAL: "settings/bb.workspace.approval",
  BRANDING_LOGO: "settings/bb.branding.logo",
  APP_IM: "settings/bb.app.im",
  WATERMARK: "settings/bb.workspace.watermark",
  DATA_CLASSIFICATION: "settings/bb.workspace.data-classification",
  SEMANTIC_TYPES: "settings/bb.workspace.semantic-types",
  SCHEMA_TEMPLATE: "settings/bb.workspace.schema-template",
  MAIL_DELIVERY: "settings/bb.plugin.mail-delivery",
  MCP_SERVER: "settings/bb.plugin.mcp-server",
  MAXIMUM_SQL_RESULT_SIZE: "settings/bb.workspace.maximum-sql-result-size",
} as const;

/** Condensed Setting fields for AI reference */
export interface SettingRef {
  /** Resource name: "settings/{setting}" */
  name: string;
  /** Setting value (varies by setting type — see proto for details) */
  value: unknown;
}

export const SETTING_CLIENT = "settingServiceClientConnect" as const;
export const SETTING_UPDATE_MASK_FIELDS = ["value"] as const;

/**
 * Service methods:
 * - getSetting({ name }) → Setting
 * - updateSetting({ setting, updateMask }) → Setting
 * - listSettings({}) → { settings }
 */
