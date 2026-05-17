import { useQuery } from "@tanstack/react-query";
import { workloadIdentityServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List workload identities. */
export function useWorkloadIdentityList() {
  return useQuery({
    queryKey: queryKeys.workloadIdentity.list(),
    queryFn: () =>
      workloadIdentityServiceClientConnect.listWorkloadIdentities({} as never),
  });
}
