/**
 * AI Reference Types — Condensed domain types for AI consumption.
 *
 * This directory contains ~500 LOC of condensed type references
 * instead of ~38K LOC from src/types/proto-es/.
 *
 * AI agents: read these files INSTEAD of proto-es.
 * Developers: these are reference-only, not runtime types.
 */

// Service map (domain → client → store → methods)
export { SERVICE_MAP } from "./service-map";
export type { ServiceDomain } from "./service-map";

// Core domain types
export type { DatabaseRef } from "./database";
export { DATABASE_CLIENT, DATABASE_UPDATE_MASK_FIELDS } from "./database";

export type { ProjectRef, ProjectIamPolicyRef } from "./project";
export { PROJECT_CLIENT, PROJECT_UPDATE_MASK_FIELDS } from "./project";

export type { InstanceRef, DataSourceRef } from "./instance";
export { INSTANCE_CLIENT, INSTANCE_UPDATE_MASK_FIELDS } from "./instance";

export type { IssueRef } from "./issue";
export { ISSUE_CLIENT, ISSUE_UPDATE_MASK_FIELDS } from "./issue";

export type { PlanRef, PlanSpecRef } from "./plan";
export { PLAN_CLIENT, PLAN_UPDATE_MASK_FIELDS } from "./plan";

export type { RolloutRef, StageRef, TaskRef, TaskRunRef } from "./rollout";
export { ROLLOUT_CLIENT } from "./rollout";

export type { UserRef } from "./user";
export { USER_CLIENT, USER_UPDATE_MASK_FIELDS } from "./user";

export type { SettingRef } from "./setting";
export { SETTING_CLIENT, SETTING_UPDATE_MASK_FIELDS, SETTING_NAMES } from "./setting";

export type { QueryRequestRef, ExportRequestRef, QueryHistoryRef } from "./sql";
export { SQL_CLIENT } from "./sql";

export type { PolicyRef, MaskingPolicyRef, MaskingExceptionRef } from "./policy";
export type { PolicyType } from "./policy";
export { POLICY_CLIENT, POLICY_UPDATE_MASK_FIELDS } from "./policy";
