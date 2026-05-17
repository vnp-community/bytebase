import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { settingServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** Get a workspace setting by name. */
export function useSetting(settingName: string) {
  return useQuery({
    queryKey: queryKeys.setting.byName(settingName),
    queryFn: () =>
      settingServiceClientConnect.getSetting({ name: settingName }),
    enabled: !!settingName,
  });
}

/** Update a setting. */
export function useUpdateSetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { setting: unknown; updateMask: string[] }) =>
      settingServiceClientConnect.updateSetting(args as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.setting.all });
    },
  });
}
