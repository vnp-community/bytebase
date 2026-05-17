import { useQuery } from "@tanstack/react-query";
import { rolloutServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Get a rollout by name. */
export function useRollout(name: string) {
  return useQuery({
    queryKey: queryKeys.rollout.detail(name),
    queryFn: () => rolloutServiceClientConnect.getRollout({ name }),
    enabled: !!name,
  });
}
