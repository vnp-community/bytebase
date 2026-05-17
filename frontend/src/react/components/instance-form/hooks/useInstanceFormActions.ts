import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { instanceServiceClientConnect } from "@/connect";
import { pushNotification } from "@/store";

/**
 * Action hook for InstanceFormBody.
 * Handles: test connection, save instance.
 */
export function useInstanceFormActions() {
  const { t } = useTranslation();
  const [isRequesting, setIsRequesting] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message?: string;
  } | null>(null);

  const testConnection = useCallback(
    async (instance: unknown) => {
      setIsRequesting(true);
      setTestResult(null);
      try {
        await instanceServiceClientConnect.addDataSource({
          instance,
        } as never);
        setTestResult({ success: true });
        pushNotification({
          module: "bytebase",
          style: "SUCCESS",
          title: t("instance.successfully-connected"),
        });
      } catch (err) {
        setTestResult({
          success: false,
          message: err instanceof Error ? err.message : String(err),
        });
      } finally {
        setIsRequesting(false);
      }
    },
    [t]
  );

  const saveInstance = useCallback(
    async (instance: unknown, updateMask: string[]) => {
      setIsRequesting(true);
      try {
        await instanceServiceClientConnect.updateInstance({
          instance,
          updateMask,
        } as never);
        pushNotification({
          module: "bytebase",
          style: "SUCCESS",
          title: t("common.updated"),
        });
      } catch {
        // error shown by interceptor
      } finally {
        setIsRequesting(false);
      }
    },
    [t]
  );

  return {
    isRequesting,
    testResult,
    testConnection,
    saveInstance,
  };
}
