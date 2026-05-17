import { useQuery } from "@tanstack/react-query";
import { subscriptionServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Get the current workspace subscription (read-only). */
export function useSubscription() {
  return useQuery({
    queryKey: queryKeys.subscription.current(),
    queryFn: () => subscriptionServiceClientConnect.getSubscription({}),
    staleTime: 10 * 60 * 1000, // 10 min — subscription rarely changes
  });
}
