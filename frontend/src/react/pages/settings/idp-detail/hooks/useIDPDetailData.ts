import { useEffect, useState } from "react";
import { identityProviderServiceClientConnect } from "@/connect";

/**
 * Data hook for IDPDetailPage — fetches a single IDP by name.
 */
export function useIDPDetailData(idpName: string) {
  const [idp, setIdp] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    identityProviderServiceClientConnect
      .getIdentityProvider({ name: idpName })
      .then((resp) => {
        if (!cancelled) setIdp(resp);
      })
      .catch(() => {
        // error shown by interceptor
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [idpName]);

  return {
    idp,
    loading,
    refetch: () => {
      identityProviderServiceClientConnect
        .getIdentityProvider({ name: idpName })
        .then((resp) => setIdp(resp))
        .catch(() => {});
    },
  };
}
