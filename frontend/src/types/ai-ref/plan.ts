/**
 * AI REFERENCE — Plan Domain
 * Full type: src/types/proto-es/v1/plan_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Plan fields for AI reference */
export interface PlanRef {
  /** Resource name: "projects/{project}/plans/{plan}" */
  name: string;
  /** Human-readable title */
  title: string;
  /** Description (markdown) */
  description: string;
  /** Creator resource name */
  creator: string;
  /** Plan specs — each spec is one SQL script or schema change */
  specs: PlanSpecRef[];
}

export interface PlanSpecRef {
  /** Unique spec ID */
  id: string;
  /** Change type: "CREATE_DATABASE" | "CHANGE_DATABASE" */
  type: string;
  /** Sheet resource name containing the SQL */
  sheet: string;
}

export const PLAN_CLIENT = "planServiceClientConnect" as const;
export const PLAN_UPDATE_MASK_FIELDS = ["title", "description", "specs"] as const;

/**
 * Service methods:
 * - getPlan({ name }) → Plan
 * - listPlans({ parent, pageSize?, filter? }) → { plans, nextPageToken }
 * - createPlan({ parent, plan }) → Plan
 * - updatePlan({ plan, updateMask }) → Plan
 */
