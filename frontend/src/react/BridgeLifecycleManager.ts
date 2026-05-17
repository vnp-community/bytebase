import type {
  BridgePageBaseProps,
  ReactRoot,
} from "./bridge-types";

interface MountRequest {
  pageName: string;
  props?: BridgePageBaseProps;
}

/**
 * Manages the lifecycle of bridge-mounted React pages.
 *
 * Key responsibilities:
 * - Cancellable mount: new mount aborts any pending mount via AbortController.
 * - Unmounts previous root before creating a new one.
 * - Wraps every page in ReactErrorBoundary (via mount.ts buildTree).
 * - destroy() clears page cache and unmounts everything.
 */
export class BridgeLifecycleManager {
  private currentRoot: ReactRoot | null = null;
  private pendingAbort: AbortController | null = null;
  private currentPage = "";

  /**
   * Mount a React page into the given container.
   * Cancels any pending mount automatically.
   */
  async mount(
    container: HTMLElement,
    request: MountRequest,
    externalSignal?: AbortSignal
  ): Promise<void> {
    // Cancel previous pending mount
    this.pendingAbort?.abort();
    const abort = new AbortController();
    this.pendingAbort = abort;

    // Combine with external signal if provided
    const signal = externalSignal
      ? this.combineSignals(abort.signal, externalSignal)
      : abort.signal;

    const { mountReactPage, updateReactPage } = await import("./mount");
    if (signal.aborted) return;

    // If same page, just update props
    if (this.currentRoot && this.currentPage === request.pageName) {
      await updateReactPage(this.currentRoot, request.pageName, request.props);
      return;
    }

    // Unmount previous root before creating new
    this.unmountCurrent();
    if (signal.aborted) return;

    const root = await mountReactPage(
      container,
      request.pageName,
      request.props
    );
    if (signal.aborted) {
      root?.unmount();
      return;
    }

    this.currentRoot = root;
    this.currentPage = request.pageName;
    this.pendingAbort = null;
  }

  /**
   * Re-render current page with new props without remounting.
   */
  async update(page: string, props?: BridgePageBaseProps): Promise<void> {
    if (!this.currentRoot) return;
    const { updateReactPage } = await import("./mount");
    await updateReactPage(this.currentRoot, page, props);
  }

  /**
   * Unmount the current React root if any.
   */
  unmountCurrent(): void {
    if (this.currentRoot) {
      this.currentRoot.unmount();
      this.currentRoot = null;
      this.currentPage = "";
    }
  }

  /**
   * Full teardown: abort pending, unmount, and clear page cache.
   */
  async destroy(): Promise<void> {
    this.pendingAbort?.abort();
    this.pendingAbort = null;
    this.unmountCurrent();
    const { clearPageCache } = await import("./mount");
    clearPageCache();
  }

  private combineSignals(
    ...signals: AbortSignal[]
  ): AbortSignal {
    const controller = new AbortController();
    for (const signal of signals) {
      if (signal.aborted) {
        controller.abort();
        return controller.signal;
      }
      signal.addEventListener("abort", () => controller.abort(), { once: true });
    }
    return controller.signal;
  }
}

/** Singleton instance for app-wide use. */
export const bridgeManager = new BridgeLifecycleManager();
