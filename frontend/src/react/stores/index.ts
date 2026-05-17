/**
 * Zustand stores barrel — client-only state management.
 *
 * Architecture:
 *   stores/app/     — combined app store (auth, iam, instance, workspace, etc.)
 *   stores/auth.ts  — standalone auth store for React-only components
 *   stores/ui.ts    — UI preferences (persisted)
 *   stores/sqlEditor.ts — SQL Editor session state (ephemeral)
 *
 * For server data, use TanStack Query hooks from "@/react/hooks/queries".
 */

// Existing combined app store
export { createAppStore } from "./app";
export type { AppStoreState, ProjectListParams } from "./app";

// Standalone React stores (new)
export { useAuthStore, syncAuthFromVue } from "./auth";
export type { AuthStore } from "./auth";

export { useUIStore } from "./ui";
export type { UIStore } from "./ui";

export { useSQLEditorStore } from "./sqlEditor";
export type { SQLEditorStore } from "./sqlEditor";
