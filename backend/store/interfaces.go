package store

import (
	"context"
	"database/sql"
	"time"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store/model"
)

// Compile-time verification: *Store satisfies DataStore interface.
var _ DataStore = (*Store)(nil)

// =============================================================================
// USER DOMAIN (principal.go)
// =============================================================================

// UserReader provides read access to user data.
type UserReader interface {
	GetUserByID(ctx context.Context, id int) (*UserMessage, error)
	GetUserByEmail(ctx context.Context, email string) (*UserMessage, error)
	BatchGetUsersByEmails(ctx context.Context, workspace string, emails []string) ([]*UserMessage, error)
	ListUsers(ctx context.Context, find *FindUserMessage) ([]*UserMessage, error)
}

// UserWriter provides write access to user data.
type UserWriter interface {
	CreateUser(ctx context.Context, create *UserMessage) (*UserMessage, error)
	UpdateUser(ctx context.Context, currentUser *UserMessage, patch *UpdateUserMessage) (*UserMessage, error)
	UpdateUserEmail(ctx context.Context, user *UserMessage, newEmail string) (*UserMessage, error)
}

// UserStore provides full user data access.
type UserStore interface {
	UserReader
	UserWriter
}

// =============================================================================
// PROJECT DOMAIN (project.go)
// =============================================================================

// ProjectReader provides read access to project data.
type ProjectReader interface {
	GetDefaultProjectID(ctx context.Context, workspace string) (string, error)
	GetProject(ctx context.Context, find *FindProjectMessage) (*ProjectMessage, error)
	ListProjects(ctx context.Context, find *FindProjectMessage) ([]*ProjectMessage, error)
}

// ProjectWriter provides write access to project data.
type ProjectWriter interface {
	CreateProject(ctx context.Context, create *ProjectMessage, creator *UserMessage) (*ProjectMessage, error)
}

// =============================================================================
// PLAN DOMAIN (plan.go)
// =============================================================================

// PlanReader provides read access to plan data.
type PlanReader interface {
	GetPlan(ctx context.Context, find *FindPlanMessage) (*PlanMessage, error)
	CreatePlan(ctx context.Context, plan *PlanMessage, creator string) (*PlanMessage, error)
	ListPlans(ctx context.Context, find *FindPlanMessage) ([]*PlanMessage, error)
	UpdatePlan(ctx context.Context, patch *UpdatePlanMessage) (*PlanMessage, error)
}

// =============================================================================
// ISSUE DOMAIN (issue.go)
// =============================================================================

// IssueReader provides read access to issue data.
type IssueReader interface {
	GetIssue(ctx context.Context, find *FindIssueMessage) (*IssueMessage, error)
	ListIssues(ctx context.Context, find *FindIssueMessage) ([]*IssueMessage, error)
	CreateIssue(ctx context.Context, create *IssueMessage) (*IssueMessage, error)
	UpdateIssue(ctx context.Context, projectID string, uid int64, patch *UpdateIssueMessage) (*IssueMessage, error)
	CreateIssueComments(ctx context.Context, creator string, creates ...*IssueCommentMessage) (*IssueCommentMessage, error)
}

// =============================================================================
// DATABASE DOMAIN (database.go)
// =============================================================================

// DatabaseReader provides read access to database data.
type DatabaseReader interface {
	GetDatabase(ctx context.Context, find *FindDatabaseMessage) (*DatabaseMessage, error)
	ListDatabases(ctx context.Context, find *FindDatabaseMessage) ([]*DatabaseMessage, error)
}

// DatabaseWriter provides write access to database data.
type DatabaseWriter interface {
	UpdateDatabase(ctx context.Context, patch *UpdateDatabaseMessage) (*DatabaseMessage, error)
}

// =============================================================================
// INSTANCE DOMAIN (instance.go)
// =============================================================================

// InstanceReader provides read access to instance data.
type InstanceReader interface {
	GetInstance(ctx context.Context, find *FindInstanceMessage) (*InstanceMessage, error)
	ListInstances(ctx context.Context, find *FindInstanceMessage) ([]*InstanceMessage, error)
	CreateInstance(ctx context.Context, instanceCreate *InstanceMessage) (*InstanceMessage, error)
}

// =============================================================================
// POLICY DOMAIN (policy.go)
// =============================================================================

// PolicyReader provides read access to policy data.
type PolicyReader interface {
	GetPolicy(ctx context.Context, find *FindPolicyMessage) (*PolicyMessage, error)
	ListPolicies(ctx context.Context, find *FindPolicyMessage) ([]*PolicyMessage, error)
	GetRolloutPolicy(ctx context.Context, workspaceID string, environment string) (*storepb.RolloutPolicy, error)
	GetWorkspaceIamPolicy(ctx context.Context, workspaceID string) (*IamPolicyMessage, error)
	GetProjectIamPolicy(ctx context.Context, workspaceID string, projectID string) (*IamPolicyMessage, error)
}

// =============================================================================
// SETTING DOMAIN (setting.go)
// =============================================================================

// SettingReader provides read access to setting data.
type SettingReader interface {
	GetSetting(ctx context.Context, workspace string, name storepb.SettingName) (*SettingMessage, error)
	ListSettings(ctx context.Context, find *FindSettingMessage) ([]*SettingMessage, error)
	GetWorkspaceProfileSetting(ctx context.Context, workspaceID string) (*storepb.WorkspaceProfileSetting, error)
	GetAISetting(ctx context.Context, workspaceID string) (*storepb.AISetting, error)
	GetEnvironment(ctx context.Context, workspaceID string) (*storepb.EnvironmentSetting, error)
	GetEnvironmentByID(ctx context.Context, workspaceID string, id string) (*storepb.EnvironmentSetting_Environment, error)
}

// =============================================================================
// WORKSPACE DOMAIN (workspace.go)
// =============================================================================

// WorkspaceReader provides read access to workspace data.
type WorkspaceReader interface {
	GetWorkspaceID(ctx context.Context) (string, error)
	GetWorkspaceByID(ctx context.Context, resourceID string) (*WorkspaceMessage, error)
}

// =============================================================================
// AUDIT LOG DOMAIN (audit_log.go)
// =============================================================================

// AuditLogWriter provides write access to audit log data.
type AuditLogWriter interface {
	CreateAuditLog(ctx context.Context, workspace string, payload *storepb.AuditLog) error
}

// =============================================================================
// DB SCHEMA DOMAIN (db_schema.go)
// =============================================================================

// DBSchemaReader provides read access to database schema metadata.
type DBSchemaReader interface {
	GetDBSchema(ctx context.Context, find *FindDBSchemaMessage) (*model.DatabaseMetadata, error)
}

// =============================================================================
// SHEET DOMAIN (sheet.go)
// =============================================================================

// SheetReader provides read access to sheet data.
type SheetReader interface {
	GetSheetTruncated(ctx context.Context, sha256Hex string) (*SheetMessage, error)
	GetSheetFull(ctx context.Context, sha256Hex string) (*SheetMessage, error)
	CreateSheets(ctx context.Context, creates ...*SheetMessage) ([]*SheetMessage, error)
}

// =============================================================================
// ROLE DOMAIN (role.go)
// =============================================================================

// RoleReader provides read access to role data.
type RoleReader interface {
	GetRole(ctx context.Context, find *FindRoleMessage) (*RoleMessage, error)
	ListRoles(ctx context.Context, find *FindRoleMessage) ([]*RoleMessage, error)
}

// =============================================================================
// CHANGELOG DOMAIN (changelog.go)
// =============================================================================

// ChangelogReader provides read access to changelog data.
type ChangelogReader interface {
	GetChangelog(ctx context.Context, find *FindChangelogMessage) (*ChangelogMessage, error)
	ListChangelogs(ctx context.Context, find *FindChangelogMessage) ([]*ChangelogMessage, error)
}

// =============================================================================
// TASK DOMAIN (task.go)
// =============================================================================

// TaskStore provides access to task data.
type TaskStore interface {
	ListTasks(ctx context.Context, find *TaskFind) ([]*TaskMessage, error)
	CreateTasks(ctx context.Context, projectID string, planUID int64, tasks []*TaskMessage) ([]*TaskMessage, error)
	BatchSkipTasks(ctx context.Context, projectID string, taskUIDs []int64, comment string) error
}

// =============================================================================
// TASK RUN DOMAIN (task_run.go)
// =============================================================================

// TaskRunStore provides access to task run data.
type TaskRunStore interface {
	ListTaskRuns(ctx context.Context, find *FindTaskRunMessage) ([]*TaskRunMessage, error)
	GetTaskRunV1(ctx context.Context, find *FindTaskRunMessage) (*TaskRunMessage, error)
	ListTaskRunLogs(ctx context.Context, projectID string, taskRunUID int64) ([]*TaskRunLog, error)
	CreatePendingTaskRuns(ctx context.Context, creator string, creates ...*TaskRunMessage) error
	BatchCancelTaskRuns(ctx context.Context, projectID string, taskRunIDs []int64) error
}

// =============================================================================
// QUERY HISTORY DOMAIN (query_history.go)
// =============================================================================

// QueryHistoryStore provides access to query history data.
type QueryHistoryStore interface {
	CreateQueryHistory(ctx context.Context, create *QueryHistoryMessage) (*QueryHistoryMessage, error)
	ListQueryHistories(ctx context.Context, find *FindQueryHistoryMessage) ([]*QueryHistoryMessage, error)
}

// =============================================================================
// ACCESS GRANT DOMAIN (access_grant.go)
// =============================================================================

// AccessGrantReader provides read access to access grant data.
type AccessGrantReader interface {
	ListAccessGrants(ctx context.Context, find *FindAccessGrantMessage) ([]*AccessGrantMessage, error)
}

// =============================================================================
// EXPORT ARCHIVE DOMAIN (export_archive.go)
// =============================================================================

// ExportArchiveReader provides read access to export archive data.
type ExportArchiveReader interface {
	GetExportArchive(ctx context.Context, workspaceID string, resourceID string) (*ExportArchiveMessage, error)
}

// =============================================================================
// ACCOUNT DOMAIN (account.go)
// =============================================================================

// AccountReader provides read access to account data.
type AccountReader interface {
	GetAccountByEmail(ctx context.Context, email string) (*AccountMessage, error)
}

// =============================================================================
// SIGNAL DOMAIN (signal.go)
// =============================================================================

// SignalWriter provides write access to signal data.
type SignalWriter interface {
	SendSignal(ctx context.Context, signalType storepb.Signal_Type, projectID string, uid int64) error
}

// =============================================================================
// PLAN WEBHOOK DOMAIN (plan_webhook_delivery.go)
// =============================================================================

// PlanWebhookWriter provides webhook delivery operations.
type PlanWebhookWriter interface {
	ResetPlanWebhookDelivery(ctx context.Context, projectID string, planID int64) error
}

// =============================================================================
// SYNC HISTORY DOMAIN (sync_history.go)
// =============================================================================

// SyncHistoryReader provides read access to sync history data.
type SyncHistoryReader interface {
	GetSyncHistory(ctx context.Context, resourceID string) (*SyncHistory, error)
}

// =============================================================================
// AUTH DOMAIN (workspace, IDP, group, token, email verification)
// =============================================================================

// AuthStore provides data access methods required by the AuthService.
// It covers workspace management, identity providers, group sync,
// web refresh tokens, email verification, and service/workload identity lookups.
type AuthStore interface {
	// Workspace management
	FindWorkspace(ctx context.Context, find *FindWorkspaceMessage) (*WorkspaceMessage, error)
	CreateWorkspace(ctx context.Context, create *WorkspaceMessage, adminEmail string) (*WorkspaceMessage, error)
	PatchWorkspaceIamPolicy(ctx context.Context, patch *PatchIamPolicyMessage) (*IamPolicyMessage, error)

	// Identity providers
	GetIdentityProviderByID(ctx context.Context, resourceID string) (*IdentityProviderMessage, error)

	// Group management (for IDP group sync)
	ListGroups(ctx context.Context, find *FindGroupMessage) ([]*GroupMessage, error)
	UpdateGroup(ctx context.Context, patch *UpdateGroupMessage) (*GroupMessage, error)

	// Audit logging
	SearchAuditLogs(ctx context.Context, find *AuditLogFind) ([]*AuditLog, error)

	// Web refresh tokens
	CreateWebRefreshToken(ctx context.Context, create *WebRefreshTokenMessage) error
	GetAndDeleteWebRefreshToken(ctx context.Context, tokenHash string) (*WebRefreshTokenMessage, error)
	DeleteWebRefreshToken(ctx context.Context, tokenHash string) error
	DeleteWebRefreshTokensByUser(ctx context.Context, userEmail string) error

	// Email verification
	UpsertEmailVerificationCodeIfCooldownExpired(ctx context.Context, msg *EmailVerificationCodeMessage, cooldown time.Duration) (bool, error)
	GetEmailVerificationCode(ctx context.Context, email string, purpose storepb.EmailVerificationCodePurpose) (*EmailVerificationCodeMessage, error)
	IncrementEmailVerificationCodeAttempts(ctx context.Context, email string, purpose storepb.EmailVerificationCodePurpose) error
	DeleteEmailVerificationCodeIfMatch(ctx context.Context, email string, purpose storepb.EmailVerificationCodePurpose, codeHash string) error

	// Service/Workload identity lookups
	GetServiceAccountByEmail(ctx context.Context, email string) (*ServiceAccountMessage, error)
	GetWorkloadIdentityByEmail(ctx context.Context, email string) (*WorkloadIdentityMessage, error)
}

// =============================================================================
// AGGREGATE INTERFACE
// =============================================================================

// DataStore is the aggregate read/write interface for the Store layer.
// It combines all domain-specific interfaces for use in services that need
// broad access (e.g., during migration from concrete *Store).
type DataStore interface {
	UserStore
	ProjectReader
	ProjectWriter
	PlanReader
	IssueReader
	DatabaseReader
	DatabaseWriter
	InstanceReader
	PolicyReader
	SettingReader
	WorkspaceReader
	AuditLogWriter
	DBSchemaReader
	SheetReader
	RoleReader
	ChangelogReader
	TaskStore
	TaskRunStore
	QueryHistoryStore
	AccessGrantReader
	ExportArchiveReader
	AccountReader
	SignalWriter
	PlanWebhookWriter
	SyncHistoryReader
	AuthStore
	GetDB() *sql.DB
	Close() error
	DeleteCache()
}

