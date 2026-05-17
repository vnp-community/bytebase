// AI: Copy + rename for mutation hooks.
// Pattern: useMutation + onSuccess cache invalidation
//
// RULES:
//   1. useMutation for create/update/delete operations
//   2. Always invalidate relevant queries on success
//   3. updateMask REQUIRED for update operations
//   4. No manual try/catch — interceptors handle error notifications

// TODO: Uncomment and adapt these imports
// import { useMutation, useQueryClient } from "@tanstack/react-query";
// import { templateServiceClientConnect } from "@/connect";
// import { create } from "@bufbuild/protobuf";
// import { TemplateSchema } from "@/types/proto-es/v1/template_service_pb";

/**
 * Create a new entity.
 * Usage: const { mutate: createEntity } = useCreateTemplate();
 * Then:  createEntity({ parent: "projects/hr", title: "New Entity" });
 */
export function useCreateTemplate() {
  // TODO: Uncomment and replace
  // const queryClient = useQueryClient();
  // return useMutation({
  //   mutationFn: ({ parent, title }: { parent: string; title: string }) =>
  //     templateServiceClientConnect.createTemplate({
  //       parent,
  //       template: create(TemplateSchema, { title }),
  //     }),
  //   onSuccess: (_data, variables) => {
  //     // Invalidate list cache so it refetches
  //     queryClient.invalidateQueries({ queryKey: ["templates", variables.parent] });
  //   },
  // });

  return { mutate: () => {}, isPending: false };
}

/**
 * Update an existing entity.
 * IMPORTANT: updateMask must list ONLY the changed fields.
 * Usage: const { mutate: updateEntity } = useUpdateTemplate();
 * Then:  updateEntity({ template: { ...existing, title: "New" }, updateMask: ["title"] });
 */
export function useUpdateTemplate() {
  // TODO: Uncomment and replace
  // const queryClient = useQueryClient();
  // return useMutation({
  //   mutationFn: ({
  //     template,
  //     updateMask,
  //   }: {
  //     template: { name: string; title?: string; /* ...fields */ };
  //     updateMask: string[];
  //   }) =>
  //     templateServiceClientConnect.updateTemplate({ template, updateMask }),
  //   onSuccess: (data) => {
  //     // Update single-entity cache
  //     queryClient.setQueryData(["template", data.name], data);
  //     // Invalidate list cache
  //     queryClient.invalidateQueries({ queryKey: ["templates"] });
  //   },
  // });

  return { mutate: () => {}, isPending: false };
}

/**
 * Delete an entity.
 * Usage: const { mutate: deleteEntity } = useDeleteTemplate();
 * Then:  deleteEntity({ name: "instances/prod/databases/mydb" });
 */
export function useDeleteTemplate() {
  // TODO: Uncomment and replace
  // const queryClient = useQueryClient();
  // return useMutation({
  //   mutationFn: ({ name }: { name: string }) =>
  //     templateServiceClientConnect.deleteTemplate({ name }),
  //   onSuccess: (_data, variables) => {
  //     // Remove from single-entity cache
  //     queryClient.removeQueries({ queryKey: ["template", variables.name] });
  //     // Invalidate list cache
  //     queryClient.invalidateQueries({ queryKey: ["templates"] });
  //   },
  // });

  return { mutate: () => {}, isPending: false };
}
