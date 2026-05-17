import { create } from "zustand";
import { devtools } from "zustand/middleware";

/**
 * SQL Editor session store — ephemeral tab/connection state.
 * NOT persisted — resets on page reload.
 */

interface SQLEditorTab {
  id: string;
  title: string;
  statement: string;
  connectionName: string; // instance or database name
  mode: "READONLY" | "ADMIN";
}

interface SQLEditorState {
  activeTabId: string | null;
  tabs: SQLEditorTab[];
  isExecuting: boolean;
}

interface SQLEditorActions {
  addTab: (tab: SQLEditorTab) => void;
  removeTab: (tabId: string) => void;
  setActiveTab: (tabId: string) => void;
  updateTabStatement: (tabId: string, statement: string) => void;
  updateTabConnection: (tabId: string, connectionName: string) => void;
  setIsExecuting: (v: boolean) => void;
  clearTabs: () => void;
}

export type SQLEditorStore = SQLEditorState & SQLEditorActions;

export const useSQLEditorStore = create<SQLEditorStore>()(
  devtools(
    (set) => ({
      // State
      activeTabId: null,
      tabs: [],
      isExecuting: false,

      // Actions
      addTab: (tab) =>
        set(
          (s) => ({
            tabs: [...s.tabs, tab],
            activeTabId: tab.id,
          }),
          false,
          "addTab"
        ),
      removeTab: (tabId) =>
        set(
          (s) => {
            const newTabs = s.tabs.filter((t) => t.id !== tabId);
            return {
              tabs: newTabs,
              activeTabId:
                s.activeTabId === tabId
                  ? newTabs[newTabs.length - 1]?.id ?? null
                  : s.activeTabId,
            };
          },
          false,
          "removeTab"
        ),
      setActiveTab: (tabId) =>
        set({ activeTabId: tabId }, false, "setActiveTab"),
      updateTabStatement: (tabId, statement) =>
        set(
          (s) => ({
            tabs: s.tabs.map((t) =>
              t.id === tabId ? { ...t, statement } : t
            ),
          }),
          false,
          "updateTabStatement"
        ),
      updateTabConnection: (tabId, connectionName) =>
        set(
          (s) => ({
            tabs: s.tabs.map((t) =>
              t.id === tabId ? { ...t, connectionName } : t
            ),
          }),
          false,
          "updateTabConnection"
        ),
      setIsExecuting: (v) =>
        set({ isExecuting: v }, false, "setIsExecuting"),
      clearTabs: () =>
        set({ tabs: [], activeTabId: null }, false, "clearTabs"),
    }),
    { name: "bb-sql-editor" }
  )
);
