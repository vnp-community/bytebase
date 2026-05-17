import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { identityProviderServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";

/** List all identity providers. */
export function useIDPList() {
  return useQuery({
    queryKey: queryKeys.idp.list(),
    queryFn: () =>
      identityProviderServiceClientConnect.listIdentityProviders({}),
    select: (data) => data.identityProviders,
  });
}

/** Get a single IDP by name. */
export function useIDP(name: string) {
  return useQuery({
    queryKey: queryKeys.idp.detail(name),
    queryFn: () =>
      identityProviderServiceClientConnect.getIdentityProvider({ name }),
    enabled: !!name,
  });
}

/** Delete an IDP. */
export function useDeleteIDP() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      identityProviderServiceClientConnect.deleteIdentityProvider({ name }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.idp.all });
    },
  });
}

/** Test IDP connection. */
export function useTestIDPConnection() {
  return useMutation({
    mutationFn: (identityProvider: unknown) =>
      identityProviderServiceClientConnect.testIdentityProvider({
        identityProvider,
      } as never),
  });
}
