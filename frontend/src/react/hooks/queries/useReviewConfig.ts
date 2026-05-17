import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { reviewConfigServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List all review configs. */
export function useReviewConfigList() {
  return useQuery({
    queryKey: queryKeys.reviewConfig.list(),
    queryFn: () => reviewConfigServiceClientConnect.listReviewConfigs({}),
    select: (data) => data.reviewConfigs,
  });
}

/** Get a review config. */
export function useReviewConfig(name: string) {
  return useQuery({
    queryKey: queryKeys.reviewConfig.detail(name),
    queryFn: () => reviewConfigServiceClientConnect.getReviewConfig({ name }),
    enabled: !!name,
  });
}

/** Update a review config. */
export function useUpdateReviewConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { reviewConfig: unknown; updateMask: string[] }) =>
      reviewConfigServiceClientConnect.updateReviewConfig(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.reviewConfig.all });
    },
  });
}
