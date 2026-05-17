import { useEffect, useMemo, useState } from "react";
import { useVueState } from "@/react/hooks/useVueState";
import {
  useDatabaseV1Store,
  useProjectV1Store,
} from "@/store";
import { projectNamePrefix } from "@/store/modules/v1/common";

/**
 * Data hook for ProjectSyncSchemaPage.
 * Manages: source/target databases, diff results, schema state.
 */
export function useSyncSchemaData(projectId: string) {
  const projectName = `${projectNamePrefix}${projectId}`;
  const projectStore = useProjectV1Store();
  const databaseStore = useDatabaseV1Store();

  const project = useVueState(() =>
    projectStore.getProjectByName(projectName)
  );

  const databases = useVueState(() =>
    databaseStore.databaseListByProject(projectName)
  );

  const [currentStep, setCurrentStep] = useState(0);
  const [sourceDatabase, setSourceDatabase] = useState<string>("");
  const [targetDatabases, setTargetDatabases] = useState<string[]>([]);
  const [sourceSchemaType, setSourceSchemaType] = useState<
    "CHANGELOG" | "RAW_SQL" | "CURRENT"
  >("CURRENT");

  // Fetch databases on mount
  useEffect(() => {
    databaseStore.fetchDatabaseListByProject(projectName);
  }, [projectName, databaseStore]);

  return {
    project,
    projectName,
    databases,
    currentStep,
    setCurrentStep,
    sourceDatabase,
    setSourceDatabase,
    targetDatabases,
    setTargetDatabases,
    sourceSchemaType,
    setSourceSchemaType,
  };
}
