import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { userServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";
import type { User } from "@/types/proto-es/v1/user_service_pb";

/** Get a single user by resource name. */
export function useUser(name: string) {
  return useQuery({
    queryKey: queryKeys.user.detail(name),
    queryFn: () => userServiceClientConnect.getUser({ name }),
    enabled: !!name,
  });
}

/** List all users. */
export function useUserList() {
  return useQuery({
    queryKey: queryKeys.user.list(),
    queryFn: () =>
      userServiceClientConnect.listUsers({ pageSize: 1000 }),
    select: (data) => data.users,
  });
}

/** Update a user with automatic cache invalidation. */
export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      user,
      updateMask,
    }: {
      user: Partial<User> & { name: string };
      updateMask: string[];
    }) =>
      userServiceClientConnect.updateUser({
        user,
        updateMask,
      } as never),
    onSuccess: (updated) => {
      qc.setQueryData(queryKeys.user.detail(updated.name), updated);
      qc.invalidateQueries({ queryKey: queryKeys.user.all });
    },
  });
}

/** Delete a user. */
export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      userServiceClientConnect.deleteUser({ name }),
    onSuccess: (_, name) => {
      qc.removeQueries({ queryKey: queryKeys.user.detail(name) });
      qc.invalidateQueries({ queryKey: queryKeys.user.all });
    },
  });
}
