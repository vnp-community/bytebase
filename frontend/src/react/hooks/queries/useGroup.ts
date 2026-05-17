import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { groupServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List all groups. */
export function useGroupList() {
  return useQuery({
    queryKey: queryKeys.group.list(),
    queryFn: () => groupServiceClientConnect.listGroups({ pageSize: 1000 }),
    select: (data) => data.groups,
  });
}

/** Get a group by name. */
export function useGroup(name: string) {
  return useQuery({
    queryKey: queryKeys.group.detail(name),
    queryFn: () => groupServiceClientConnect.getGroup({ name }),
    enabled: !!name,
  });
}

/** Update a group. */
export function useUpdateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { group: unknown; updateMask: string[] }) =>
      groupServiceClientConnect.updateGroup(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.group.all });
    },
  });
}
