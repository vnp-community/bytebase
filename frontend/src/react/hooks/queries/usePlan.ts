import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { planServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List plans for a project. */
export function usePlanList(project: string) {
  return useQuery({
    queryKey: queryKeys.plan.list(project),
    queryFn: () =>
      planServiceClientConnect.listPlans({ parent: project, pageSize: 50 }),
    enabled: !!project,
    select: (data) => data.plans,
  });
}

/** Get a single plan. */
export function usePlan(name: string) {
  return useQuery({
    queryKey: queryKeys.plan.detail(name),
    queryFn: () => planServiceClientConnect.getPlan({ name }),
    enabled: !!name,
  });
}

/** Create a new plan. */
export function useCreatePlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { parent: string; plan: unknown }) =>
      planServiceClientConnect.createPlan(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.plan.all });
    },
  });
}
