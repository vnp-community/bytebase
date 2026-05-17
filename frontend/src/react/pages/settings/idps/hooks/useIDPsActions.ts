import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { identityProviderServiceClientConnect } from "@/connect";
import { pushNotification } from "@/store";

/**
 * Action hook for IDPsPage — create, delete, test connection.
 */
export function useIDPsActions(refetch: () => void) {
  const { t } = useTranslation();
  const [isRequesting, setIsRequesting] = useState(false);

  const deleteIDP = useCallback(
    async (idpName: string) => {
      if (!window.confirm(t("settings.sso.delete-confirm"))) return;
      setIsRequesting(true);
      try {
        await identityProviderServiceClientConnect.deleteIdentityProvider({
          name: idpName,
        });
        pushNotification({
          module: "bytebase",
          style: "SUCCESS",
          title: t("common.deleted"),
        });
        refetch();
      } catch {
        // error shown by interceptor
      } finally {
        setIsRequesting(false);
      }
    },
    [t, refetch]
  );

  const testConnection = useCallback(
    async (idpName: string) => {
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
    },
    []
  );

  return {
    isRequesting,
    deleteIDP,
    testConnection,
  };
}
