import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { orgPolicyServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List policies for a parent resource. */
export function usePolicyList(parent: string) {
  return useQuery({
    queryKey: queryKeys.policy.list(parent),
    queryFn: () =>
      orgPolicyServiceClientConnect.listPolicies({ parent }),
    enabled: !!parent,
    select: (data) => data.policies,
  });
}

/** Update a policy. */
export function useUpdatePolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { policy: unknown; updateMask: string[] }) =>
      orgPolicyServiceClientConnect.updatePolicy(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.policy.all });
    },
  });
}
