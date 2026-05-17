import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { databaseGroupServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List database groups for a project. */
export function useDatabaseGroupList(project: string) {
  return useQuery({
    queryKey: queryKeys.databaseGroup.list(project),
    queryFn: () =>
      databaseGroupServiceClientConnect.listDatabaseGroups({
        parent: project,
      }),
    enabled: !!project,
    select: (data) => data.databaseGroups,
  });
}

/** Get a single database group. */
export function useDatabaseGroup(name: string) {
  return useQuery({
    queryKey: queryKeys.databaseGroup.detail(name),
    queryFn: () =>
      databaseGroupServiceClientConnect.getDatabaseGroup({ name }),
    enabled: !!name,
  });
}

/** Delete a database group. */
export function useDeleteDatabaseGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      databaseGroupServiceClientConnect.deleteDatabaseGroup({ name }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.databaseGroup.all });
    },
  });
}
