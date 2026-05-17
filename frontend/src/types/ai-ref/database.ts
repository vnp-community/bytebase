/**
 * AI REFERENCE — Database Domain
 * Full type: src/types/proto-es/v1/database_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Database fields for AI reference */
export interface DatabaseRef {
  /** Resource name: "instances/{instance}/databases/{db}" */
  name: string;
  /** Human-readable title */
  title: string;
  /** Environment resource name: "environments/{env}" */
  environment: string;
  /** Instance resource name: "instances/{instance}" */
  instance: string;
  /** Project resource name: "projects/{project}" */
  project: string;
  /** Key-value labels */
  labels: Record<string, string>;
  /** ISO timestamp of last successful sync */
  lastSuccessfulSyncTime?: string;
  /** Schema size in bytes */
  schemaSize: number;
}

/** Client import: `import { databaseServiceClientConnect } from "@/connect"` */
export const DATABASE_CLIENT = "databaseServiceClientConnect" as const;

/** Fields allowed in updateMask for updateDatabase() */
export const DATABASE_UPDATE_MASK_FIELDS = [
  "labels",
  "environment",
  "project",
  "title",
] as const;

/**
 * Service methods:
 * - getDatabase({ name }) → Database
 * - listDatabases({ parent, pageSize?, filter? }) → { databases, nextPageToken }
 * - updateDatabase({ database, updateMask }) → Database
 * - searchDatabases({ parent, filter? }) → { databases }
 * - batchGetDatabases({ parent, names }) → { databases }
 * - syncDatabase({ name }) → SyncDatabaseResponse
 */
