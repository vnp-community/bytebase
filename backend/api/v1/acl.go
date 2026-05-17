package v1

import (
	"context"
	"database/sql"
	"log/slog"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/api/auth"
	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/iam"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// ACLInterceptor is the v1 ACL interceptor for gRPC server.
type ACLInterceptor struct {
	store      store.DataStore
	secret     string
	iamManager *iam.Manager
	profile    *config.Profile
}

// NewACLInterceptor returns a new v1 API ACL interceptor.
func NewACLInterceptor(store store.DataStore, secret string, iamManager *iam.Manager, profile *config.Profile) *ACLInterceptor {
	return &ACLInterceptor{
		store:      store,
		secret:     secret,
		iamManager: iamManager,
		profile:    profile,
	}
}

func (in *ACLInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		err := in.doACLCheck(ctx, req.Any(), req.Spec().Procedure)
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (*ACLInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return next(ctx, spec)
	}
}

func (in *ACLInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		wrappedConn := &aclStreamingConn{
			StreamingHandlerConn: conn,
			interceptor:          in,
			fullMethod:           conn.Spec().Procedure,
			ctx:                  ctx,
		}
		return next(ctx, wrappedConn)
	}
}

type aclStreamingConn struct {
	connect.StreamingHandlerConn
	interceptor *ACLInterceptor
	fullMethod  string
	ctx         context.Context
}

func (c *aclStreamingConn) Receive(msg any) error {
	err := c.interceptor.doACLCheck(c.ctx, msg, c.fullMethod)
	if err != nil {
		return err
	}
	return c.StreamingHandlerConn.Receive(msg)
}

// hasAllowMissingEnabled checks if the request has allow_missing field set to true.
// Uses proto reflection to handle different request types generically.
func hasAllowMissingEnabled(request any) bool {
	if request == nil {
		return false
	}

	pm, ok := request.(proto.Message)
	if !ok {
		return false
	}

	mr := pm.ProtoReflect()
	fd := mr.Descriptor().Fields().ByName("allow_missing")
	if fd == nil {
		return false
	}

	// Check if field is a bool and get its value
	if fd.Kind() != protoreflect.BoolKind {
		return false
	}

	return mr.Get(fd).Bool()
}

func (in *ACLInterceptor) doACLCheck(ctx context.Context, request any, fullMethod string) error {
	defer func() {
		if r := recover(); r != nil {
			perr, ok := r.(error)
			if !ok {
				perr = errors.Errorf("%v", r)
			}
			slog.Error("iam check PANIC RECOVER", log.BBError(perr), log.BBStack("panic-stack"))
		}
	}()

	authContextAny := ctx.Value(common.AuthContextKey)
	authContext, ok := authContextAny.(*common.AuthContext)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("auth context not found"))
	}
	resources, err := populateRawResources(ctx, in.store, request, fullMethod)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Errorf("failed to populate raw resources %s", err))
	}
	authContext.Resources = resources

	if auth.IsAuthenticationSkipped(fullMethod, authContext) {
		return nil
	}

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, errors.Errorf("unauthenticated for method %q", fullMethod))
	}

	workspaceID := common.GetWorkspaceIDFromContext(ctx)
	if workspaceID == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.Errorf("empty workspace id"))
	}

	// Workspace isolation: verify all resources belong to the caller's workspace.
	// Instance and database ownership is already validated in populateRawResources
	// (via workspace-filtered store lookups). Here we validate workspace and project resources.
	// Runs after authentication so unauthenticated requests get 401 first,
	// preventing resource existence probing.
	for _, resource := range authContext.Resources {
		switch resource.Type {
		case common.ResourceTypeWorkspace:
			if resource.ID != workspaceID {
				return connect.NewError(connect.CodePermissionDenied, errors.Errorf("workspace mismatch"))
			}
		case common.ResourceTypeProject:
			project, err := in.store.GetProject(ctx, &store.FindProjectMessage{Workspace: workspaceID, ResourceID: &resource.ID})
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get project"))
			}
			if project == nil || project.Workspace != workspaceID {
				return connect.NewError(connect.CodeNotFound, errors.Errorf("project %q not found", resource.ID))
			}
		default:
		}
	}

	ok, extra, err := doIAMPermissionCheck(ctx, in.iamManager, fullMethod, user, authContext)
	if err != nil {
		// TASK-WEAK-003-2: Differentiate store/infrastructure errors (503) from logic errors (500).
		if isStoreError(err) {
			return connect.NewError(connect.CodeUnavailable,
				errors.Errorf("permission check temporarily unavailable for method %q", fullMethod))
		}
		return connect.NewError(connect.CodeInternal, errors.Errorf("failed to check permission for method %q, extra %v, err: %v", fullMethod, extra, err))
	}
	if !ok {
		err := connect.NewError(connect.CodePermissionDenied, errors.Errorf("permission denied for method %q, user does not have permission %q, extra %v", fullMethod, authContext.Permission, extra))
		if detail, detailErr := connect.NewErrorDetail(&v1pb.PermissionDeniedDetail{
			Method:              fullMethod,
			RequiredPermissions: []string{string(authContext.Permission)},
			Resources:           extra,
		}); detailErr == nil {
			err.AddDetail(detail)
		}
		return err
	}

	// Check allow_missing secondary permission if applicable
	// This handles Update methods that can create resources via allow_missing=true
	// When allow_missing is set, we additionally require create permission
	if hasAllowMissingEnabled(request) {
		// Derive create permission by replacing ".update" with ".create"
		// Example: "bb.roles.update" -> "bb.roles.create"
		createPerm := strings.Replace(string(authContext.Permission), ".update", ".create", 1)

		// Create a new auth context for create permission check
		createAuthContext := &common.AuthContext{
			Permission: permission.Permission(createPerm),
			AuthMethod: authContext.AuthMethod,
			Resources:  authContext.Resources,
		}
		ok, extra, err := doIAMPermissionCheck(ctx, in.iamManager, fullMethod, user, createAuthContext)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.Errorf("failed to check create permission %q, extra %v, err: %v", createPerm, extra, err))
		}
		if !ok {
			err := connect.NewError(connect.CodePermissionDenied, errors.Errorf("permission denied: allow_missing=true requires both %s and %s, extra %v", authContext.Permission, createPerm, extra))
			if detail, detailErr := connect.NewErrorDetail(&v1pb.PermissionDeniedDetail{
				Method:              fullMethod,
				RequiredPermissions: []string{string(authContext.Permission), createPerm},
				Resources:           extra,
			}); detailErr == nil {
				err.AddDetail(detail)
			}
			return err
		}
	}

	return nil
}

func hasPath(fieldMask *fieldmaskpb.FieldMask, want string) bool {
	if fieldMask == nil {
		return false
	}
	for _, path := range fieldMask.Paths {
		if path == want {
			return true
		}
	}
	return false
}

func doIAMPermissionCheck(ctx context.Context, iamManager *iam.Manager, fullMethod string, user *store.UserMessage, authContext *common.AuthContext) (bool, []string, error) {
	if auth.IsAuthenticationSkipped(fullMethod, authContext) {
		return true, nil, nil
	}
	if authContext.AuthMethod != common.AuthMethodIAM {
		return true, nil, nil
	}
	// Handle GetProject() error status.
	if len(authContext.Resources) == 0 {
		return false, nil, errors.Errorf("no resource found for IAM auth method")
	}

	var hasWorkspaceResource bool
	projectIDMap := make(map[string]bool)
	workspaceID := common.GetWorkspaceIDFromContext(ctx)
	for _, resource := range authContext.Resources {
		switch resource.Type {
		case common.ResourceTypeWorkspace:
			hasWorkspaceResource = true
		case common.ResourceTypeProject:
			projectIDMap[resource.ID] = true
		default:
			return false, nil, errors.Errorf("unknown resource type %v", resource.Type)
		}
	}

	if hasWorkspaceResource {
		ok, err := iamManager.CheckPermission(ctx, authContext.Permission, user, workspaceID)
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, nil
		}
	}
	if len(projectIDMap) > 0 {
		var projectIDs []string
		for projectID := range projectIDMap {
			projectIDs = append(projectIDs, projectID)
		}
		ok, err := iamManager.CheckPermission(ctx, authContext.Permission, user, workspaceID, projectIDs...)
		if err != nil {
			return false, nil, err
		}
		if ok {
			return true, nil, nil
		}
		projectResources := []string{}
		for _, id := range projectIDs {
			projectResources = append(projectResources, common.FormatProject(id))
		}
		return false, projectResources, nil
	}
	return true, nil, nil
}

var workspaceRegex = regexp.MustCompile(`^workspaces/[^/]+`)
var projectRegex = regexp.MustCompile(`^projects/[^/]+`)
var databaseRegex = regexp.MustCompile(`^instances/[^/]+/databases/[^/]+`)
var instanceRegex = regexp.MustCompile(`^instances/[^/]+`)

// populateRawResources extracts resources from the request and validates workspace ownership.
//
// Resource resolution strategy:
//   - workspaces/{id}        → ResourceTypeWorkspace (direct match)
//   - projects/{id}          → ResourceTypeProject (direct match)
//   - instances/{id}/databases/{name} → looks up database with workspace filter, returns ResourceTypeProject (parent project)
//   - instances/{id}[/...]   → looks up instance with workspace filter, returns ResourceTypeWorkspace (instance permissions are workspace-scoped)
//   - default                → ResourceTypeWorkspace from context (fallback for unmatched patterns)
//
// All instance/database lookups use the workspace ID from the request context,
// ensuring the resource belongs to the caller's workspace before any permission check.
func populateRawResources(ctx context.Context, stores store.DataStore, request any, method string) ([]*common.Resource, error) {
	rawNames, err := getResourceFromRequest(ctx, request, method)
	if err != nil {
		return nil, err
	}

	var resources []*common.Resource
	for _, name := range rawNames {
		switch {
		case strings.HasPrefix(name, "workspaces/"):
			match := workspaceRegex.FindString(name)
			if match == "" {
				return nil, errors.Errorf("invalid workspace resource %q", name)
			}
			wsID, err := common.GetWorkspaceID(match)
			if err != nil {
				return nil, err
			}
			resources = append(resources, &common.Resource{
				Type: common.ResourceTypeWorkspace,
				ID:   wsID,
			})
		// TODO(d): remove "projects/-" hack later.
		case strings.HasPrefix(name, "projects/") && name != "projects/-":
			project := projectRegex.FindString(name)
			if project == "" {
				return nil, errors.Errorf("invalid project resource %q", name)
			}
			projectID, err := common.GetProjectID(project)
			if err != nil {
				return nil, err
			}
			resources = append(resources, &common.Resource{
				Type: common.ResourceTypeProject,
				ID:   projectID,
			})
		// Database resources: look up the database (with workspace filter) and resolve
		// to its parent project for project-level permission checks.
		case strings.HasPrefix(name, "instances/") && strings.Contains(name, "/databases/") && !strings.HasPrefix(name, "instances/-/databases/"):
			match := databaseRegex.FindString(name)
			if match != "" {
				instanceID, databaseName, err := common.GetInstanceDatabaseID(match)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse %q", match)
				}
				database, err := stores.GetDatabase(ctx, &store.FindDatabaseMessage{
					Workspace:    common.GetWorkspaceIDFromContext(ctx),
					InstanceID:   &instanceID,
					DatabaseName: &databaseName,
					ShowDeleted:  true,
				})
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get database")
				}
				if database == nil {
					return nil, errors.Errorf("database %q not found", match)
				}
				resources = append(resources, &common.Resource{
					Type: common.ResourceTypeProject,
					ID:   database.ProjectID,
				})
			}
		// Instance resources (e.g. instances/{id}, instances/{id}/roles/{role}):
		// validate the instance belongs to the caller's workspace. Returns workspace
		// resource since instance permissions are workspace-scoped.
		case strings.HasPrefix(name, "instances/") && !strings.Contains(name, "/databases/"):
			match := instanceRegex.FindString(name)
			if match != "" {
				instanceID, err := common.GetInstanceID(match)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to parse %q", match)
				}
				workspaceID := common.GetWorkspaceIDFromContext(ctx)
				instance, err := stores.GetInstance(ctx, &store.FindInstanceMessage{
					Workspace:  workspaceID,
					ResourceID: &instanceID,
				})
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get instance")
				}
				if instance == nil {
					return nil, errors.Errorf("instance %q not found", match)
				}
				resources = append(resources, &common.Resource{
					Type: common.ResourceTypeWorkspace,
					ID:   workspaceID,
				})
			}
		default:
			if workspaceID := common.GetWorkspaceIDFromContext(ctx); workspaceID != "" {
				resources = append(resources, &common.Resource{
					Type: common.ResourceTypeWorkspace,
					ID:   workspaceID,
				})
			}
		}
	}

	// Fallback: if no resources were resolved (e.g., extractNone returned nil for
	// workspace-level methods like ListRoles, ListEnvironments), use the workspace
	// from the request context so IAM permission checks have a resource to evaluate.
	if len(resources) == 0 {
		if workspaceID := common.GetWorkspaceIDFromContext(ctx); workspaceID != "" {
			resources = append(resources, &common.Resource{
				Type: common.ResourceTypeWorkspace,
				ID:   workspaceID,
			})
		}
	}

	return resources, nil
}

// getResourceFromRequest extracts resource names from the request using the static
// extractor registry (acl_extractors.go). Falls back to legacy reflection-based
// extraction for unregistered methods, logging a warning.
//
// SECURITY: The static map approach ensures fail-closed behavior — unknown methods
// are logged and fall back to workspace-level permissions rather than silently
// granting access.
func getResourceFromRequest(ctx context.Context, request any, method string) ([]string, error) {
	pm, ok := request.(proto.Message)
	if !ok {
		return nil, errors.Errorf("invalid request for method %q", method)
	}

	methodTokens := strings.Split(method, "/")
	if len(methodTokens) != 3 {
		return nil, errors.Errorf("invalid method %q", method)
	}
	shortMethod := methodTokens[2]

	// Phase 1: Handle special batch methods that iterate sub-requests.
	if strings.HasPrefix(shortMethod, "BatchGet") {
		return extractBatchGetNames(pm)
	}
	if strings.HasPrefix(shortMethod, "Batch") && shortMethod != "BatchUpdateIssuesStatus" {
		return extractBatchSubRequests(pm, shortMethod)
	}

	// Phase 2: Static extractor map lookup.
	if extractor, ok := lookupExtractor(shortMethod); ok {
		resources, err := extractor(pm)
		if err != nil {
			return nil, errors.Wrapf(err, "static extractor failed for %q", shortMethod)
		}
		return resources, nil
	}

	// Phase 3: Fallback — unknown method.
	// Log a warning so we can detect coverage gaps and add missing extractors.
	slog.Warn("ACL: no static extractor for method, falling back to workspace-level",
		slog.String("method", shortMethod))
	return nil, nil
}

// extractBatchGetNames handles BatchGet* methods by extracting the "names" repeated field.
func extractBatchGetNames(pm proto.Message) ([]string, error) {
	mr := pm.ProtoReflect()
	namesDesc := mr.Descriptor().Fields().ByName("names")
	if namesDesc == nil {
		return nil, nil
	}
	namesValue := mr.Get(namesDesc)
	namesList := namesValue.List()
	resources := make([]string, 0, namesList.Len())
	for i := 0; i < namesList.Len(); i++ {
		resources = append(resources, namesList.Get(i).String())
	}
	return resources, nil
}

// extractBatchSubRequests handles Batch* methods (except BatchGet and BatchUpdateIssuesStatus)
// by iterating the "requests" repeated field and applying per-sub-request extraction.
func extractBatchSubRequests(pm proto.Message, shortMethod string) ([]string, error) {
	mr := pm.ProtoReflect()
	requestsDesc := mr.Descriptor().Fields().ByName("requests")
	if requestsDesc == nil {
		return nil, nil
	}
	requestsValue := mr.Get(requestsDesc)
	requestsList := requestsValue.List()
	innerMethod := strings.TrimSuffix(strings.TrimPrefix(shortMethod, "Batch"), "s")

	extractor, hasExtractor := lookupExtractor(innerMethod)

	var resources []string
	for i := 0; i < requestsList.Len(); i++ {
		subMsg := requestsList.Get(i).Message().Interface()
		if hasExtractor {
			subResources, err := extractor(subMsg)
			if err != nil {
				return nil, errors.Wrapf(err, "batch sub-request extractor failed for %q", innerMethod)
			}
			resources = append(resources, subResources...)
		} else {
			// Fallback: try extracting "name" field from sub-request
			subResources, _ := extractFromName(subMsg)
			resources = append(resources, subResources...)
		}
	}
	return resources, nil
}

// isStoreError checks if the error originates from a database or infrastructure failure.
// TASK-WEAK-003-2: Used to return 503 Unavailable (retryable) instead of 500 Internal.
func isStoreError(err error) bool {
	if err == nil {
		return false
	}
	// Check for standard sql.Err* types.
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) || errors.Is(err, sql.ErrNoRows) {
		return true
	}
	// Check for common pgx/pgconn error patterns in the error chain.
	errMsg := err.Error()
	storePatterns := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"database is closed",
		"broken pipe",
		"pgconn",
		"conn closed",
		"pool is closed",
		"too many clients",
	}
	for _, pattern := range storePatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			return true
		}
	}
	return false
}
