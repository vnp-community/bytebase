/**
 * IDPs module — modular decomposition scaffold.
 *
 * Original: IDPsPage.tsx (2,104 LOC), IDPDetailPage.tsx (1,625 LOC)
 * Target: 11 files, each < 200 LOC
 *
 * Directory structure:
 *   idps/
 *     IDPsPage.tsx           — Container (~80 LOC)
 *     IDPsView.tsx           — Table + empty state (~180 LOC)
 *     hooks/useIDPsData.ts   — List IDPs, feature checks (~80 LOC)
 *     hooks/useIDPsActions.ts — Create, delete, test connection (~100 LOC)
 *     components/IDPsTable.tsx — Table with type badges (~150 LOC)
 *     components/CreateWizardDrawer.tsx — IDP creation wizard (~300 LOC)
 *   idp-detail/
 *     IDPDetailPage.tsx      — Container (~80 LOC)
 *     IDPDetailView.tsx      — Tabs layout (~180 LOC)
 *     hooks/useIDPDetailData.ts — Fetch single IDP (~80 LOC)
 *     hooks/useIDPDetailActions.ts — Update, test connection (~100 LOC)
 *     components/ProviderConfigForm.tsx — Config form (~200 LOC)
 *     components/FieldMappingForm.tsx — Field mapping (~100 LOC)
 *
 * Key extraction points:
 *   - ProviderConfigForm contains OIDC/LDAP/OAuth2 specific fields
 *   - TestConnection logic → isolated in useIDPsActions / useIDPDetailActions
 *   - ExternalURLInfo → shared utility component
 *   - Template list → moved to constants file
 */

// Hooks
export { useIDPsData } from "./hooks/useIDPsData";
export { useIDPsActions } from "./hooks/useIDPsActions";
