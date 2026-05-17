import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectServiceClientConnect } from "@/connect";
import { queryKeys } from "./query-keys";
import type { Project } from "@/types/proto-es/v1/project_service_pb";

/** Get a single project by its resource name. */
export function useProject(name: string) {
  return useQuery({
    queryKey: queryKeys.project.detail(name),
    queryFn: () => projectServiceClientConnect.getProject({ name }),
    enabled: !!name,
  });
}

/** List all projects. */
export function useProjectList() {
  return useQuery({
    queryKey: queryKeys.project.list(),
    queryFn: () =>
      projectServiceClientConnect.listProjects({ pageSize: 1000 }),
    select: (data) => data.projects,
  });
}

/** Update a project with automatic cache invalidation. */
export function useUpdateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      project,
      updateMask,
    }: {
      project: Partial<Project> & { name: string };
      updateMask: string[];
    }) =>
      projectServiceClientConnect.updateProject({
        project,
        updateMask,
      } as never),
    onSuccess: (updated) => {
      qc.setQueryData(queryKeys.project.detail(updated.name), updated);
      qc.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

/** Delete (archive) a project. */
export function useDeleteProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      projectServiceClientConnect.deleteProject({ name }),
    onSuccess: (_, name) => {
      qc.removeQueries({ queryKey: queryKeys.project.detail(name) });
      qc.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

/** Get project IAM policy. */
export function useProjectIamPolicy(projectName: string) {
  return useQuery({
    queryKey: queryKeys.project.iamPolicy(projectName),
    queryFn: () =>
      projectServiceClientConnect.getIamPolicy({
        resource: projectName,
      } as never),
    enabled: !!projectName,
  });
}
