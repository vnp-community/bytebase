// i18n: i18next | use t("key") from useTranslation()
import { Plus, ShieldUser } from "lucide-react";
import { useTranslation } from "react-i18next";
import { FeatureBadge } from "@/react/components/FeatureBadge";
import { LearnMoreLink } from "@/react/components/LearnMoreLink";
import { PermissionGuard } from "@/react/components/PermissionGuard";
import { Button } from "@/react/components/ui/button";
import { SearchInput } from "@/react/components/ui/search-input";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/react/components/ui/alert";
import {
  Tabs,
  TabsList,
  TabsPanel,
  TabsTrigger,
} from "@/react/components/ui/tabs";
import { Tooltip } from "@/react/components/ui/tooltip";
import { PlanFeature } from "@/types/proto-es/v1/subscription_service_pb";
import { MemberTable, MemberTableByRole } from "./components/MembersTable";
// EditMemberRoleDrawer remains in the original MembersPage.tsx until fully extracted
// import { EditMemberRoleDrawer } from "../MembersPage";
import { RequestRoleSheet } from "../RequestRoleSheet";
import { useMembersData } from "./hooks/useMembersData";
import { useMembersActions } from "./hooks/useMembersActions";
import { useMembersPermissions } from "./hooks/useMembersPermissions";
import { useState } from "react";

/**
 * MembersPage — Container component.
 * Composes data, action, and permission hooks;
 * delegates rendering to MemberTable / MemberTableByRole.
 */
export function MembersPage({ projectId }: { projectId?: string }) {
  const { t } = useTranslation();
  const [memberViewTab, setMemberViewTab] = useState<"MEMBERS" | "ROLES">(
    "MEMBERS"
  );

  const data = useMembersData(projectId);
  const actions = useMembersActions(
    data.projectName,
    data.projectIamPolicy
  );
  const perms = useMembersPermissions({
    projectName: data.projectName,
    project: data.project,
    canSetIamPolicy: data.canSetIamPolicy,
    hasRequestRoleFeature: data.hasRequestRoleFeature,
  });

  return (
    <div className="w-full px-4 overflow-x-hidden flex flex-col pt-2 pb-4">
      {!data.projectName && data.remainingUserCount <= 3 && (
        <Alert variant="warning" className="mb-2">
          <AlertTitle>{t("subscription.usage.user-count.title")}</AlertTitle>
          <AlertDescription>
            {data.remainingUserCount > 0
              ? t("subscription.usage.user-count.remaining", {
                  total: data.remainingUserCount + data.remainingUserCount, // from store
                  count: data.remainingUserCount,
                })
              : t("subscription.usage.user-count.runoutof", {
                  total: data.remainingUserCount,
                })}{" "}
            {t("subscription.usage.user-count.upgrade")}
          </AlertDescription>
        </Alert>
      )}
      {data.projectName && (
        <div className="textinfolabel mb-4">
          {t("project.members.description")}{" "}
          <LearnMoreLink
            href="https://docs.bytebase.com/administration/roles/?source=console#project-roles"
            className="text-accent"
          />
        </div>
      )}

      <div className="flex items-center justify-between gap-x-2 mb-4">
        <SearchInput
          placeholder={t("settings.members.search-member")}
          value={data.memberSearchText}
          onChange={(e) => data.setMemberSearchText(e.target.value)}
        />
        <div className="flex items-center gap-x-2">
          <PermissionGuard {...perms.setIamPolicyPermissionGuard}>
            {({ disabled }) => (
              <div className="flex items-center gap-x-2">
                {memberViewTab === "MEMBERS" && (
                  <Button
                    variant="outline"
                    disabled={
                      disabled ||
                      !data.canSetIamPolicy ||
                      actions.selectedMembers.length === 0
                    }
                    onClick={actions.handleRevokeSelected}
                  >
                    {t("settings.members.revoke-access")}
                  </Button>
                )}
                <Button
                  disabled={disabled || !data.canSetIamPolicy}
                  onClick={actions.openGrantDrawer}
                >
                  <Plus className="h-4 w-4 mr-1" />
                  {t("settings.members.grant-access")}
                </Button>
              </div>
            )}
          </PermissionGuard>
          {perms.requestRoleButtonState.visible &&
            (perms.requestRoleDisabledReason ? (
              <Tooltip content={perms.requestRoleDisabledReason}>
                <span className="inline-flex">
                  <Button
                    disabled
                    onClick={() => actions.setShowRequestRoleDialog(true)}
                  >
                    {data.hasRequestRoleFeature ? (
                      <ShieldUser className="size-4 mr-1" />
                    ) : (
                      <FeatureBadge
                        feature={PlanFeature.FEATURE_REQUEST_ROLE_WORKFLOW}
                        clickable={false}
                        className="mr-1"
                      />
                    )}
                    {t("issue.title.request-role")}
                  </Button>
                </span>
              </Tooltip>
            ) : (
              <PermissionGuard
                permissions={[...perms.REQUEST_ROLE_REQUIRED_PERMISSIONS]}
                project={data.project}
              >
                {({ disabled }) => (
                  <Button
                    disabled={disabled}
                    onClick={() => actions.setShowRequestRoleDialog(true)}
                  >
                    <ShieldUser className="size-4 mr-1" />
                    {t("issue.title.request-role")}
                  </Button>
                )}
              </PermissionGuard>
            ))}
        </div>
      </div>

      <Tabs
        value={memberViewTab}
        onValueChange={(v) => setMemberViewTab(v as "MEMBERS" | "ROLES")}
      >
        <TabsList>
          <TabsTrigger value="MEMBERS">
            {t("settings.members.view-by-members")}
          </TabsTrigger>
          <TabsTrigger value="ROLES">
            {t("settings.members.view-by-roles")}
          </TabsTrigger>
        </TabsList>
        <TabsPanel value="MEMBERS">
          <div className="py-4">
            <MemberTable
              bindings={data.memberBindings}
              allowEdit={data.canSetIamPolicy}
              selectedBindings={actions.selectedMembers}
              onSelectionChange={actions.setSelectedMembers}
              onUpdateBinding={actions.handleMemberUpdateBinding}
              onRevokeBinding={actions.handleMemberRevokeBinding}
              scope={data.scope}
            />
          </div>
        </TabsPanel>
        <TabsPanel value="ROLES">
          <div className="py-4">
            <MemberTableByRole
              bindings={data.memberBindings}
              allowEdit={data.canSetIamPolicy}
              onUpdateBinding={actions.handleMemberUpdateBinding}
              onRevokeBinding={actions.handleMemberRevokeBinding}
              scope={data.scope}
            />
          </div>
        </TabsPanel>
      </Tabs>

      {data.project && (
        <RequestRoleSheet
          open={actions.showRequestRoleDialog}
          project={data.project}
          onClose={() => actions.setShowRequestRoleDialog(false)}
        />
      )}

      {actions.showEditMemberDrawer && (
        <EditMemberRoleDrawer
          member={actions.editingMember}
          projectName={data.projectName}
          onClose={actions.closeEditDrawer}
        />
      )}
    </div>
  );
}
