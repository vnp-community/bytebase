// i18n: i18next | use t("key") from useTranslation()
// AI: Copy + rename this file to YourPageName.tsx
// RULES:
//   1. Named export MUST match filename (e.g., MembersPage.tsx → export function MembersPage)
//   2. Register route in the correct router file (see .ai-context/NEW_PAGE_PLAYBOOK.md)
//   3. Set props.page in route to match the export name
//   4. Verify the glob pattern in src/react/mount.ts covers this directory

// TODO: Replace "TemplatePage" with your page name
export function TemplatePage() {
  // TODO: Add data hooks
  // const { data, isLoading, error } = useYourData(name);

  // TODO: Add action hooks
  // const { mutate: create } = useCreateYourEntity();
  // const { mutate: update } = useUpdateYourEntity();

  return (
    <div className="flex flex-col gap-4 p-4">
      {/* TODO: Replace with actual page title */}
      <h1 className="text-xl font-semibold text-main-text">
        TODO: Page Title
      </h1>

      {/* TODO: Add page content */}
      <div className="flex flex-col gap-3">
        <p className="text-control-placeholder">Page content goes here.</p>
      </div>
    </div>
  );
}
