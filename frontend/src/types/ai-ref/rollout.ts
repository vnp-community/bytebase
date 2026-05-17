/**
 * AI REFERENCE — Rollout Domain
 * Full type: src/types/proto-es/v1/rollout_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Rollout fields for AI reference */
export interface RolloutRef {
  /** Resource name: "projects/{project}/rollouts/{rollout}" */
  name: string;
  /** Plan resource name that this rollout executes */
  plan: string;
  /** Stages (one per environment) */
  stages: StageRef[];
}

export interface StageRef {
  /** Stage resource name */
  name: string;
  /** Environment resource name */
  environment: string;
  /** Tasks within this stage */
  tasks: TaskRef[];
}

export interface TaskRef {
  /** Task resource name */
  name: string;
  /** Task type */
  type: "SCHEMA_UPDATE" | "DATA_UPDATE" | "DATABASE_CREATE" | "DATABASE_RESTORE";
  /** Task status */
  status: "NOT_STARTED" | "PENDING" | "RUNNING" | "DONE" | "FAILED" | "CANCELED" | "SKIPPED";
  /** Database resource name */
  database: string;
}

export interface TaskRunRef {
  /** TaskRun resource name */
  name: string;
  /** Execution status */
  status: "PENDING" | "RUNNING" | "DONE" | "FAILED" | "CANCELED";
  /** Start and end time (ISO) */
  startTime?: string;
  endTime?: string;
}

export const ROLLOUT_CLIENT = "rolloutServiceClientConnect" as const;

/**
 * Service methods:
 * - getRollout({ name }) → Rollout
 * - listRollouts({ parent, pageSize? }) → { rollouts, nextPageToken }
 * - createRollout({ parent, rollout }) → Rollout
 * - runTasks({ parent, tasks, reason? }) → RunTasksResponse
 * - batchRunTasks({ parent, tasks, reason? }) → BatchRunTasksResponse
 */
