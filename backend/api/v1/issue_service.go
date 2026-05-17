package v1

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	celoperators "github.com/google/cel-go/common/operators"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/component/bus"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/component/webhook"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
)

// IssueService implements the issue service.
type IssueService struct {
	v1connect.UnimplementedIssueServiceHandler
	store          *store.Store
	webhookManager *webhook.Manager
	bus            bus.EventBus
	licenseService *enterprise.LicenseService
	iamManager     *iam.Manager
}

type filterIssueMessage struct {
	ApprovalStatus *v1pb.Issue_ApprovalStatus
	// Approver is the user who can approve the issue.
	Approver *store.UserMessage
}

// NewIssueService creates a new IssueService.
func NewIssueService(
	store *store.Store,
	webhookManager *webhook.Manager,
	bus bus.EventBus,
	licenseService *enterprise.LicenseService,
	iamManager *iam.Manager,
) *IssueService {
	return &IssueService{
		store:          store,
		webhookManager: webhookManager,
		bus:            bus,
		licenseService: licenseService,
		iamManager:     iamManager,
	}
}

// GetIssue gets a issue.
func (s *IssueService) GetIssue(ctx context.Context, req *connect.Request[v1pb.GetIssueRequest]) (*connect.Response[v1pb.Issue], error) {
	issue, err := s.getIssueMessage(ctx, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	issueV1, err := s.convertToIssue(issue)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to issue"))
	}
	return connect.NewResponse(issueV1), nil
}

func (s *IssueService) getIssueFind(
	ctx context.Context,
	filter string,
	query string,
	limit,
	offset *int,
) (*store.FindIssueMessage, *filterIssueMessage, error) {
	issueFind := &store.FindIssueMessage{
		Workspace: common.GetWorkspaceIDFromContext(ctx),
		Limit:     limit,
		Offset:    offset,
	}
	if query != "" {
		issueFind.Query = &query
	}
	if filter == "" {
		return issueFind, nil, nil
	}

	e, err := cel.NewEnv()
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to create cel env"))
	}
	ast, iss := e.Parse(filter)
	if iss != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to parse filter %v, error: %v", filter, iss.String()))
	}

	filterIssue := &filterIssueMessage{}

	var parseFilter func(expr celast.Expr) (string, error)
	parseFilter = func(expr celast.Expr) (string, error) {
		switch expr.Kind() {
		case celast.CallKind:
			functionName := expr.AsCall().FunctionName()
			switch functionName {
			case celoperators.LogicalAnd:
				return getSubConditionFromExpr(expr, parseFilter, "AND")
			case celoperators.Equals:
				variable, value := getVariableAndValueFromExpr(expr)
				switch variable {
				case "status":
					issueStatus, err := convertToAPIIssueStatus(v1pb.IssueStatus(v1pb.IssueStatus_value[value.(string)]))
					if err != nil {
						return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to convert to issue status, err: %v", err))
					}
					issueFind.StatusList = append(issueFind.StatusList, issueStatus)
				case "type":
					issueType, err := convertToAPIIssueType(v1pb.Issue_Type(v1pb.Issue_Type_value[value.(string)]))
					if err != nil {
						return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to convert to issue type, err: %v", err))
					}
					issueFind.Types = &[]storepb.Issue_Type{issueType}
				case "labels":
					issueFind.LabelList = append(issueFind.LabelList, value.(string))
				case "approval_status":
					approvalStatusValue, ok := v1pb.Issue_ApprovalStatus_value[value.(string)]
					if !ok {
						return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf(`invalid approval_status %q`, value))
					}
					filterIssue.ApprovalStatus = new(v1pb.Issue_ApprovalStatus(approvalStatusValue))
				case "current_approver", "creator":
					user, err := s.getUserByIdentifier(ctx, value.(string))
					if err != nil {
						return "", connect.NewError(connect.CodeInternal, errors.Errorf("failed to get user %v with error %v", value, err.Error()))
					}
					if variable == "current_approver" {
						filterIssue.Approver = user
					} else {
						issueFind.CreatorID = &user.Email
					}
				default:
					return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unsupport variable %q with %v operator", variable, celoperators.Equals))
				}
			case celoperators.GreaterEquals, celoperators.LessEquals:
				variable, rawValue := getVariableAndValueFromExpr(expr)
				value, ok := rawValue.(string)
				if !ok {
					return "", errors.Errorf("expect string, got %T, hint: filter literals should be string", rawValue)
				}
				if variable != "create_time" {
					return "", errors.Errorf(`">=" and "<=" are only supported for "create_time"`)
				}
				t, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return "", errors.Errorf("failed to parse time %v, error: %v", value, err)
				}
				if functionName == celoperators.GreaterEquals {
					issueFind.CreatedAtAfter = &t
				} else {
					issueFind.CreatedAtBefore = &t
				}
			case celoperators.In:
				variable, value := getVariableAndValueFromExpr(expr)
				rawList, ok := value.([]any)
				if !ok {
					return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid list value %q for %v", value, variable))
				}
				if len(rawList) == 0 {
					return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("empty list value for filter %v", variable))
				}

				switch variable {
				case "status":
					for _, raw := range rawList {
						newStatus, err := convertToAPIIssueStatus(v1pb.IssueStatus(v1pb.IssueStatus_value[raw.(string)]))
						if err != nil {
							return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to convert to issue status, err: %v", err))
						}
						issueFind.StatusList = append(issueFind.StatusList, newStatus)
					}
				case "type":
					var types []storepb.Issue_Type
					for _, raw := range rawList {
						issueType, err := convertToAPIIssueType(v1pb.Issue_Type(v1pb.Issue_Type_value[raw.(string)]))
						if err != nil {
							return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to convert to issue type, err: %v", err))
						}
						types = append(types, issueType)
					}
					issueFind.Types = &types
				case "labels":
					for _, label := range rawList {
						issueLabel, ok := label.(string)
						if !ok {
							return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf(`label should be string`))
						}
						issueFind.LabelList = append(issueFind.LabelList, issueLabel)
					}
				case "risk_level":
					for _, raw := range rawList {
						riskLevel, err := convertToAPIRiskLevel(v1pb.RiskLevel(v1pb.RiskLevel_value[raw.(string)]))
						if err != nil {
							return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("failed to convert to risk level, err: %v", err))
						}
						issueFind.RiskLevelList = append(issueFind.RiskLevelList, riskLevel)
					}
				default:
					return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unsupport variable %q with %v operator", variable, celoperators.In))
				}
			default:
				return "", connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unsupported function %q", functionName))
			}
		default:
			return "", errors.Errorf("unexpected expr kind %v", expr.Kind())
		}
		return "", nil
	}

	if _, err := parseFilter(ast.NativeRep().Expr()); err != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to parse filter"))
	}
	return issueFind, filterIssue, nil
}

func (s *IssueService) ListIssues(ctx context.Context, req *connect.Request[v1pb.ListIssuesRequest]) (*connect.Response[v1pb.ListIssuesResponse], error) {
	if req.Msg.PageSize < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("page size must be non-negative: %d", req.Msg.PageSize))
	}

	projectID, err := common.GetProjectID(req.Msg.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
	}

	offset, err := parseLimitAndOffset(&pageSize{
		token:   req.Msg.PageToken,
		limit:   int(req.Msg.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	issueFind, issueFilter, err := s.getIssueFind(ctx, req.Msg.Filter, req.Msg.Query, &limitPlusOne, &offset.offset)
	if err != nil {
		return nil, err
	}
	issueFind.ProjectIDs = []string{projectID}

	orderByKeys, err := store.GetIssueOrders(req.Msg.OrderBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	issueFind.OrderByKeys = orderByKeys

	issues, err := s.store.ListIssues(ctx, issueFind)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to search issue"))
	}

	var nextPageToken string
	if len(issues) == limitPlusOne {
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get next page token"))
		}
		issues = issues[:offset.limit]
	}

	converted, err := s.convertToIssues(ctx, issues, issueFilter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to issue"))
	}
	return connect.NewResponse(&v1pb.ListIssuesResponse{
		Issues:        converted,
		NextPageToken: nextPageToken,
	}), nil
}

func (s *IssueService) SearchIssues(ctx context.Context, req *connect.Request[v1pb.SearchIssuesRequest]) (*connect.Response[v1pb.SearchIssuesResponse], error) {
	if req.Msg.PageSize < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("page size must be non-negative: %d", req.Msg.PageSize))
	}

	projectID, err := common.GetProjectID(req.Msg.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
	}

	offset, err := parseLimitAndOffset(&pageSize{
		token:   req.Msg.PageToken,
		limit:   int(req.Msg.PageSize),
		maximum: 1000,
	})
	if err != nil {
		return nil, err
	}
	limitPlusOne := offset.limit + 1

	issueFind, issueFilter, err := s.getIssueFind(ctx, req.Msg.Filter, req.Msg.Query, &limitPlusOne, &offset.offset)
	if err != nil {
		return nil, err
	}

	orderByKeys, err := store.GetIssueOrders(req.Msg.OrderBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	issueFind.OrderByKeys = orderByKeys

	var projectIDs []string
	if projectID != "-" {
		projectIDs = append(projectIDs, projectID)
	} else {
		// Cross-project search
		user, ok := GetUserFromContext(ctx)
		if !ok {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
		}
		projectIDsFilter, err := getProjectIDsSearchFilter(ctx, user, permission.IssuesGet, s.iamManager, s.store)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get projectIDs"))
		}
		if projectIDsFilter != nil {
			projectIDs = *projectIDsFilter
		}
	}
	issueFind.ProjectIDs = projectIDs

	issues, err := s.store.ListIssues(ctx, issueFind)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to search issue"))
	}

	var nextPageToken string
	if len(issues) == limitPlusOne {
		if nextPageToken, err = offset.getNextPageToken(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get next page token"))
		}
		issues = issues[:offset.limit]
	}

	converted, err := s.convertToIssues(ctx, issues, issueFilter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to issue"))
	}
	return connect.NewResponse(&v1pb.SearchIssuesResponse{
		Issues:        converted,
		NextPageToken: nextPageToken,
	}), nil
}

func (s *IssueService) getUserByIdentifier(ctx context.Context, identifier string) (*store.UserMessage, error) {
	email := strings.TrimPrefix(identifier, "users/")
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("invalid empty creator identifier"))
	}
	account, err := s.store.GetAccountByEmail(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf(`failed to find user "%s" with error: %v`, email, err.Error()))
	}
	if account == nil {
		return nil, errors.Errorf("cannot found user %s", email)
	}
	return &store.UserMessage{
		Email:         account.Email,
		Name:          account.Name,
		Type:          account.Type,
		MemberDeleted: account.MemberDeleted,
	}, nil
}

// CreateIssue creates a issue.
func (s *IssueService) CreateIssue(ctx context.Context, req *connect.Request[v1pb.CreateIssueRequest]) (*connect.Response[v1pb.Issue], error) {
	projectID, err := common.GetProjectID(req.Msg.Parent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
	}
	project, err := s.store.GetProject(ctx, &store.FindProjectMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &projectID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get project"))
	}
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("project not found for id: %v", projectID))
	}

	if project.Setting.ForceIssueLabels && len(req.Msg.Issue.Labels) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("require issue labels"))
	}

	// enforceIssueTitle is enforced on CreatePlan (plan.Name is gated there). Issues
	// with empty Title inherit plan.Name via buildIssueMessage; ROLE_GRANT issues
	// have their own title-required check below. No gate needed here.

	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}

	issue, err := s.buildIssueMessage(ctx, project, user.Email, req.Msg)
	if err != nil {
		return nil, err
	}
	issue, err = s.store.CreateIssue(ctx, issue)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create issue"))
	}

	issue, err = postCreateIssue(ctx, s.store, s.webhookManager, s.licenseService, s.bus, project, user.Name, user.Email, issue)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	converted, err := s.convertToIssue(issue)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert to issue"))
	}

	return connect.NewResponse(converted), nil
}

func (s *IssueService) buildIssueMessage(ctx context.Context, project *store.ProjectMessage, userEmail string, request *v1pb.CreateIssueRequest) (*store.IssueMessage, error) {
	var planUID *int64
	var roleGrant *storepb.RoleGrant
	var title, description string

	// Type-specific validation and preparation
	switch request.Issue.Type {
	case v1pb.Issue_ROLE_GRANT:
		// Title is required for role grant requests.
		if strings.TrimSpace(request.Issue.Title) == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("issue title is required"))
		}

		// Check if role grant workflow feature is enabled.
		if err := s.licenseService.IsFeatureEnabled(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_REQUEST_ROLE_WORKFLOW); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.Errorf("role request requires approval workflow feature (available in Enterprise plan)"))
		}

		// Validate role grant fields.
		if request.Issue.RoleGrant.GetRole() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("expect role grant role"))
		}
		if request.Issue.RoleGrant.GetUser() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("expect role grant user"))
		}

		// Validate CEL expression if it's not empty.
		if expression := request.Issue.RoleGrant.GetCondition().GetExpression(); expression != "" {
			e, err := cel.NewEnv(common.IAMPolicyConditionCELAttributes...)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create cel environment"))
			}
			if _, issues := e.Compile(expression); issues != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("found issues in role grant condition expression, issues: %v", issues.String()))
			}
		}

		roleGrantUserEmail, err := common.GetUserEmail(request.Issue.RoleGrant.User)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user email from %q", request.Issue.RoleGrant.User))
		}
		roleGrantUser, err := s.store.GetUserByEmail(ctx, roleGrantUserEmail)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user by email %q", roleGrantUserEmail))
		}
		if roleGrantUser == nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("user %q not found", request.Issue.RoleGrant.User))
		}
		roleGrant = &storepb.RoleGrant{
			Role:       request.Issue.RoleGrant.Role,
			User:       common.FormatUserEmail(roleGrantUser.Email),
			Condition:  request.Issue.RoleGrant.Condition,
			Expiration: request.Issue.RoleGrant.Expiration,
		}

		title = strings.TrimSpace(request.Issue.Title)
		description = request.Issue.Description

	case v1pb.Issue_DATABASE_CHANGE, v1pb.Issue_DATABASE_EXPORT:
		// Validate and fetch plan (shared logic for both types)
		if request.Issue.Plan == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("plan is required"))
		}

		_, planID, err := common.GetProjectIDPlanID(request.Issue.Plan)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("%v", err.Error()))
		}

		plan, err := s.store.GetPlan(ctx, &store.FindPlanMessage{Workspace: common.GetWorkspaceIDFromContext(ctx), UID: &planID, ProjectID: project.ResourceID})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get plan"))
		}
		if plan == nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("plan %d not found in project %s", planID, project.ResourceID))
		}
		planUID = &plan.UID

		// Use plan's title and description as defaults if not provided by request
		title = strings.TrimSpace(request.Issue.Title)
		if title == "" {
			title = plan.Name
		}
		description = request.Issue.Description
		if description == "" {
			description = plan.Description
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("unknown issue type %q", request.Issue.Type))
	}

	// Convert v1pb.Issue_Type to storepb.Issue_Type
	issueType, err := convertToAPIIssueType(request.Issue.Type)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to convert issue type"))
	}

	// Build the issue message (common structure)
	issue := &store.IssueMessage{
		ProjectID:    project.ResourceID,
		CreatorEmail: userEmail,
		PlanUID:      planUID,
		Title:        title,
		Status:       storepb.Issue_OPEN,
		Type:         issueType,
		Description:  description,
		Payload: &storepb.Issue{
			RoleGrant: roleGrant,
			Approval: &storepb.IssuePayloadApproval{
				ApprovalFindingDone: false,
				ApprovalTemplate:    nil,
				Approvers:           nil,
			},
			Labels: request.Issue.Labels,
		},
	}

	return issue, nil
}

