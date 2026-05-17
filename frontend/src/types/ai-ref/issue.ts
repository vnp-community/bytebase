/**
 * AI REFERENCE — Issue Domain
 * Full type: src/types/proto-es/v1/issue_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Issue fields for AI reference */
export interface IssueRef {
  /** Resource name: "projects/{project}/issues/{issue}" */
  name: string;
  /** Human-readable title */
  title: string;
  /** Issue status */
  status: "OPEN" | "DONE" | "CANCELED";
  /** Issue type */
  type: "DATABASE_CHANGE" | "GRANT_REQUEST" | "DATABASE_DATA_EXPORT";
  /** Creator resource name: "users/{email}" */
  creator: string;
  /** Assignee resource name */
  assignee: string;
  /** Parent project resource name */
  project: string;
  /** Approval status */
  approvalFindingDone: boolean;
  /** List of subscribers */
  subscribers: string[];
  /** Description (markdown) */
  description: string;
  /** Plan resource name (1:1 relationship) */
  plan: string;
  /** Rollout resource name (1:1 relationship) */
  rollout: string;
}

export const ISSUE_CLIENT = "issueServiceClientConnect" as const;
export const ISSUE_UPDATE_MASK_FIELDS = ["title", "description", "subscribers", "assignee"] as const;

/**
 * Service methods:
 * - getIssue({ name }) → Issue
 * - listIssues({ parent, pageSize?, filter? }) → { issues, nextPageToken }
 * - createIssue({ parent, issue }) → Issue
 * - updateIssue({ issue, updateMask }) → Issue
 * - approveIssue({ name, comment? }) → Issue
 * - rejectIssue({ name, comment? }) → Issue
 * - batchUpdateIssuesStatus({ parent, issues, status }) → { issues }
 */
