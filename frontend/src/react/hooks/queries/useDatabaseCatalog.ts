import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { databaseCatalogServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";
import type { DatabaseCatalog } from "@/types/proto-es/v1/database_catalog_service_pb";
import { extractDatabaseResourceName } from "@/utils";

const ensureDatabaseResourceName = (name: string) => {
  return extractDatabaseResourceName(name).database;
};

const ensureDatabaseCatalogResourceName = (name: string) => {
  const database = ensureDatabaseResourceName(name);
  return `${database}/catalog`;
};

/** Get database catalog */
export function useDatabaseCatalog(databaseName: string) {
  return useQuery({
    queryKey: [...queryKeys.schema.all, "catalog", databaseName], // using schema queryKey root or custom
    queryFn: () =>
      databaseCatalogServiceClientConnect.getDatabaseCatalog({
        name: ensureDatabaseCatalogResourceName(databaseName),
      }),
    enabled: !!databaseName,
  });
}

/** Update database catalog */
export function useUpdateDatabaseCatalog() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { catalog: DatabaseCatalog }) =>
      databaseCatalogServiceClientConnect.updateDatabaseCatalog(args as never),
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: [...queryKeys.schema.all, "catalog"] });
    },
  });
}
