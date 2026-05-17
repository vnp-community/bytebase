import { useQuery } from "@tanstack/react-query";
import { databaseServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Get database schema (metadata). */
export function useDatabaseSchema(databaseName: string) {
  return useQuery({
    queryKey: queryKeys.schema.detail(databaseName),
    queryFn: () =>
      databaseServiceClientConnect.getDatabaseSchema({
        name: `${databaseName}/schema`,
      } as never),
    enabled: !!databaseName,
  });
}

/** Get database metadata. */
export function useDatabaseMetadata(databaseName: string) {
  return useQuery({
    queryKey: queryKeys.schema.metadata(databaseName),
    queryFn: () =>
      databaseServiceClientConnect.getDatabaseMetadata({
        name: `${databaseName}/metadata`,
      } as never),
    enabled: !!databaseName,
  });
}
