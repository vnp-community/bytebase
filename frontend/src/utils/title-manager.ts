/**
 * Unified Title Manager (TASK-W-025)
 *
 * Manages document title with source tracking and debounce to prevent
 * title flicker when both Vue and React attempt to set the title.
 *
 * Priority: React > Vue (React title takes precedence when both set).
 */

import { DOCUMENT_TITLE_SEPARATOR, DOCUMENT_TITLE_SUFFIX } from "./document-title";

type TitleSource = "vue" | "react";

interface PendingTitle {
  title: string;
  source: TitleSource;
  timestamp: number;
}

let pendingTitle: PendingTitle | null = null;
let debounceTimer: ReturnType<typeof setTimeout> | null = null;
let currentSource: TitleSource | null = null;

const DEBOUNCE_MS = 50;

/**
 * Set the page title with source tracking and debounce.
 *
 * - If called from React while a Vue title is pending, React wins.
 * - If called from Vue while a React title is pending, Vue is ignored.
 * - All calls are debounced by 50ms to prevent flicker during navigation.
 */
export function setPageTitle(title: string, source: TitleSource): void {
  const now = Date.now();

  // React takes priority: if React already set a title in this cycle, ignore Vue
  if (
    source === "vue" &&
    pendingTitle?.source === "react" &&
    now - pendingTitle.timestamp < DEBOUNCE_MS * 2
  ) {
    return;
  }

  pendingTitle = { title, source, timestamp: now };

  if (debounceTimer) {
    clearTimeout(debounceTimer);
  }

  debounceTimer = setTimeout(() => {
    if (pendingTitle) {
      const fullTitle = pendingTitle.title
        ? `${pendingTitle.title}${DOCUMENT_TITLE_SEPARATOR}${DOCUMENT_TITLE_SUFFIX}`
        : DOCUMENT_TITLE_SUFFIX;
      document.title = fullTitle;
      currentSource = pendingTitle.source;
      pendingTitle = null;
    }
    debounceTimer = null;
  }, DEBOUNCE_MS);
}

/**
 * Reset title tracking — called at the beginning of each navigation.
 * Allows Vue to set a new title without being blocked by a stale React title.
 */
export function resetTitleTracking(): void {
  currentSource = null;
  pendingTitle = null;
  if (debounceTimer) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }
}

/**
 * Get the current title source (for diagnostics).
 */
export function getCurrentTitleSource(): TitleSource | null {
  return currentSource;
}
