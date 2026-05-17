import type { ReactNode } from "react";

/**
 * Base props shared by all bridge-mounted React pages.
 */
export interface BridgePageBaseProps {
  /** Additional key-value props forwarded from the Vue host. */
  [key: string]: unknown;
}

/**
 * A React page component that can be mounted via the bridge.
 * Replaces the untyped `(props: any) => any` pattern.
 */
export type ReactPageComponent<P extends BridgePageBaseProps = BridgePageBaseProps> =
  (props: P) => ReactNode;

/**
 * Core React dependencies loaded once and cached.
 * Replaces the untyped `ReactDeps = any` pattern.
 */
export interface ReactCoreDeps {
  createElement: typeof import("react").createElement;
  StrictMode: typeof import("react").StrictMode;
  createRoot: (container: Element | DocumentFragment) => ReactRoot;
  I18nextProvider: (props: { i18n: ReactI18nInstance; children?: ReactNode }) => ReactNode;
  QueryProvider: (props: { children?: ReactNode }) => ReactNode;
  i18n: ReactI18nInstance;
}

/**
 * Minimal i18n instance shape used by the bridge.
 */
export interface ReactI18nInstance {
  language: string;
  changeLanguage: (lng: string) => Promise<void>;
}

/**
 * Minimal React Root shape returned by createRoot.
 */
export interface ReactRoot {
  render: (element: ReactNode) => void;
  unmount: () => void;
}

/**
 * Typed mount function signature.
 * Replaces untyped dynamic mount calls.
 */
export type MountFunction<P extends BridgePageBaseProps = BridgePageBaseProps> = (
  container: HTMLElement,
  page: string,
  props?: P,
  signal?: AbortSignal,
) => Promise<ReactRoot>;
