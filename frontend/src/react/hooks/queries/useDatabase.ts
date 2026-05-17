import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { databaseServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";
import type { Database } from "@/types/proto-es/v1/database_service_pb";

/** Get a single database by its resource name. */
export function useDatabase(name: string) {
  return useQuery({
    queryKey: queryKeys.database.detail(name),
    queryFn: () => databaseServiceClientConnect.getDatabase({ name }),
    enabled: !!name,
  });
}

/** List databases under a parent (project or instance). */
export function useDatabaseList(parent: string, filter?: string) {
  return useQuery({
    queryKey: [...queryKeys.database.list(parent), filter],
    queryFn: () =>
      databaseServiceClientConnect.listDatabases({
        parent,
        filter: filter ?? "",
        pageSize: 1000,
      }),
    enabled: !!parent,
    select: (data) => data.databases,
  });
}

/** Update a database with automatic cache invalidation. */
export function useUpdateDatabase() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      database,
      updateMask,
    }: {
      database: Partial<Database> & { name: string };
      updateMask: string[];
    }) =>
      databaseServiceClientConnect.updateDatabase({
        database,
        updateMask,
      } as never),
    onSuccess: (updated) => {
      qc.setQueryData(queryKeys.database.detail(updated.name), updated);
      qc.invalidateQueries({ queryKey: queryKeys.database.all });
    },
  });
}

/** Batch update databases. */
export function useBatchUpdateDatabases() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      parent: string;
      requests: Array<{
        database: Partial<Database> & { name: string };
        updateMask: string[];
      }>;
    }) =>
      databaseServiceClientConnect.batchUpdateDatabases({
        parent: args.parent,
        requests: args.requests,
      } as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.database.all });
    },
  });
}
