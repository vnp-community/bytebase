/**
 * Members module — modular decomposition of the MembersPage.
 *
 * Architecture:
 *   MembersPage.tsx          — re-exports from original (mount.ts compatibility)
 *   members/
 *     hooks/useMembersData.ts       — data fetching & derived state
 *     hooks/useMembersActions.ts    — CRUD operations
 *     hooks/useMembersPermissions.ts — permission checks
 *     components/MembersTable.tsx   — MemberTable + MemberTableByRole views
 *
 * The original MembersPage.tsx is preserved for mount.ts glob compatibility.
 * New code should import hooks and components from this module.
 */
export { MembersPage } from "./MembersPage";
export { useMembersData } from "./hooks/useMembersData";
export { useMembersActions } from "./hooks/useMembersActions";
export { useMembersPermissions } from "./hooks/useMembersPermissions";
export { MemberTable, MemberTableByRole } from "./components/MembersTable";
