package v1

import (
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
)

// ResourceExtractorFunc extracts resource names from a proto request.
// Returns a list of resource name strings (e.g., "projects/foo", "instances/bar/databases/baz").
// The returned names are then resolved to typed resources by populateRawResources.
type ResourceExtractorFunc func(request proto.Message) ([]string, error)

// aclResourceExtractors maps RPC method short names to explicit resource extraction functions.
// SECURITY: Every registered RPC method MUST have an entry. Missing entries trigger
// a fallback to reflection-based extraction with a warning log.
//
// The map is keyed by short method name (e.g., "GetDatabase", "CreatePlan").
// For batch methods, the "Batch" prefix triggers sub-request iteration before lookup.
var aclResourceExtractors = map[string]ResourceExtractorFunc{
	// ============================================================
	// AuthService — public/workspace-level endpoints
	// ============================================================
	"Login":          extractNone,
	"Logout":         extractNone,
	"Refresh":        extractNone,
	"GetCurrentUser": extractNone,
	"GetUser":        extractFromName,
	"ListUsers":      extractNone,
	"CreateUser":     extractNone,
	"UpdateUser":     extractFromUserUpdate,
	"DeleteUser":     extractFromName,
	"UndeleteUser":   extractFromName,

	// ============================================================
	// ActuatorService — workspace-level
	// ============================================================
	"GetActuatorInfo":    extractNone,
	"UpdateActuatorInfo": extractNone,

	// ============================================================
	// SubscriptionService — workspace-level
	// ============================================================
	"GetSubscription":    extractNone,
	"UpdateSubscription": extractNone,

	// ============================================================
	// WorkspaceService (additional)
	// ============================================================
	"ListWorkspaces": extractNone,


	// ============================================================
	// ProjectService
	// ============================================================
	"GetProject":      extractFromName,
	"ListProjects":    extractNone,
	"SearchProjects":  extractNone,
	"CreateProject":   extractNone,
	"UpdateProject":   extractFromProjectUpdate,
	"DeleteProject":   extractFromName,
	"UndeleteProject": extractFromName,
	"GetIamPolicy":    extractFromResource,
	"SetIamPolicy":    extractFromResource,
	"AddWebhook":      extractFromProject,
	"UpdateWebhook":   extractFromProject,
	"RemoveWebhook":   extractFromProject,
	"TestWebhook":     extractFromProject,

	// ============================================================
	// DatabaseService
	// ============================================================
	"GetDatabase":          extractFromName,
	"ListDatabases":        extractFromParent,
	"UpdateDatabase":       extractFromDatabaseUpdate,
	"SyncDatabase":         extractFromName,
	"GetDatabaseMetadata":  extractFromName,
	"GetDatabaseSchema":    extractFromName,
	"GetDatabaseSDLSchema": extractFromName,
	"DiffSchema":           extractFromName,
	"GetSchemaString":      extractFromName,

	// ============================================================
	// InstanceService
	// ============================================================
	"GetInstance":      extractFromName,
	"ListInstances":    extractNone,
	"CreateInstance":   extractNone,
	"UpdateInstance":   extractFromInstanceUpdate,
	"DeleteInstance":   extractFromName,
	"UndeleteInstance": extractFromName,
	"SyncInstance":     extractFromName,
	"AddDataSource":   extractFromInstanceField,
	"UpdateDataSource": extractFromInstanceField,
	"RemoveDataSource": extractFromInstanceField,

	// ============================================================
	// PlanService
	// ============================================================
	"GetPlan":    extractFromName,
	"ListPlans":  extractFromParent,
	"CreatePlan": extractFromParent,
	"UpdatePlan": extractFromPlanUpdate,

	// ============================================================
	// IssueService
	// ============================================================
	"GetIssue":           extractFromName,
	"ListIssues":         extractFromParent,
	"SearchIssues":       extractFromParent,
	"CreateIssue":        extractFromParent,
	"UpdateIssue":        extractFromIssueUpdate,
	"ApproveIssue":       extractFromName,
	"RejectIssue":        extractFromName,
	"RequestIssue":       extractFromName,
	"ListIssueComments":  extractFromParent,
	"CreateIssueComment": extractFromParent,
	"UpdateIssueComment": extractFromParent,

	// ============================================================
	// RolloutService
	// ============================================================
	"GetRollout":        extractFromName,
	"CreateRollout":     extractFromParent,
	"ListTaskRuns":      extractFromParent,
	"GetTaskRunLog":     extractFromParent,
	"RunTasks":          extractFromParent,
	"SkipTasks":         extractFromParent,
	"RestartTasks":      extractFromParent,
	"CancelTaskRuns":    extractFromParent,

	// ============================================================
	// SettingService — workspace-level
	// ============================================================
	"GetSetting":    extractFromName,
	"ListSettings":  extractNone,
	"UpdateSetting": extractFromSettingUpdate,

	// ============================================================
	// EnvironmentService
	// ============================================================
	"GetEnvironment":      extractFromName,
	"ListEnvironments":    extractNone,
	"CreateEnvironment":   extractNone,
	"UpdateEnvironment":   extractFromName,
	"DeleteEnvironment":   extractFromName,
	"UndeleteEnvironment": extractFromName,

	// ============================================================
	// RoleService — workspace-level
	// ============================================================
	"GetRole":      extractFromName,
	"ListRoles":    extractNone,
	"CreateRole":   extractNone,
	"UpdateRole":   extractFromRoleUpdate,
	"DeleteRole":   extractFromName,

	// ============================================================
	// SheetService
	// ============================================================
	"GetSheet":    extractFromName,
	"CreateSheet": extractFromParent,
	"UpdateSheet": extractFromName,

	// ============================================================
	// ReleaseService
	// ============================================================
	"GetRelease":    extractFromName,
	"ListReleases":  extractFromParent,
	"CreateRelease": extractFromParent,
	"UpdateRelease": extractFromReleaseUpdate,

	// ============================================================
	// ChangelistService
	// ============================================================
	"GetChangelist":    extractFromName,
	"ListChangelists":  extractFromParent,
	"CreateChangelist": extractFromParent,
	"UpdateChangelist": extractFromName,
	"DeleteChangelist": extractFromName,

	// ============================================================
	// BranchService
	// ============================================================
	"GetBranch":      extractFromName,
	"ListBranches":   extractFromParent,
	"CreateBranch":   extractFromParent,
	"UpdateBranch":   extractFromName,
	"DeleteBranch":   extractFromName,
	"MergeBranch":    extractFromName,
	"RebaseBranch":   extractFromName,
	"DiffDatabase":   extractFromName,

	// ============================================================
	// SQLService — workspace-level
	// ============================================================
	"Query":           extractFromName,
	"Export":          extractFromName,
	"AdminExecute":   extractFromName,
	"Check":          extractNone,

	// ============================================================
	// AuditLogService — workspace-level
	// ============================================================
	"SearchAuditLogs":  extractNone,
	"ExportAuditLogs":  extractNone,

	// ============================================================
	// RiskService — workspace-level
	// ============================================================
	"ListRisks":    extractNone,
	"CreateRisk":   extractNone,
	"UpdateRisk":   extractFromName,
	"DeleteRisk":   extractFromName,

	// ============================================================
	// ReviewConfigService — workspace-level
	// ============================================================
	"GetReviewConfig":    extractFromName,
	"ListReviewConfigs":  extractNone,
	"CreateReviewConfig": extractNone,
	"UpdateReviewConfig": extractFromReviewConfigUpdate,
	"DeleteReviewConfig": extractFromName,

	// ============================================================
	// GroupService — workspace-level
	// ============================================================
	"GetGroup":    extractFromName,
	"ListGroups":  extractNone,
	"CreateGroup": extractNone,
	"UpdateGroup": extractFromGroupUpdate,
	"DeleteGroup": extractFromName,

	// ============================================================
	// WorkspaceService
	// ============================================================
	"GetWorkspace": extractFromName,

	// ============================================================
	// VCSConnectorService
	// ============================================================
	"GetVCSConnector":    extractFromName,
	"ListVCSConnectors":  extractFromParent,
	"CreateVCSConnector": extractFromParent,
	"UpdateVCSConnector": extractFromName,
	"DeleteVCSConnector": extractFromName,

	// ============================================================
	// VCSProviderService — workspace-level
	// ============================================================
	"GetVCSProvider":    extractFromName,
	"ListVCSProviders":  extractNone,
	"CreateVCSProvider": extractNone,
	"UpdateVCSProvider": extractFromName,
	"DeleteVCSProvider": extractFromName,

	// ============================================================
	// IdentityProviderService — workspace-level
	// ============================================================
	"GetIdentityProvider":      extractFromName,
	"ListIdentityProviders":    extractNone,
	"CreateIdentityProvider":   extractNone,
	"UpdateIdentityProvider":   extractFromIdentityProviderUpdate,
	"DeleteIdentityProvider":   extractFromName,
	"UndeleteIdentityProvider": extractFromName,
	"TestIdentityProvider":     extractNone,
}

// ============================================================
// Shared helper extractors
// ============================================================

// extractNone returns nil — used for public/workspace-level endpoints.
func extractNone(_ proto.Message) ([]string, error) {
	return nil, nil
}

// extractFromName extracts the "name" field via proto reflection.
func extractFromName(msg proto.Message) ([]string, error) {
	return extractField(msg, "name")
}

// extractFromParent extracts the "parent" field via proto reflection.
func extractFromParent(msg proto.Message) ([]string, error) {
	return extractField(msg, "parent")
}

// extractFromResource extracts the "resource" field (used by Get/SetIAMPolicy).
func extractFromResource(msg proto.Message) ([]string, error) {
	return extractField(msg, "resource")
}

// extractFromProject extracts the "project" field (used by webhook methods).
func extractFromProject(msg proto.Message) ([]string, error) {
	return extractField(msg, "project")
}

// extractFromInstanceField extracts the "instance" field from DataSource requests.
func extractFromInstanceField(msg proto.Message) ([]string, error) {
	return extractField(msg, "instance")
}

// extractField is a controlled single-field reflection helper.
func extractField(msg proto.Message, fieldName string) ([]string, error) {
	mr := msg.ProtoReflect()
	fd := mr.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if fd == nil {
		return nil, nil
	}
	val := mr.Get(fd).String()
	if val == "" {
		return nil, nil
	}
	return []string{val}, nil
}

// ============================================================
// Custom extractors for complex update methods
// ============================================================

// extractFromDatabaseUpdate handles UpdateDatabase — checks both name and project transfer.
func extractFromDatabaseUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateDatabaseRequest)
	if !ok {
		return nil, errors.New("expected UpdateDatabaseRequest")
	}
	var resources []string
	if name := req.GetDatabase().GetName(); name != "" {
		resources = append(resources, name)
	}
	// If transferring project, also check the target project
	if hasFieldMaskPath(req.GetUpdateMask(), "project") {
		if project := req.GetDatabase().GetProject(); project != "" {
			resources = append(resources, project)
		}
	}
	return resources, nil
}

// extractFromBatchIssuesStatus handles BatchUpdateIssuesStatus (non-AIP-compliant).
func extractFromBatchIssuesStatus(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.BatchUpdateIssuesStatusRequest)
	if !ok {
		return nil, errors.New("expected BatchUpdateIssuesStatusRequest")
	}
	return req.Issues, nil
}

// ============================================================
// Generic update extractors — extract "name" from the inner resource message
// ============================================================

func extractFromUserUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateUserRequest)
	if !ok {
		return nil, errors.New("expected UpdateUserRequest")
	}
	if name := req.GetUser().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromProjectUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateProjectRequest)
	if !ok {
		return nil, errors.New("expected UpdateProjectRequest")
	}
	if name := req.GetProject().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromInstanceUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateInstanceRequest)
	if !ok {
		return nil, errors.New("expected UpdateInstanceRequest")
	}
	if name := req.GetInstance().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromPlanUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdatePlanRequest)
	if !ok {
		return nil, errors.New("expected UpdatePlanRequest")
	}
	if name := req.GetPlan().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromIssueUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateIssueRequest)
	if !ok {
		return nil, errors.New("expected UpdateIssueRequest")
	}
	if name := req.GetIssue().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromSettingUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateSettingRequest)
	if !ok {
		return nil, errors.New("expected UpdateSettingRequest")
	}
	if name := req.GetSetting().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromRoleUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateRoleRequest)
	if !ok {
		return nil, errors.New("expected UpdateRoleRequest")
	}
	if name := req.GetRole().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromReleaseUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateReleaseRequest)
	if !ok {
		return nil, errors.New("expected UpdateReleaseRequest")
	}
	if name := req.GetRelease().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromReviewConfigUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateReviewConfigRequest)
	if !ok {
		return nil, errors.New("expected UpdateReviewConfigRequest")
	}
	if name := req.GetReviewConfig().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromGroupUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateGroupRequest)
	if !ok {
		return nil, errors.New("expected UpdateGroupRequest")
	}
	if name := req.GetGroup().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

func extractFromIdentityProviderUpdate(msg proto.Message) ([]string, error) {
	req, ok := msg.(*v1pb.UpdateIdentityProviderRequest)
	if !ok {
		return nil, errors.New("expected UpdateIdentityProviderRequest")
	}
	if name := req.GetIdentityProvider().GetName(); name != "" {
		return []string{name}, nil
	}
	return nil, nil
}

// hasFieldMaskPath checks if a field mask contains the specified path.
func hasFieldMaskPath(mask *fieldmaskpb.FieldMask, path string) bool {
	if mask == nil {
		return false
	}
	for _, p := range mask.Paths {
		if p == path {
			return true
		}
	}
	return false
}

// lookupExtractor returns the static extractor for a short method name.
// Falls back to reflection-based extraction if no entry exists.
// This function is the bridge between the old and new systems during migration.
func lookupExtractor(shortMethod string) (ResourceExtractorFunc, bool) {
	// Check for BatchUpdateIssuesStatus special case first
	if shortMethod == "BatchUpdateIssuesStatus" {
		return extractFromBatchIssuesStatus, true
	}

	// Handle batch methods by stripping prefix/suffix
	if strings.HasPrefix(shortMethod, "Batch") {
		inner := strings.TrimSuffix(strings.TrimPrefix(shortMethod, "Batch"), "s")
		if ext, ok := aclResourceExtractors[inner]; ok {
			return ext, true
		}
	}

	ext, ok := aclResourceExtractors[shortMethod]
	return ext, ok
}

// init registers additional extractors using google.api annotations.
func init() {
	// BatchUpdateIssuesStatus is explicitly handled in lookupExtractor.
	// Batch methods delegate to their singular form after stripping Batch prefix.
}
