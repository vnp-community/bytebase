/**
 * Instance Form module — modular decomposition scaffold.
 *
 * Original: InstanceFormBody.tsx (1,913 LOC)
 * Target: 6 files, each < 200 LOC
 *
 * Sub-components extracted from original:
 *   - SpannerHostInput (lines 68-164) → components/SpannerHostInput.tsx
 *   - BigQueryHostInput (lines 165-226) → components/BigQueryHostInput.tsx
 *   - InstanceEngineRadioGrid (lines 227-272) → components/EngineRadioGrid.tsx
 *   - ResourceIdField (lines 303-507) → components/ResourceIdField.tsx
 *   - ScanIntervalInput (lines 508-630) → components/ScanIntervalInput.tsx
 *   - SyncDatabases (lines 631-810) → components/SyncDatabases.tsx
 *
 * Engine-specific form dispatch:
 *   InstanceFormBody → switch(engine) → MySQLForm / PostgreSQLForm / CommonFields
 */
export { useInstanceFormData } from "./hooks/useInstanceFormData";
export { useInstanceFormActions } from "./hooks/useInstanceFormActions";
