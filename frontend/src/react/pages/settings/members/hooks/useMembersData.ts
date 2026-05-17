import { useMemo, useState, useEffect } from "react";
import type { MemberBinding } from "@/components/Member/types";
import { getMemberBindings } from "@/components/Member/utils";
import { useAppStore } from "@/react/stores/app";
import { useProject } from "@/react/hooks/queries";
import { projectNamePrefix } from "@/store/modules/v1/common";
import { State } from "@/types/proto-es/v1/common_pb";
import { PlanFeature } from "@/types/proto-es/v1/subscription_service_pb";
import {
  hasProjectPermissionV2,
  hasWorkspacePermissionV2,
  isBindingPolicyExpired,
} from "@/utils";

const EMPTY_ROLE_SET = new Set<string>();

export function useMembersData(projectId?: string) {
  const currentUser = useAppStore(s => s.currentUser);
  const userCountInIam = useAppStore(s => s.userCountInIam());
  const userCountLimit = useAppStore(s => s.userCountLimit());

  const remainingUserCount = useMemo(
    () => Math.max(0, userCountLimit - userCountInIam),
    [userCountLimit, userCountInIam]
  );

  const projectName = projectId
    ? `${projectNamePrefix}${projectId}`
    : undefined;
  
  const { data: project } = useProject(projectName ?? "", {
    enabled: !!projectName,
  });

  const [memberSearchText, setMemberSearchText] = useState("");

  const hasRequestRoleFeature = useAppStore(s => s.hasFeature(PlanFeature.FEATURE_REQUEST_ROLE_WORKFLOW));

  const loadProjectIamPolicy = useAppStore(s => s.loadProjectIamPolicy);
  // Fetch project IAM policy on mount
  useEffect(() => {
    if (projectName) {
      loadProjectIamPolicy(projectName);
    }
  }, [projectName, loadProjectIamPolicy]);

  const projectIamPolicy = useAppStore(s =>
    projectName ? s.projectPoliciesByName[projectName] : undefined
  );

  const workspaceIamPolicy = useAppStore(s => s.workspacePolicy);
  const memberBindings = useMemo(() =>
    getMemberBindings({
      policies:
        projectName && projectIamPolicy
          ? [{ level: "PROJECT" as const, policy: projectIamPolicy }]
          : [
              {
                level: "WORKSPACE" as const,
                policy: workspaceIamPolicy,
              },
            ],
      searchText: memberSearchText,
      ignoreRoles: EMPTY_ROLE_SET,
    }),
    [projectName, projectIamPolicy, workspaceIamPolicy, memberSearchText]
  );

  const canSetIamPolicy = project
    ? !isDefaultProject(project.name) &&
      project.state !== State.DELETED &&
      hasProjectPermissionV2(project, "bb.projects.setIamPolicy")
    : hasWorkspacePermissionV2("bb.workspaces.setIamPolicy");

  const isSaaSMode = useAppStore(s => s.isSaaSMode());

  const scope: "workspace" | "project" = projectName ? "project" : "workspace";

  return {
    currentUser,
    projectIamPolicy,
    project,
    projectName,
    memberSearchText,
    setMemberSearchText,
    memberBindings,
    canSetIamPolicy,
    isSaaSMode,
    remainingUserCount,
    hasRequestRoleFeature,
    scope,
  };
}

// Re-export for convenience
import { isDefaultProject } from "@/types";
export { isBindingPolicyExpired };
