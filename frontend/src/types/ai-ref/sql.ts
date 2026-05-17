/**
 * AI REFERENCE — SQL Service Domain
 * Full type: src/types/proto-es/v1/sql_service_pb.d.ts (DO NOT read directly)
 */

/** Query request shape for sqlServiceClientConnect.query() */
export interface QueryRequestRef {
  /** Instance resource name (e.g., "instances/prod") */
  instance: string;
  /** SQL statement to execute */
  statement: string;
  /** Database name (e.g., "mydb") */
  database: string;
  /** Max rows to return */
  limit?: number;
  /** Query timeout in seconds */
  timeout?: number;
}

/** Export request shape for sqlServiceClientConnect.export() */
export interface ExportRequestRef {
  /** Instance resource name */
  instance: string;
  /** SQL statement */
  statement: string;
  /** Database name */
  database: string;
  /** Export format */
  format: "CSV" | "JSON" | "SQL" | "XLSX";
  /** Password for encrypted export (optional) */
  password?: string;
}

/** Query history search result (condensed) */
export interface QueryHistoryRef {
  /** Database resource name */
  database: string;
  /** SQL statement */
  statement: string;
  /** Creator resource name */
  creator: string;
  /** When the query was executed (ISO) */
  createTime: string;
  /** Duration in milliseconds */
  duration: number;
}

export const SQL_CLIENT = "sqlServiceClientConnect" as const;

/**
 * Service methods:
 * - query({ instance, statement, database, limit?, timeout? }) → QueryResponse
 * - export({ instance, statement, database, format, password? }) → ExportResponse
 * - searchQueryHistories({ parent, pageSize?, filter? }) → { queryHistories, nextPageToken }
 */
