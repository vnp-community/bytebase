import type { ReactCoreDeps, ReactRoot } from "./bridge-types";

// Use import.meta.glob so vue-tsc does not follow the import into React .tsx files.
const sidebarLoader = import.meta.glob("./components/ProjectSidebar.tsx");

type SidebarComponent = (props: Record<string, unknown>) => React.ReactNode;

let cachedDeps: ReactCoreDeps | null = null;
let cachedSidebar: SidebarComponent | null = null;

async function loadCoreDeps(): Promise<ReactCoreDeps> {
  if (cachedDeps) return cachedDeps;
  const [
    { createElement, StrictMode },
    { createRoot },
    { I18nextProvider },
    i18nModule,
  ] = await Promise.all([
    import("react"),
    import("react-dom/client"),
    import("react-i18next"),
    import("@/react/i18n"),
  ]);
  await i18nModule.i18nReady;
  cachedDeps = {
    createElement,
    StrictMode,
    createRoot,
    I18nextProvider,
    i18n: i18nModule.default,
  } as unknown as ReactCoreDeps;
  return cachedDeps;
}

async function loadSidebar(): Promise<SidebarComponent> {
  if (cachedSidebar) return cachedSidebar;
  const loader = sidebarLoader["./components/ProjectSidebar.tsx"];
  if (!loader) throw new Error("ProjectSidebar not found");
  const mod = (await loader()) as Record<string, unknown>;
  cachedSidebar = mod.ProjectSidebar as SidebarComponent;
  return cachedSidebar;
}

export async function mountProjectSidebar(
  container: HTMLElement,
  locale: string,
  signal?: AbortSignal
): Promise<ReactRoot> {
  const [deps, ProjectSidebar] = await Promise.all([
    loadCoreDeps(),
    loadSidebar(),
  ]);
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");

  if (deps.i18n.language !== locale) {
    await deps.i18n.changeLanguage(locale);
  }
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");

  const tree = deps.createElement(
    deps.StrictMode,
    null,
    deps.createElement(
      deps.I18nextProvider as Parameters<typeof deps.createElement>[0],
      { i18n: deps.i18n },
      deps.createElement(ProjectSidebar as Parameters<typeof deps.createElement>[0])
    )
  );
  const root = deps.createRoot(container);
  root.render(tree);
  return root;
}

export async function updateProjectSidebarLocale(
  root: ReactRoot,
  locale: string
): Promise<void> {
  const [deps, ProjectSidebar] = await Promise.all([
    loadCoreDeps(),
    loadSidebar(),
  ]);
  if (deps.i18n.language !== locale) {
    await deps.i18n.changeLanguage(locale);
  }
  const tree = deps.createElement(
    deps.StrictMode,
    null,
    deps.createElement(
      deps.I18nextProvider as Parameters<typeof deps.createElement>[0],
      { i18n: deps.i18n },
      deps.createElement(ProjectSidebar as Parameters<typeof deps.createElement>[0])
    )
  );
  root.render(tree);
}
