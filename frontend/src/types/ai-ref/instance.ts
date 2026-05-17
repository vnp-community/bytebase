/**
 * AI REFERENCE — Instance Domain
 * Full type: src/types/proto-es/v1/instance_service_pb.d.ts (DO NOT read directly)
 */

/** Condensed Instance fields for AI reference */
export interface InstanceRef {
  /** Resource name: "instances/{instance}" */
  name: string;
  /** Human-readable title */
  title: string;
  /** Database engine type */
  engine: string;
  /** External link URL */
  externalLink: string;
  /** Whether instance is activated */
  activation: boolean;
  /** Data sources (admin + read-only connections) */
  dataSources: DataSourceRef[];
  /** Environment resource name */
  environment: string;
}

export interface DataSourceRef {
  /** "ADMIN" or "READ_ONLY" */
  type: string;
  /** Hostname */
  host: string;
  /** Port */
  port: string;
  /** Username */
  username: string;
  /** Whether to use SSL */
  useSsl: boolean;
}

/** Client import: `import { instanceServiceClientConnect } from "@/connect"` */
export const INSTANCE_CLIENT = "instanceServiceClientConnect" as const;

/** Fields allowed in updateMask for updateInstance() */
export const INSTANCE_UPDATE_MASK_FIELDS = [
  "title",
  "externalLink",
  "dataSources",
  "activation",
] as const;

/**
 * Service methods:
 * - getInstance({ name }) → Instance
 * - listInstances({ pageSize?, filter?, showDeleted? }) → { instances, nextPageToken }
 * - createInstance({ instance, instanceId }) → Instance
 * - updateInstance({ instance, updateMask }) → Instance
 * - deleteInstance({ name, force? }) → Empty
 * - syncInstance({ name }) → SyncInstanceResponse
 */
