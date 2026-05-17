import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { Project } from "@/types/proto-es/v1/project_service_pb";
import {
  getRequestRoleButtonState,
  REQUEST_ROLE_REQUIRED_PERMISSIONS,
} from "../requestRoleButton";
import { getSetIamPolicyPermissionGuardConfig } from "../membersPageActions";

const assertNever = (value: never): never => {
  throw new Error(`Unexpected value: ${String(value)}`);
};

/**
 * Computes permission states for the MembersPage action bar.
 */
export function useMembersPermissions({
  projectName,
  project,
  canSetIamPolicy,
  hasRequestRoleFeature,
}: {
  projectName?: string;
  project?: Project;
  canSetIamPolicy: boolean;
  hasRequestRoleFeature: boolean;
}) {
  const { t } = useTranslation();

  const requestRoleButtonState = useMemo(
    () =>
      getRequestRoleButtonState({
        projectName,
        projectReady: !!project,
        allowRequestRole: project?.allowRequestRole ?? false,
        canSetIamPolicy,
        hasRequestRoleFeature,
      }),
    [projectName, project, canSetIamPolicy, hasRequestRoleFeature]
  );

  const requestRoleDisabledReason = useMemo(() => {
    const reason = requestRoleButtonState.disabledReason;
    if (!reason) return undefined;

    switch (reason.kind) {
      case "loading":
        return t("common.loading");
      case "allow-request-role-disabled":
        return t(
          "project.members.request-role.disabled-reason.allow-request-role-disabled"
        );
      case "can-grant-access-directly":
        return t(
          "project.members.request-role.disabled-reason.can-grant-access-directly",
          {
            permission: reason.permission,
          }
        );
      case "feature-unavailable":
        return t(
          "project.members.request-role.disabled-reason.feature-unavailable"
        );
      default:
        return assertNever(reason);
    }
  }, [requestRoleButtonState.disabledReason, t]);

  const setIamPolicyPermissionGuard = useMemo(
    () => getSetIamPolicyPermissionGuardConfig(project),
    [project]
  );

  return {
    requestRoleButtonState,
    requestRoleDisabledReason,
    setIamPolicyPermissionGuard,
    REQUEST_ROLE_REQUIRED_PERMISSIONS,
  };
}
