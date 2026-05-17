import { useQuery } from "@tanstack/react-query";
import { settingServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/**
 * Environments are stored as a workspace setting in Bytebase.
 * They don't have a dedicated service — we fetch them via SettingService.
 *
 * The Pinia store (useEnvironmentV1Store) parses the setting response
 * into Environment objects. For new React code, this hook provides
 * the raw setting that contains the environment list.
 */

/** Get the environment setting (contains all environments). */
export function useEnvironmentSetting() {
  return useQuery({
    queryKey: queryKeys.environment.list(),
    queryFn: () =>
      settingServiceClientConnect.getSetting({
        name: "settings/bb.workspace.environment",
      }),
    staleTime: 10 * 60 * 1000, // environments rarely change
  });
}
