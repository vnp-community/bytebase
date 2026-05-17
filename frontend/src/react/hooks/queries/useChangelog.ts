import { useQuery } from "@tanstack/react-query";
import { databaseServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List changelogs for a database. */
export function useChangelogList(database: string) {
  return useQuery({
    queryKey: queryKeys.changelog.list(database),
    queryFn: () =>
      databaseServiceClientConnect.listChangelogs({
        parent: database,
        pageSize: 50,
      } as never),
    enabled: !!database,
    select: (data) => data.changelogs,
  });
}

/** Get a single changelog entry. */
export function useChangelog(name: string) {
  return useQuery({
    queryKey: queryKeys.changelog.detail(name),
    queryFn: () =>
      databaseServiceClientConnect.getChangelog({ name } as never),
    enabled: !!name,
  });
}
