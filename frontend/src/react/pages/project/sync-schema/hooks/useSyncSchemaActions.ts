import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  schemaDesignServiceClientConnect,
  planServiceClientConnect,
} from "@/connect";
import { pushNotification } from "@/store";

/**
 * Action hook for ProjectSyncSchemaPage.
 * Handles: schema diff, plan creation, submission.
 */
export function useSyncSchemaActions(projectName: string) {
  const { t } = useTranslation();
  const [isRequesting, setIsRequesting] = useState(false);

  const diffMetadata = useCallback(
    async (
      sourceDatabase: string,
      targetDatabase: string
    ) => {
      setIsRequesting(true);
      try {
        const result =
          await schemaDesignServiceClientConnect.diffMetadata({
            sourceMetadata: { name: sourceDatabase },
            targetMetadata: { name: targetDatabase },
          } as never);
        return result;
      } catch (err) {
        throw err;
      } finally {
        setIsRequesting(false);
      }
    },
    []
  );

  const createSyncPlan = useCallback(
    async (specs: unknown[]) => {
      setIsRequesting(true);
      try {
        const plan = await planServiceClientConnect.createPlan({
          parent: projectName,
          plan: {
            title: `Sync Schema ${new Date().toISOString()}`,
            steps: [{ specs }],
          },
        } as never);
        pushNotification({
          module: "bytebase",
          style: "SUCCESS",
          title: t("common.created"),
        });
        return plan;
      } catch {
        // error shown by interceptor
      } finally {
        setIsRequesting(false);
      }
    },
    [projectName, t]
  );

  return {
    isRequesting,
    diffMetadata,
    createSyncPlan,
  };
}
