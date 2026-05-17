/**
 * AI REFERENCE — Project Domain
 * Full type: src/types/proto-es/v1/project_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Project fields for AI reference */
export interface ProjectRef {
  /** Resource name: "projects/{project}" */
  name: string;
  /** Human-readable title */
  title: string;
  /** Short project key (e.g. "HR") */
  key: string;
  /** Workflow type: UI_CHANGE or VCS_CHANGE */
  workflow: "UI" | "VCS";
  /** "TENANT" or "STANDARD" */
  tenantMode: string;
  /** Data classification config ID */
  dataClassificationConfigId: string;
}

/** IAM Policy for project-level RBAC */
export interface ProjectIamPolicyRef {
  /** Bindings: role → members */
  bindings: Array<{
    role: string;
    members: string[];
    condition?: { expression: string };
  }>;
}

/** Client import: `import { projectServiceClientConnect } from "@/connect"` */
export const PROJECT_CLIENT = "projectServiceClientConnect" as const;

/** Fields allowed in updateMask for updateProject() */
export const PROJECT_UPDATE_MASK_FIELDS = [
  "title",
  "key",
  "tenantMode",
  "schemaChange",
  "dataClassificationConfigId",
] as const;

/**
 * Service methods:
 * - getProject({ name }) → Project
 * - listProjects({ pageSize?, filter?, showDeleted? }) → { projects, nextPageToken }
 * - updateProject({ project, updateMask }) → Project
 * - getIamPolicy({ project }) → IamPolicy
 * - setIamPolicy({ project, policy }) → IamPolicy
 */
