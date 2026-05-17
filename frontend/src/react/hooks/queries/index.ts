/**
 * TanStack Query hooks barrel — all 24 domain query hooks.
 *
 * Usage:
 *   import { useDatabase, useDatabaseList, useUpdateDatabase } from "@/react/hooks/queries";
 */

// Query key factory
export { queryKeys } from "./query-keys";

// ── Core domains (P3-002) ──────────────────────────
export * from "./useDatabase";
export * from "./useProject";
export * from "./useInstance";
export * from "./useUser";
export * from "./useEnvironment";

// ── Issue / Plan / Rollout (P3-003) ────────────────
export * from "./useIssue";
export * from "./usePlan";
export * from "./useRollout";

// ── IAM / Auth ─────────────────────────────────────
export * from "./useRole";
export * from "./useGroup";
export * from "./useIDP";
export * from "./useAccessGrant";
export * from "./useServiceAccount";
export * from "./useWorkloadIdentity";

// ── Settings / Config ──────────────────────────────
export * from "./useSetting";
export * from "./useSubscription";
export * from "./usePolicy";
export * from "./useReviewConfig";

// ── SQL Editor / Worksheets ────────────────────────
export * from "./useWorksheet";
export * from "./useSchema";

// ── Audit / Changelog / Release ────────────────────
export * from "./useAuditLog";
export * from "./useChangelog";
export * from "./useRelease";
export * from "./useDatabaseGroup";
