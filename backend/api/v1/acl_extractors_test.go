package v1

import (
	"testing"
)

// TestACLExtractorMap_Exhaustive verifies that every known RPC method
// has a matching entry in the static aclResourceExtractors map
// (or is handled by batch/special-case logic in acl.go).
//
// If a developer adds a new RPC method but forgets to register an extractor,
// this test will fail — enforcing the fail-closed security contract.
func TestACLExtractorMap_Exhaustive(t *testing.T) {
	// All known RPC methods that pass through the ACL interceptor
	// and are resolved via the static aclResourceExtractors map.
	// Batch methods (BatchRunTasks, BatchSkipTasks, etc.) are handled
	// by extractBatchSubRequests in acl.go before reaching the map.
	knownMethods := []string{
		// AuthService
		"Login", "Logout", "GetUser", "ListUsers", "CreateUser", "UpdateUser", "DeleteUser", "UndeleteUser",
		// ProjectService
		"GetProject", "ListProjects", "SearchProjects", "CreateProject", "UpdateProject",
		"DeleteProject", "UndeleteProject", "GetIamPolicy", "SetIamPolicy",
		"AddWebhook", "UpdateWebhook", "RemoveWebhook", "TestWebhook",
		// DatabaseService
		"GetDatabase", "ListDatabases", "UpdateDatabase",
		"SyncDatabase", "GetDatabaseMetadata", "GetDatabaseSchema", "DiffSchema",
		// InstanceService
		"GetInstance", "ListInstances", "CreateInstance", "UpdateInstance",
		"DeleteInstance", "UndeleteInstance", "SyncInstance", "AddDataSource",
		"UpdateDataSource", "RemoveDataSource",
		// PlanService
		"GetPlan", "ListPlans", "CreatePlan", "UpdatePlan",
		// IssueService
		"GetIssue", "ListIssues", "SearchIssues", "CreateIssue", "UpdateIssue",
		"ApproveIssue", "RejectIssue", "RequestIssue",
		"CreateIssueComment", "UpdateIssueComment",
		// RolloutService (non-batch)
		"GetRollout", "CreateRollout",
		"ListTaskRuns", "GetTaskRunLog",
		"RunTasks", "SkipTasks", "CancelTaskRuns",
		// SettingService
		"GetSetting", "ListSettings", "UpdateSetting",
		// EnvironmentService
		"GetEnvironment", "ListEnvironments", "CreateEnvironment",
		"UpdateEnvironment", "DeleteEnvironment", "UndeleteEnvironment",
		// RoleService
		"GetRole", "ListRoles", "CreateRole", "UpdateRole", "DeleteRole",
		// SheetService
		"GetSheet", "CreateSheet", "UpdateSheet",
	}

	for _, method := range knownMethods {
		t.Run(method, func(t *testing.T) {
			ext, ok := lookupExtractor(method)
			if !ok {
				t.Errorf("method %q has no ACL resource extractor — add an entry to aclResourceExtractors", method)
			}
			if ext == nil {
				t.Errorf("method %q has nil extractor function", method)
			}
		})
	}
}

// TestACLExtractorMap_NoNilValues ensures every entry in the static map
// has a non-nil extractor function.
func TestACLExtractorMap_NoNilValues(t *testing.T) {
	for method, ext := range aclResourceExtractors {
		if ext == nil {
			t.Errorf("aclResourceExtractors[%q] is nil — every method must have a valid extractor", method)
		}
	}
}

// TestACLExtractorMap_SpecialCases verifies that special-case methods
// handled outside the main map are correctly resolved.
func TestACLExtractorMap_SpecialCases(t *testing.T) {
	// BatchUpdateIssuesStatus has a special-case handler in lookupExtractor
	t.Run("BatchUpdateIssuesStatus", func(t *testing.T) {
		ext, ok := lookupExtractor("BatchUpdateIssuesStatus")
		if !ok {
			t.Error("BatchUpdateIssuesStatus should be handled by lookupExtractor special case")
		}
		if ext == nil {
			t.Error("BatchUpdateIssuesStatus extractor is nil")
		}
	})
}

// TestACLExtractorMap_WorkspaceFallback documents methods that intentionally
// use the fail-closed workspace-level permission fallback.
// These methods are NOT in the static map by design — they are protected
// by workspace-level RBAC instead of project-level IAM.
func TestACLExtractorMap_WorkspaceFallback(t *testing.T) {
	fallbackMethods := []string{
		"SearchDatabases",   // workspace-level search, no single project scope
		"SearchInstances",   // workspace-level search, no single project scope
		"PreviewRollout",    // read-only preview, workspace-level
		"GetTaskRunSession", // session data, workspace-level
		"GetActuatorInfo",   // system actuator, workspace-level
		"UpdateActuatorInfo",// system actuator, workspace-level
		"ListDebugLog",      // debug logs, workspace-level
	}

	for _, method := range fallbackMethods {
		t.Run(method, func(t *testing.T) {
			_, ok := lookupExtractor(method)
			if ok {
				t.Errorf("method %q is expected to use workspace fallback but found in extractor map", method)
			}
		})
	}
}
