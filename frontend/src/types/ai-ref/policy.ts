/**
 * AI REFERENCE — Policy Domain (OrgPolicy, Masking, Access Control)
 * Full type: src/types/proto-es/v1/org_policy_service_pb.d.ts (DO NOT read directly)
 */

/** Policy types used in orgPolicyServiceClientConnect */
export type PolicyType =
  | "MASKING"
  | "SLOW_QUERY"
  | "DISABLE_COPY_DATA"
  | "MASKING_EXCEPTION"
  | "RESTRICT_ISSUE_CREATION_FOR_SQL_REVIEW";

/** Condensed Policy fields for AI reference */
export interface PolicyRef {
  /** Resource name: "{parent}/policies/{type}" */
  name: string;
  /** Policy type */
  type: PolicyType;
  /** Whether the policy is enforced */
  enforce: boolean;
  /** Policy-specific payload (varies by type) */
  payload: unknown;
}

/** Masking policy payload */
export interface MaskingPolicyRef {
  maskingRules: Array<{
    /** CEL condition expression */
    condition: string;
    /** Masking level */
    maskingLevel: "NONE" | "PARTIAL" | "FULL";
    /** Semantic types to match */
    semanticTypes: string[];
  }>;
}

/** Masking exception policy payload */
export interface MaskingExceptionRef {
  exceptions: Array<{
    /** Member resource name */
    member: string;
    /** Action: "QUERY" | "EXPORT" */
    action: string;
    /** CEL condition for database/table matching */
    condition: string;
    /** Masking level granted */
    maskingLevel: "NONE" | "PARTIAL" | "FULL";
    /** Expiration (ISO timestamp) */
    expiration?: string;
  }>;
}

export const POLICY_CLIENT = "orgPolicyServiceClientConnect" as const;
export const POLICY_UPDATE_MASK_FIELDS = ["payload"] as const;

/**
 * Service methods:
 * - getPolicy({ name }) → Policy
 * - listPolicies({ parent, policyType? }) → { policies }
 * - createPolicy({ parent, policy }) → Policy
 * - updatePolicy({ policy, updateMask }) → Policy
 * - deletePolicy({ name }) → Empty
 */
