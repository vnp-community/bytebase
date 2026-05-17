import { useQuery } from "@tanstack/react-query";
import { serviceAccountServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List service accounts. */
export function useServiceAccountList() {
  return useQuery({
    queryKey: queryKeys.serviceAccount.list(),
    queryFn: () =>
      serviceAccountServiceClientConnect.listServiceAccounts({} as never),
  });
}
