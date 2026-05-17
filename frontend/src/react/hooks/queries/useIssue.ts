import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { issueServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List issues for a project, optionally filtered. */
export function useIssueList(project: string, filter?: string) {
  return useQuery({
    queryKey: queryKeys.issue.list(project, filter),
    queryFn: () =>
      issueServiceClientConnect.listIssues({
        parent: project,
        filter: filter ?? "",
        pageSize: 50,
      }),
    enabled: !!project,
    select: (data) => data.issues,
  });
}

/** Get a single issue by resource name. */
export function useIssue(name: string) {
  return useQuery({
    queryKey: queryKeys.issue.detail(name),
    queryFn: () => issueServiceClientConnect.getIssue({ name }),
    enabled: !!name,
  });
}

/** Update an issue. */
export function useUpdateIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { issue: unknown; updateMask: string[] }) =>
      issueServiceClientConnect.updateIssue(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.issue.all });
    },
  });
}
