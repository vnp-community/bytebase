import { useInfiniteQuery } from "@tanstack/react-query";
import { auditLogServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Paginated audit log list using cursor-based infinite query. */
export function useAuditLogList(filter?: string) {
  return useInfiniteQuery({
    queryKey: queryKeys.auditLog.list(filter),
    queryFn: ({ pageParam }) =>
      auditLogServiceClientConnect.searchAuditLogs({
        filter: filter ?? "",
        pageSize: 50,
        pageToken: pageParam ?? "",
      } as never),
    initialPageParam: "",
    getNextPageParam: (lastPage) =>
      (lastPage as { nextPageToken?: string }).nextPageToken || undefined,
  });
}
