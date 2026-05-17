// Package mock provides mock implementations for store interfaces.
//
// To regenerate mocks after interface changes, run from the project root:
//
//	go generate ./backend/store/mock/...
//
// Prerequisites:
//
//	go install go.uber.org/mock/mockgen@latest
package mock

//go:generate mockgen -package mock -destination mock_store.go github.com/bytebase/bytebase/backend/store UserReader,UserWriter,UserStore,ProjectReader,ProjectWriter,PlanReader,IssueReader,DatabaseReader,DatabaseWriter,InstanceReader,PolicyReader,SettingReader,WorkspaceReader,AuditLogWriter,DBSchemaReader,SheetReader,RoleReader,ChangelogReader,TaskStore,TaskRunStore,QueryHistoryStore,AccessGrantReader,ExportArchiveReader,AccountReader,SignalWriter,PlanWebhookWriter,SyncHistoryReader,AuthStore,DataStore
