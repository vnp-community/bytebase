import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { worksheetServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Search worksheets (the API uses 'searchWorksheets', not 'listWorksheets'). */
export function useWorksheetList(parent?: string) {
  return useQuery({
    queryKey: queryKeys.worksheet.list(parent),
    queryFn: () =>
      worksheetServiceClientConnect.searchWorksheets({
        parent: parent ?? "",
        pageSize: 100,
      } as never),
    select: (data) => (data as { worksheets: unknown[] }).worksheets,
  });
}

/** Get a worksheet by name. */
export function useWorksheet(name: string) {
  return useQuery({
    queryKey: queryKeys.worksheet.detail(name),
    queryFn: () => worksheetServiceClientConnect.getWorksheet({ name }),
    enabled: !!name,
  });
}

/** Update a worksheet. */
export function useUpdateWorksheet() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { worksheet: unknown; updateMask: string[] }) =>
      worksheetServiceClientConnect.updateWorksheet(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.worksheet.all });
    },
  });
}
