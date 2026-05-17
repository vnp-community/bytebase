import { useQuery } from "@tanstack/react-query";
import { accessGrantServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List access grants for a project. */
export function useAccessGrantList(project: string) {
  return useQuery({
    queryKey: queryKeys.accessGrant.list(project),
    queryFn: () =>
      accessGrantServiceClientConnect.listAccessGrants({
        parent: project,
      } as never),
    enabled: !!project,
  });
}
