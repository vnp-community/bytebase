import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { identityProviderServiceClientConnect } from "@/connect";
import { pushNotification } from "@/store";

/**
 * Action hook for IDPDetailPage — update config, test connection.
 */
export function useIDPDetailActions(idpName: string, refetch: () => void) {
  const { t } = useTranslation();
  const [isRequesting, setIsRequesting] = useState(false);

  const updateIDP = useCallback(
    async (patch: unknown, updateMask: string[]) => {
      setIsRequesting(true);
      try {
        await identityProviderServiceClientConnect.updateIdentityProvider({
          identityProvider: patch,
          updateMask,
        } as never);
        pushNotification({
          module: "bytebase",
          style: "SUCCESS",
          title: t("common.updated"),
        });
        refetch();
      } catch {
        // error shown by interceptor
      } finally {
        setIsRequesting(false);
      }
    },
    [t, refetch, idpName]
  );

  const testConnection = useCallback(async () => {
    setIsRequesting(true);
    try {
      const result =
        await identityProviderServiceClientConnect.testIdentityProvider({
          identityProvider: { name: idpName },
        });
      return result;
    } catch (err) {
      throw err;
    } finally {
      setIsRequesting(false);
    }
  }, [idpName]);

  return {
    isRequesting,
    updateIDP,
    testConnection,
  };
}
