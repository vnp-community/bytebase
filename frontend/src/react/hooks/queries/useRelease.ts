import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { releaseServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List releases for a project. */
export function useReleaseList(project: string) {
  return useQuery({
    queryKey: queryKeys.release.list(project),
    queryFn: () =>
      releaseServiceClientConnect.listReleases({
        parent: project,
        pageSize: 50,
      }),
    enabled: !!project,
    select: (data) => data.releases,
  });
}

/** Get a single release. */
export function useRelease(name: string) {
  return useQuery({
    queryKey: queryKeys.release.detail(name),
    queryFn: () => releaseServiceClientConnect.getRelease({ name }),
    enabled: !!name,
  });
}

/** Create a release. */
export function useCreateRelease() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { parent: string; release: unknown }) =>
      releaseServiceClientConnect.createRelease(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.release.all });
    },
  });
}
