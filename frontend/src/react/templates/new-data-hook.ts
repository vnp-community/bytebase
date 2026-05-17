// AI: Copy + rename for data fetching hooks.
// Pattern: useQuery + queryKey convention
//
// RULES:
//   1. queryKey must be unique and contain all dependencies
//   2. Use `enabled` to prevent fetching when params are empty
//   3. Re-export from a hooks barrel file (src/react/hooks/queries/index.ts)
//   4. Return type should be the proto-es type (not a ref type)

// TODO: Uncomment and adapt these imports
// import { useQuery } from "@tanstack/react-query";
// import { templateServiceClientConnect } from "@/connect";

/**
 * Fetch a single entity by resource name.
 * Usage: const { data, isLoading, error } = useTemplateData("instances/prod/databases/mydb");
 */
export function useTemplateData(name: string) {
  // TODO: Uncomment and replace with actual service client
  // return useQuery({
  //   queryKey: ["template", name],
  //   queryFn: () => templateServiceClientConnect.getTemplate({ name }),
  //   enabled: !!name,
  // });

  // Placeholder — remove when implementing
  return { data: undefined, isLoading: false, error: null };
}

/**
 * List entities with optional filtering and pagination.
 * Usage: const { data } = useTemplateList("projects/my-project", { pageSize: 50 });
 */
export function useTemplateList(
  parent: string,
  options?: { pageSize?: number; filter?: string }
) {
  // TODO: Uncomment and replace
  // return useQuery({
  //   queryKey: ["templates", parent, options?.filter],
  //   queryFn: () =>
  //     templateServiceClientConnect.listTemplates({
  //       parent,
  //       pageSize: options?.pageSize ?? 100,
  //       filter: options?.filter,
  //     }),
  //   enabled: !!parent,
  // });

  return { data: undefined, isLoading: false, error: null };
}
