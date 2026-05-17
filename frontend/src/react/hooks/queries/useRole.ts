import { useQuery } from "@tanstack/react-query";
import { roleServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List all roles. */
export function useRoleList() {
  return useQuery({
    queryKey: queryKeys.role.list(),
    queryFn: () => roleServiceClientConnect.listRoles({}),
    select: (data) => data.roles,
  });
}
