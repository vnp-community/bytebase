import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { instanceServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";
import type { Instance } from "@/types/proto-es/v1/instance_service_pb";

/** Get a single instance by its resource name. */
export function useInstance(name: string) {
  return useQuery({
    queryKey: queryKeys.instance.detail(name),
    queryFn: () => instanceServiceClientConnect.getInstance({ name }),
    enabled: !!name,
  });
}

/** List all instances (optionally filtered). */
export function useInstanceList(filter?: string) {
  return useQuery({
    queryKey: queryKeys.instance.list(filter),
    queryFn: () =>
      instanceServiceClientConnect.listInstances({
        pageSize: 1000,
        filter: filter ?? "",
      }),
    select: (data) => data.instances,
  });
}

/** Update an instance with automatic cache invalidation. */
export function useUpdateInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      instance,
      updateMask,
    }: {
      instance: Partial<Instance> & { name: string };
      updateMask: string[];
    }) =>
      instanceServiceClientConnect.updateInstance({
        instance,
        updateMask,
      } as never),
    onSuccess: (updated) => {
      qc.setQueryData(queryKeys.instance.detail(updated.name), updated);
      qc.invalidateQueries({ queryKey: queryKeys.instance.all });
    },
  });
}

/** Create a new instance. */
export function useCreateInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      instance: Partial<Instance>;
      instanceId: string;
    }) =>
      instanceServiceClientConnect.createInstance({
        instance: args.instance,
        instanceId: args.instanceId,
      } as never),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.instance.all });
    },
  });
}

/** Delete an instance. */
export function useDeleteInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      instanceServiceClientConnect.deleteInstance({ name }),
    onSuccess: (_, name) => {
      qc.removeQueries({ queryKey: queryKeys.instance.detail(name) });
      qc.invalidateQueries({ queryKey: queryKeys.instance.all });
    },
  });
}
