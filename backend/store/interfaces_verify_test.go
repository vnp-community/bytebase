package store

// Compile-time verification: *Store satisfies all domain interfaces.
// If any method signature changes, the build will fail here.
var _ UserReader = (*Store)(nil)
var _ UserWriter = (*Store)(nil)
var _ UserStore = (*Store)(nil)
var _ ProjectReader = (*Store)(nil)
var _ ProjectWriter = (*Store)(nil)
var _ PlanReader = (*Store)(nil)
var _ IssueReader = (*Store)(nil)
var _ DatabaseReader = (*Store)(nil)
var _ InstanceReader = (*Store)(nil)
var _ PolicyReader = (*Store)(nil)
var _ SettingReader = (*Store)(nil)
var _ WorkspaceReader = (*Store)(nil)
var _ AuditLogWriter = (*Store)(nil)
var _ DBSchemaReader = (*Store)(nil)
var _ SheetReader = (*Store)(nil)
var _ RoleReader = (*Store)(nil)
var _ ChangelogReader = (*Store)(nil)
var _ DataStore = (*Store)(nil)
