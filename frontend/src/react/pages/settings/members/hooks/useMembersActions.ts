import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import type { MemberBinding } from "@/components/Member/types";
import {
  pushNotification,
  useProjectIamPolicyStore,
  useWorkspaceV1Store,
} from "@/store";
import { useAppStore } from "@/react/stores/app";
import { ALL_USERS_USER_EMAIL, userBindingPrefix } from "@/types";
import type { IamPolicy } from "@/types/proto-es/v1/iam_policy_pb";

/**
 * Actions for the MembersPage: grant, revoke, bulk-revoke members.
 */
export function useMembersActions(
  projectName?: string,
  projectIamPolicy?: IamPolicy
) {
  const { t } = useTranslation();
  const workspaceStore = useWorkspaceV1Store();
  const projectIamPolicyStore = useProjectIamPolicyStore();
  const currentUser = useAppStore(s => s.currentUser);

  const [selectedMembers, setSelectedMembers] = useState<string[]>([]);
  const [showEditMemberDrawer, setShowEditMemberDrawer] = useState(false);
  const [editingMember, setEditingMember] = useState<
    MemberBinding | undefined
  >();
  const [showRequestRoleDialog, setShowRequestRoleDialog] = useState(false);

  const handleRevokeSelected = useCallback(async () => {
    if (
      selectedMembers.some(
        (m) => m === `${userBindingPrefix}${currentUser?.email}`
      )
    ) {
      pushNotification({
        module: "bytebase",
        style: "WARN",
        title: t("settings.members.cannot-revoke-self"),
      });
      return;
    }
    if (window.confirm(t("settings.members.revoke-access-alert"))) {
      try {
        if (projectName && projectIamPolicy) {
          const policy = structuredClone(projectIamPolicy);
          for (const binding of policy.bindings) {
            binding.members = binding.members.filter(
              (member) => !selectedMembers.includes(member)
            );
          }
          policy.bindings = policy.bindings.filter((b) => b.members.length > 0);
          await projectIamPolicyStore.updateProjectIamPolicy(
            projectName,
            policy
          );
        } else {
          await workspaceStore.patchIamPolicy(
            selectedMembers.map((m) => ({ member: m, roles: [] }))
          );
        }
        pushNotification({
          module: "bytebase",
          style: "INFO",
          title: t("settings.members.revoked"),
        });
        setSelectedMembers([]);
      } catch {
        // error already shown by store
      }
    }
  }, [
    selectedMembers,
    currentUser?.email,
    projectName,
    projectIamPolicy,
    projectIamPolicyStore,
    workspaceStore,
    t,
  ]);

  const handleMemberUpdateBinding = useCallback((binding: MemberBinding) => {
    setEditingMember(binding);
    setShowEditMemberDrawer(true);
  }, []);

  const handleMemberRevokeBinding = useCallback(
    async (binding: MemberBinding) => {
      try {
        if (projectName && projectIamPolicy) {
          const policy = structuredClone(projectIamPolicy);
          for (const b of policy.bindings) {
            b.members = b.members.filter(
              (member) => member !== binding.binding
            );
          }
          policy.bindings = policy.bindings.filter(
            (b) => b.members.length > 0
          );
          await projectIamPolicyStore.updateProjectIamPolicy(
            projectName,
            policy
          );
        } else {
          await workspaceStore.patchIamPolicy([
            { member: binding.binding, roles: [] },
          ]);
        }
        pushNotification({
          module: "bytebase",
          style: "INFO",
          title: t("settings.members.revoked"),
        });
      } catch {
        // error already shown by store
      }
    },
    [projectName, projectIamPolicy, projectIamPolicyStore, workspaceStore, t]
  );

  const openGrantDrawer = useCallback(() => {
    setEditingMember(undefined);
    setShowEditMemberDrawer(true);
  }, []);

  const closeEditDrawer = useCallback(() => {
    setShowEditMemberDrawer(false);
    setEditingMember(undefined);
  }, []);

  return {
    selectedMembers,
    setSelectedMembers,
    showEditMemberDrawer,
    editingMember,
    showRequestRoleDialog,
    setShowRequestRoleDialog,
    handleRevokeSelected,
    handleMemberUpdateBinding,
    handleMemberRevokeBinding,
    openGrantDrawer,
    closeEditDrawer,
  };
}
