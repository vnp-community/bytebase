# Adding a New Page — 5-Step Playbook

---

## Step 1: Create the React Component File

**Location**: `src/react/pages/{section}/YourPageName.tsx`

**Section directories:**
- `settings/` — workspace-level settings pages
- `project/` — project-scoped pages
- `auth/` — authentication pages
- `workspace/` — workspace-level pages

**Rules:**
- File name = component name = exported function name
- **Named export ONLY** — `export function YourPageName()` NOT `export default`
- Copy from `src/react/templates/new-page.tsx` for boilerplate

```typescript
// src/react/pages/settings/YourPageName.tsx

export function YourPageName() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-semibold text-main-text">Page Title</h1>
      {/* page content */}
    </div>
  );
}
```

---

## Step 2: Register Route in Router File

**Router file per section:**

| Section | Router File |
|---|---|
| Settings | `src/router/dashboard/workspaceSetting.ts` |
| Project | `src/router/dashboard/projectV1.ts` |
| Workspace | `src/router/dashboard/workspace.ts` |
| Instance | `src/router/dashboard/instance.ts` |
| Auth | `src/router/auth.ts` |

**Add route definition:**

```typescript
{
  path: "your-path",
  name: "workspace.settings.your-page",  // dot-separated namespace
  component: () => import("@/react/ReactPageMount.vue"),
  props: { page: "YourPageName" },  // ← MUST match exported function name exactly
  meta: {
    title: (route) => t("your.page.title"),  // i18n key
    requiredPermissionList: () => ["bb.your.permission"],  // RBAC check
  },
},
```

> ⚠️ `props.page` value MUST exactly match the function name in the `.tsx` file.
> `"YourPageName"` → looks for `export function YourPageName()` in globbed directories.

---

## Step 3: Check Mount Glob (only if new directory)

Open `src/react/mount.ts` and check if your page's directory is already globbed:

```typescript
// Currently globbed directories (auto-discovered):
"./pages/settings/*.tsx"     ← ✅ settings pages
"./pages/project/*.tsx"      ← ✅ project pages
"./pages/workspace/*.tsx"    ← ✅ workspace pages
"./pages/auth/*.tsx"         ← ✅ auth pages
"./components/*.tsx"         ← ✅ shared components
"./components/sql-editor/*.tsx" ← ✅ SQL editor components
"./components/auth/*.tsx"    ← ✅ auth components (specific files)
"./plugins/agent/components/AgentWindow.tsx" ← ✅ agent plugin
```

**If your page IS in one of these directories**: Skip this step.

**If your page is in a NEW directory**: Add a glob pattern to `mount.ts`:
```typescript
const yourNewPageLoaders = import.meta.glob("./pages/your-section/*.tsx");
// Then add to pageLoaders: ...yourNewPageLoaders,
// Also add the directory to pageDirs array for resolution
```

---

## Step 4: Add Navigation Entry (if user needs to click to get there)

**Sidebar navigation:**
- Workspace settings sidebar: defined in route `meta` and rendered by `SettingRouteShell`
- Project sidebar: `src/react/components/ProjectSidebar.tsx`
- Main sidebar: `src/react/components/DashboardSidebar.tsx` or `src/views/DashboardSidebar.vue`

**Example — add to settings sidebar:**
The Settings section uses `SettingRouteShell` which reads route names. Add your route name to the appropriate group in `src/react/pages/settings/SettingRouteShell.tsx`.

---

## Step 5: Verify

```bash
pnpm dev
# Navigate to your new route in the browser
# Check browser console for errors
# Verify:
# 1. Page renders (not blank)
# 2. Permission check works (try as non-admin)
# 3. Document title updates
```

---

## Common Errors + Fixes

| Symptom | Cause | Fix |
|---|---|---|
| **Blank page** | `props.page` doesn't match exported function name | Check route `props: { page: "ExactName" }` matches `export function ExactName()` |
| **404 page** | Route not registered | Add route to correct router file (Step 2) |
| **"Unknown React page" console error** | File not in any glob pattern | Check `mount.ts` glob patterns (Step 3) |
| **Permission denied** | Wrong `requiredPermissionList` | Check permission strings against `src/store/modules/v1/permission.ts` |
| **Page loads but no data** | Missing `useVueState` or query hook | Add data fetching hook in component |
| **Document title not updating** | Missing `meta.title` in route | Add `title: (route) => t("key")` to route meta |
