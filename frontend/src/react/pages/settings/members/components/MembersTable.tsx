// i18n: i18next | use t("key") from useTranslation()
import React, { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Building2,
  ChevronDown,
  ChevronRight,
  Pencil,
  Trash2,
  Users,
} from "lucide-react";
import { groupProjectRoleBindings } from "@/components/Member/projectRoleBindings";
import type { MemberBinding } from "@/components/Member/types";
import { UserAvatar } from "@/react/components/UserAvatar";
import { Badge } from "@/react/components/ui/badge";
import { Button } from "@/react/components/ui/button";
import { useAppStore } from "@/react/stores/app";
import { cn } from "@/react/lib/utils";
import { State } from "@/types/proto-es/v1/common_pb";
import type { Binding } from "@/types/proto-es/v1/iam_policy_pb";
import { AccountType, getAccountTypeByEmail } from "@/types/v1/user";
import {
  displayRoleTitle,
  isBindingPolicyExpired,
  sortRoles,
} from "@/utils";

// ============================================================
// Shared helpers
// ============================================================

function MemberAccountCell({
  mb,
  showExpired,
}: {
  mb: MemberBinding;
  showExpired?: boolean;
}) {
  const { t } = useTranslation();
  const currentUser = useAppStore(s => s.currentUser);
  const isSaaSMode = useAppStore(s => s.isSaaSMode());

  return (
    <div className="flex items-center gap-x-3">
      {mb.type === "users" ? (
        <UserAvatar title={mb.title || mb.user?.email || "?"} size={showExpired ? "sm" : undefined} />
      ) : (
        <div className={cn(
          "rounded-full bg-control-bg-hover flex items-center justify-center shrink-0",
          showExpired ? "size-7" : "size-9"
        )}>
          <Users className={cn("text-control-light", showExpired ? "size-3.5" : "size-4")} />
        </div>
      )}
      <div className="flex flex-col">
        <div className="flex items-center gap-x-1.5">
          <span className={cn("font-medium text-accent", showExpired && "line-through")}>
            {mb.title}
          </span>
          {showExpired && (
            <Badge variant="destructive" className="text-xs">
              {t("common.expired")}
            </Badge>
          )}
          {mb.type === "users" && mb.user?.name === currentUser?.name && (
            <Badge className="text-xs">{t("common.you")}</Badge>
          )}
          {isSaaSMode && mb.type === "users" && mb.pending && (
            <Badge variant="warning" className="text-xs">
              {t("settings.members.pending-invite")}
            </Badge>
          )}
          {mb.type === "users" &&
            mb.user?.email &&
            getAccountTypeByEmail(mb.user.email) ===
              AccountType.SERVICE_ACCOUNT && (
              <Badge variant="secondary" className="text-xs">
                {t("settings.members.service-account")}
              </Badge>
            )}
          {mb.type === "users" &&
            mb.user?.email &&
            getAccountTypeByEmail(mb.user.email) ===
              AccountType.WORKLOAD_IDENTITY && (
              <Badge variant="secondary" className="text-xs">
                {t("settings.members.workload-identity")}
              </Badge>
            )}
          {mb.group && (
            <span className="text-control-light text-xs">
              ({mb.group.members.length}{" "}
              {t("common.members", { count: mb.group.members.length })})
            </span>
          )}
          {mb.group?.deleted && (
            <Badge variant="destructive" className="text-xs">
              {t("common.deleted")}
            </Badge>
          )}
        </div>
        <span className="text-control-light text-xs">
          {mb.type === "users"
            ? mb.user?.email
            : mb.binding.replace("group:", "groups/")}
        </span>
      </div>
    </div>
  );
}

function MemberActionButtons({
  mb,
  allowEdit,
  onUpdate,
  onRevoke,
}: {
  mb: MemberBinding;
  allowEdit: boolean;
  onUpdate: (binding: MemberBinding) => void;
  onRevoke: (binding: MemberBinding) => void;
}) {
  const { t } = useTranslation();

  const canEdit =
    mb.type === "users"
      ? mb.user?.state !== State.DELETED
      : mb.type === "groups"
        ? !mb.group?.deleted
        : true;

  if (!allowEdit) return null;

  return (
    <div className="flex items-center gap-x-1">
      {canEdit && (
        <Button variant="ghost" size="icon" onClick={() => onUpdate(mb)}>
          <Pencil className="h-4 w-4" />
        </Button>
      )}
      <Button
        variant="ghost"
        size="icon"
        onClick={() => {
          if (window.confirm(t("settings.members.revoke-access-alert"))) {
            onRevoke(mb);
          }
        }}
      >
        <Trash2 className="h-4 w-4" />
      </Button>
    </div>
  );
}

// ============================================================
// MemberTable (view by members)
// ============================================================

export function MemberTable({
  bindings,
  allowEdit,
  selectedBindings,
  onSelectionChange,
  onUpdateBinding,
  onRevokeBinding,
  scope,
}: {
  bindings: MemberBinding[];
  allowEdit: boolean;
  selectedBindings: string[];
  onSelectionChange: (selected: string[]) => void;
  onUpdateBinding: (binding: MemberBinding) => void;
  onRevokeBinding: (binding: MemberBinding) => void;
  scope: "workspace" | "project";
}) {
  const { t } = useTranslation();

  const selectableBindings = useMemo(
    () =>
      bindings.filter(
        (b) => scope !== "project" || b.projectRoleBindings.length > 0
      ),
    [bindings, scope]
  );

  useEffect(() => {
    const visibleNames = new Set(bindings.map((b) => b.binding));
    const next = selectedBindings.filter((b) => visibleNames.has(b));
    if (next.length !== selectedBindings.length) {
      onSelectionChange(next);
    }
  }, [bindings, selectedBindings, onSelectionChange]);

  const allSelected =
    selectableBindings.length > 0 &&
    selectableBindings.every((b) => selectedBindings.includes(b.binding));

  const toggleAll = () => {
    if (allSelected) {
      onSelectionChange([]);
    } else {
      onSelectionChange(selectableBindings.map((b) => b.binding));
    }
  };

  const toggleOne = (binding: string) => {
    onSelectionChange(
      selectedBindings.includes(binding)
        ? selectedBindings.filter((b) => b !== binding)
        : [...selectedBindings, binding]
    );
  };

  const isSelectDisabled = (mb: MemberBinding) =>
    scope === "project" && mb.projectRoleBindings.length === 0;

  const renderProjectRoleSummary = (roleBindings: Binding[]) => {
    const active = roleBindings.filter((b) => !isBindingPolicyExpired(b));
    const expired = roleBindings.filter((b) => isBindingPolicyExpired(b));
    return [
      ...groupProjectRoleBindings(active).map((group) => (
        <Badge key={`active-${group.role}`} className="text-xs gap-x-1">
          {displayRoleTitle(group.role)}
          {group.bindings.length > 1 && (
            <span className="text-control-light">
              ({group.bindings.length})
            </span>
          )}
        </Badge>
      )),
      ...groupProjectRoleBindings(expired).map((group) => (
        <Badge
          key={`expired-${group.role}`}
          className="text-xs gap-x-1 line-through opacity-60"
        >
          {displayRoleTitle(group.role)}
          {group.bindings.length > 1 && (
            <span className="text-control-light">
              ({group.bindings.length})
            </span>
          )}
        </Badge>
      )),
    ];
  };

  return (
    <div className="border rounded-sm overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-control-bg border-b">
            {allowEdit && (
              <th className="w-10 px-3 py-2">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                />
              </th>
            )}
            <th className="px-4 py-2 text-left font-medium text-control-light">
              {t("settings.members.table.account")}
            </th>
            <th className="px-4 py-2 text-left font-medium text-control-light">
              {t("settings.members.table.roles")}
            </th>
            <th className="w-24 px-4 py-2 text-left font-medium text-control-light">
              {t("common.operations")}
            </th>
          </tr>
        </thead>
        <tbody>
          {bindings.map((mb) => (
            <tr
              key={mb.binding}
              className="border-b last:border-b-0 hover:bg-control-bg"
            >
              {allowEdit && (
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    checked={selectedBindings.includes(mb.binding)}
                    disabled={isSelectDisabled(mb)}
                    onChange={() => toggleOne(mb.binding)}
                  />
                </td>
              )}
              <td className="px-4 py-2">
                <MemberAccountCell mb={mb} />
              </td>
              <td className="px-4 py-2">
                <div className="flex flex-wrap gap-1">
                  {scope === "project"
                    ? renderProjectRoleSummary(mb.projectRoleBindings)
                    : sortRoles([...mb.workspaceLevelRoles]).map((role) => (
                        <Badge key={role} className="text-xs gap-x-1">
                          <Building2 className="h-3 w-3" />
                          {displayRoleTitle(role)}
                        </Badge>
                      ))}
                </div>
              </td>
              <td className="px-4 py-2">
                <MemberActionButtons
                  mb={mb}
                  allowEdit={allowEdit}
                  onUpdate={onUpdateBinding}
                  onRevoke={onRevokeBinding}
                />
              </td>
            </tr>
          ))}
          {bindings.length === 0 && (
            <tr>
              <td
                colSpan={allowEdit ? 4 : 3}
                className="px-4 py-8 text-center text-control-light"
              >
                {t("common.no-data")}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

// ============================================================
// MemberTableByRole (view by roles)
// ============================================================

export function MemberTableByRole({
  bindings,
  allowEdit,
  onUpdateBinding,
  onRevokeBinding,
  scope,
}: {
  bindings: MemberBinding[];
  allowEdit: boolean;
  onUpdateBinding: (binding: MemberBinding) => void;
  onRevokeBinding: (binding: MemberBinding) => void;
  scope: "workspace" | "project";
}) {
  const { t } = useTranslation();
  const [expandedRoles, setExpandedRoles] = useState<Set<string>>(new Set());
  const initializedRef = useRef(false);

  type RoleMember = { member: MemberBinding; allExpired: boolean };
  const roleToBindings = useMemo(() => {
    const map = new Map<string, RoleMember[]>();
    const appendToRole = (role: string, entry: RoleMember) => {
      const arr = map.get(role) ?? [];
      arr.push(entry);
      map.set(role, arr);
    };
    for (const mb of bindings) {
      if (scope === "project") {
        const bindingsByRole = new Map<string, Binding[]>();
        for (const b of mb.projectRoleBindings) {
          const arr = bindingsByRole.get(b.role) ?? [];
          arr.push(b);
          bindingsByRole.set(b.role, arr);
        }
        for (const [role, roleBindings] of bindingsByRole) {
          const allExpired = roleBindings.every((b) =>
            isBindingPolicyExpired(b)
          );
          appendToRole(role, { member: mb, allExpired });
        }
      } else {
        for (const role of mb.workspaceLevelRoles) {
          appendToRole(role, { member: mb, allExpired: false });
        }
      }
    }
    const sortedRoles = sortRoles([...map.keys()]);
    return sortedRoles.map((role) => ({
      role,
      members: (map.get(role) ?? []).slice().sort((a, b) => {
        return Number(a.allExpired) - Number(b.allExpired);
      }),
    }));
  }, [bindings, scope]);

  useEffect(() => {
    if (!initializedRef.current && roleToBindings.length > 0) {
      initializedRef.current = true;
      setExpandedRoles(new Set(roleToBindings.map((r) => r.role)));
    }
  }, [roleToBindings]);

  const toggleRole = (role: string) => {
    setExpandedRoles((prev) => {
      const next = new Set(prev);
      if (next.has(role)) next.delete(role);
      else next.add(role);
      return next;
    });
  };

  return (
    <div className="border rounded-sm overflow-hidden">
      <table className="w-full text-sm">
        <tbody>
          {roleToBindings.map(({ role, members }) => {
            const expanded = expandedRoles.has(role);
            return (
              <React.Fragment key={role}>
                <tr
                  className="bg-control-bg border-b cursor-pointer hover:bg-control-bg"
                  onClick={() => toggleRole(role)}
                >
                  <td colSpan={3} className="px-4 py-2">
                    <div className="flex items-center gap-x-2">
                      {expanded ? (
                        <ChevronDown className="h-4 w-4" />
                      ) : (
                        <ChevronRight className="h-4 w-4" />
                      )}
                      {scope === "workspace" && (
                        <Building2 className="h-4 w-4 text-control-light" />
                      )}
                      <span className="font-medium">
                        {displayRoleTitle(role)}
                      </span>
                      <span className="text-control-light text-xs">
                        ({members.length})
                      </span>
                    </div>
                  </td>
                </tr>
                {expanded &&
                  members.map(({ member: mb, allExpired }) => (
                    <tr
                      key={`${role}-${mb.binding}`}
                      className={cn(
                        "border-b last:border-b-0 hover:bg-control-bg",
                        allExpired && "opacity-60"
                      )}
                    >
                      <td className="px-4 py-2 pl-10">
                        <MemberAccountCell mb={mb} showExpired={allExpired} />
                      </td>
                      <td className="px-4 py-2" />
                      <td className="w-24 px-4 py-2">
                        <MemberActionButtons
                          mb={mb}
                          allowEdit={allowEdit}
                          onUpdate={onUpdateBinding}
                          onRevoke={onRevokeBinding}
                        />
                      </td>
                    </tr>
                  ))}
              </React.Fragment>
            );
          })}
          {roleToBindings.length === 0 && (
            <tr>
              <td
                colSpan={3}
                className="px-4 py-8 text-center text-control-light"
              >
                {t("common.no-data")}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
