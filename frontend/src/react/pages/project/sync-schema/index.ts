/**
 * Sync Schema module — modular decomposition scaffold.
 *
 * Original: ProjectSyncSchemaPage.tsx (2,196 LOC)
 * Target: 6 files, each < 200 LOC
 *
 * Sub-components extracted from original:
 *   - StepIndicator (lines 535-578)
 *   - SourceSchemaStep (lines 579-638)
 *   - DatabaseSchemaSelector (lines 639-738)
 *   - ChangelogSelector (lines 739-915)
 *   - ChangelogLabel (lines 916-934)
 *   - RawSQLEditor (lines 935-1061)
 *   - SourceSchemaInfo (lines 1062-1143)
 *   - SelectTargetDatabasesView (lines 1144-1581)
 *   - DiffViewPanel (lines 1582-1673)
 *   - SchemaDiffViewer (lines 1674-1820)
 *   - SchemaDiffViewerModal (lines 1821-1852)
 *   - MonacoEditorPanel (lines 1853-1946)
 *   - CopyButton (lines 1947-1973)
 *   - TargetDatabasesSelectPanel (lines 1974-2196)
 */
export { useSyncSchemaData } from "./hooks/useSyncSchemaData";
export { useSyncSchemaActions } from "./hooks/useSyncSchemaActions";
