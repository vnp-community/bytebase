import { useEffect, useMemo } from "react";
import { identityProviderServiceClientConnect } from "@/connect";
import { useVueState } from "@/react/hooks/useVueState";
import {
  useSubscriptionV1Store,
} from "@/store";
import { PlanFeature } from "@/types/proto-es/v1/subscription_service_pb";
import { useState } from "react";

/**
 * Data hook for IDPsPage — fetches IDP list and feature checks.
 */
export function useIDPsData() {
  const subscriptionStore = useSubscriptionV1Store();
  const [idpList, setIdpList] = useState<unknown[]>([]);
  const [loading, setLoading] = useState(true);

  const hasSSOFeature = useVueState(() =>
    subscriptionStore.hasFeature(PlanFeature.FEATURE_SSO)
  );
  const hasEnterpriseSSOFeature = useVueState(() =>
    subscriptionStore.hasFeature(PlanFeature.FEATURE_ENTERPRISE_SSO)
  );

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    identityProviderServiceClientConnect
      .listIdentityProviders({})
      .then((resp) => {
        if (!cancelled) {
          setIdpList(resp.identityProviders ?? []);
        }
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
  }, []);

  return {
    idpList,
    loading,
    hasSSOFeature,
    hasEnterpriseSSOFeature,
    refetch: () => {
      identityProviderServiceClientConnect
        .listIdentityProviders({})
        .then((resp) => setIdpList(resp.identityProviders ?? []))
        .catch(() => {});
    },
  };
}
