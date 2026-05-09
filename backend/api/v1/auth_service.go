package v1

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bytebase/bytebase/backend/api/auth"
	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/store"
)

// AuthService implements the auth service.
//
// Methods are organized across multiple files:
//   - auth_service.go        — struct, constructor, Login, Signup (this file)
//   - auth_service_login.go  — authenticateLogin dispatcher, MFA flow, login helpers
//   - auth_service_mfa.go    — MFA challenge, lockout, rate limiting
//   - auth_service_idp.go    — IDP (OAuth2/OIDC/LDAP) authentication, group sync
//   - auth_service_token.go  — Logout, Refresh, SwitchWorkspace, ExchangeToken
//   - auth_service_password.go — Password reset, email verification codes
//   - auth_service_helpers.go — Workspace resolution, account restrictions, utilities
type AuthService struct {
	v1connect.UnimplementedAuthServiceHandler
	store          *store.Store
	secret         string
	licenseService *enterprise.LicenseService
	profile        *config.Profile
	iamManager     *iam.Manager
}

// NewAuthService creates a new AuthService.
func NewAuthService(store *store.Store, secret string, licenseService *enterprise.LicenseService, profile *config.Profile, iamManager *iam.Manager) *AuthService {
	return &AuthService{
		store:          store,
		secret:         secret,
		licenseService: licenseService,
		profile:        profile,
		iamManager:     iamManager,
	}
}

// Login is the auth login method including SSO.
func (s *AuthService) Login(ctx context.Context, req *connect.Request[v1pb.LoginRequest]) (*connect.Response[v1pb.LoginResponse], error) {
	request := req.Msg
	mfaSecondLogin := request.GetMfaTempToken() != ""

	// 1. Authenticate user (password, IDP, or MFA completion)
	loginUser, err := s.authenticateLogin(ctx, request)
	if err != nil {
		return nil, err
	}

	// 2. Resolve workspace early so all subsequent checks can use it.
	// Login is allow_without_credential, so workspace is NOT in the context from auth middleware.
	preferredWS, _ := parseOptionalWorkspace(request.Workspace)
	workspaceID, err := s.resolveWorkspaceForLogin(ctx, loginUser, preferredWS)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to resolve workspace"))
	}
	common.SetAuditWorkspaceID(ctx, workspaceID)

	// 3. Post-auth checks (deleted, domain, license)
	if err := s.validateLoginPermissions(ctx, loginUser, workspaceID, request); err != nil {
		return nil, err
	}

	// 4. Check if MFA challenge needed (returns early with temp token)
	if resp, err := s.checkMFARequired(ctx, loginUser, workspaceID, mfaSecondLogin); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	}

	// 5. Generate token (workspace already resolved)
	token, err := s.generateLoginToken(ctx, loginUser, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to generate access token"))
	}

	// 6. Build response and finalize
	requireResetPassword := s.needResetPassword(ctx, loginUser, workspaceID)
	return s.finalizeLogin(ctx, req, loginUser, token, workspaceID, requireResetPassword)
}

// Signup registers a new user account (self-service).
// Creates a principal and assigns a workspace:
// - If the user's email was pre-invited to a workspace, joins that workspace.
// - Otherwise, creates a new workspace with the user as admin.
func (s *AuthService) Signup(ctx context.Context, req *connect.Request[v1pb.SignupRequest]) (*connect.Response[v1pb.LoginResponse], error) {
	request := req.Msg
	if request.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email must be set"))
	}
	if request.Title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("title must be set"))
	}
	if request.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("password must be set"))
	}
	if err := validateEndUserEmail(request.Email); err != nil {
		return nil, err
	}

	// Check if principal already exists.
	existingUser, err := s.store.GetUserByEmail(ctx, request.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find user by email"))
	}
	if existingUser != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.Errorf("email %s is already registered", request.Email))
	}

	// Resolve the target workspace (read-only) so we can check restrictions BEFORE
	// any write — otherwise a rejected signup would leave an orphan user/workspace behind.
	targetWorkspaceID, _, err := s.resolveWorkspaceIDByEmail(ctx, request.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to resolve target workspace"))
	}
	// Announce the workspace on every exit path so denied signups (DisallowSignup,
	// password restriction) still produce audit entries. Uses targetWorkspaceID (resolved
	// before any writes) rather than the provisioned workspaceID.
	defer func() { common.SetAuditWorkspaceID(ctx, targetWorkspaceID) }()

	restriction, err := getAccountRestriction(
		ctx,
		s.store,
		s.licenseService,
		s.profile.SaaS,
		targetWorkspaceID,
	)
	if err != nil {
		return nil, err
	}
	if restriction.DisallowSignup {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("sign up is disallowed for this workspace %v", targetWorkspaceID))
	}
	if err := validatePasswordWithRestriction(request.Password, convertToStorePasswordRestriction(restriction.PasswordRestriction)); err != nil {
		return nil, err
	}

	workspaceID, err := s.provisionWorkspaceForNewUser(ctx, request.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to provision workspace"))
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to generate password hash"))
	}

	// Step 2: Create the principal (global identity).
	user, err := s.store.CreateUser(ctx, &store.UserMessage{
		Email:        request.Email,
		Name:         request.Title,
		PasswordHash: string(passwordHash),
		Profile:      &storepb.UserProfile{},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create user"))
	}

	// Step 3: Generate token and finalize login.
	tokenDuration := auth.GetAccessTokenDuration(ctx, s.store, s.licenseService, workspaceID)
	token, err := auth.GenerateAccessToken(user.Email, workspaceID, s.secret, tokenDuration)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to generate access token"))
	}

	response := &v1pb.LoginResponse{}
	resp := connect.NewResponse(response)

	// Signup is always web-based — set tokens as HTTP-only cookies.
	origin := req.Header().Get("Origin")
	cookie := auth.GetTokenCookie(ctx, s.store, s.licenseService, workspaceID, origin, token)
	resp.Header().Add("Set-Cookie", cookie.String())

	refreshToken, err := auth.GenerateOpaqueToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate refresh token"))
	}
	refreshTokenDuration := auth.GetRefreshTokenDuration(ctx, s.store, s.licenseService, workspaceID)
	if err := s.store.CreateWebRefreshToken(ctx, &store.WebRefreshTokenMessage{
		TokenHash: auth.HashToken(refreshToken),
		UserEmail: user.Email,
		ExpiresAt: time.Now().Add(refreshTokenDuration),
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to create refresh token"))
	}
	refreshCookie := auth.GetRefreshTokenCookie(origin, refreshToken, refreshTokenDuration)
	resp.Header().Add("Set-Cookie", refreshCookie.String())

	// Update last login time and workspace.
	if _, err := s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{
		Profile: &storepb.UserProfile{
			LastLoginTime:      timestamppb.Now(),
			Source:             user.Profile.GetSource(),
			LastLoginWorkspace: workspaceID,
		},
	}); err != nil {
		slog.Error("failed to update user profile", log.BBError(err), slog.String("user", user.Email))
	}

	v1User, err := convertToUser(ctx, s.iamManager, user)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to convert user"))
	}
	v1User.Workspace = common.FormatWorkspace(workspaceID)
	response.User = v1User

	return resp, nil
}
