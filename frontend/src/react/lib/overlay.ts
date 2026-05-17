import { createPortal } from "react-dom";
import { getLayerRoot } from "@/react/components/ui/layer";

/**
 * Type-safe overlay family selector.
 *
 * - "overlay": Standard app dialogs and sheets (default)
 * - "agent": AI agent surfaces — above overlay
 * - "critical": Session expired only — above agent
 *
 * @throws If called outside a mounted DOM (SSR)
 */
export type OverlayFamily = "overlay" | "agent" | "critical";

/**
 * Create a React portal into the correct semantic overlay layer.
 * DO NOT use createPortal(el, document.body) directly.
 *
 * @example
 * return createOverlayPortal(<MyDialog />, "overlay");
 */
export function createOverlayPortal(
  content: React.ReactNode,
  family: OverlayFamily = "overlay"
): React.ReactPortal {
  return createPortal(content, getLayerRoot(family));
}

/**
 * Hook: get the DOM root element for an overlay family.
 * Use when you need the container element directly.
 */
export function useLayerRoot(family: OverlayFamily = "overlay"): HTMLElement {
  return getLayerRoot(family);
}
