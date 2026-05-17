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
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// MFA-related constants.
const (
	// mfaTempTokenDuration is the duration for MFA temporary tokens.
	// Following industry standards (Okta: 5 minutes, Auth0: 10 minutes, AWS Cognito: 3 minutes).
	// A short duration reduces the attack window for TOTP brute-force attempts.
	mfaTempTokenDuration = 5 * time.Minute

	// MFA phase: 5 failed attempts within 5 minutes.
	mfaMaxFailedAttempts = 5
	mfaLockoutWindow     = 5 * time.Minute

	// Error messages for MFA authentication failures.
	// These constants are used both for error responses and for querying audit logs during rate limiting.
	errMsgInvalidMFACode      = "invalid MFA code"
	errMsgInvalidRecoveryCode = "invalid recovery code"
	errMsgTooManyMFA          = "too many failed MFA attempts, please try again later"
)

// authenticateLogin is the main dispatcher for all authentication methods.
func (s *AuthService) authenticateLogin(ctx context.Context, request *v1pb.LoginRequest) (*store.UserMessage, error) {
	mfaSecondLogin := request.GetMfaTempToken() != ""

	if mfaSecondLogin {
		return s.completeMFALogin(ctx, request)
	}

	if request.GetIdpName() != "" {
		return s.getOrCreateUserWithIDP(ctx, request)
	}

	if request.EmailCode != nil && *request.EmailCode != "" {
		return s.authenticateEmailCodeLogin(ctx, request)
	}

	return s.getAndVerifyUser(ctx, request)
}

// completeMFALogin validates MFA temp token and verifies OTP or recovery code.
func (s *AuthService) completeMFALogin(ctx context.Context, request *v1pb.LoginRequest) (*store.UserMessage, error) {
	userEmail, err := auth.GetUserEmailFromMFATempToken(*request.MfaTempToken, s.secret)
	if err != nil {
		return nil, err
	}
	user, err := s.store.GetUserByEmail(ctx, userEmail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find user"))
	}
	if user == nil {
		return nil, invalidCredentialsError
	}

	if err := s.checkMFALockout(ctx, user.Email); err != nil {
		return nil, err
	}

	if request.OtpCode != nil {
		if err := challengeMFACode(user, *request.OtpCode); err != nil {
			return nil, err
		}
	} else if request.RecoveryCode != nil {
		if err := s.challengeRecoveryCode(ctx, user, *request.RecoveryCode); err != nil {
			return nil, err
		}
	} else {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("OTP or recovery code is required for MFA"))
	}
	return user, nil
}

// checkMFARequired checks if MFA is required and returns a response with temp token if so.
// Returns (nil, nil) if MFA is not required or already completed.
func (s *AuthService) checkMFARequired(ctx context.Context, user *store.UserMessage, workspaceID string, mfaSecondLogin bool) (*connect.Response[v1pb.LoginResponse], error) {
	if mfaSecondLogin {
		return nil, nil
	}

	userMFAEnabled := user.MFAConfig != nil && user.MFAConfig.OtpSecret != ""
	mfaFeatureEnabled := s.licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_TWO_FA) == nil
	if !mfaFeatureEnabled || !userMFAEnabled {
		return nil, nil
	}

	mfaTempToken, err := auth.GenerateMFATempToken(user.Email, s.secret, mfaTempTokenDuration)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to generate MFA temp token"))
	}

	return connect.NewResponse(&v1pb.LoginResponse{
		MfaTempToken: &mfaTempToken,
	}), nil
}

// validateLoginPermissions checks if the user is allowed to login.
func (s *AuthService) validateLoginPermissions(ctx context.Context, user *store.UserMessage, workspaceID string, request *v1pb.LoginRequest) error {
	if user.MemberDeleted {
		return connect.NewError(connect.CodeUnauthenticated, errors.Errorf("user has been deactivated by administrators"))
	}

	isAdmin, err := isUserWorkspaceAdmin(ctx, s.store, user, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to check user roles"))
	}

	// Skip restrictions for workspace admins and service accounts
	if isAdmin || user.Type != storepb.PrincipalType_END_USER {
		return nil
	}

	// Skip restrictions for MFA second login (already validated in first step)
	mfaSecondLogin := request.GetMfaTempToken() != ""
	if mfaSecondLogin {
		return nil
	}

	restriction, err := getAccountRestriction(
		ctx,
		s.store,
		s.licenseService,
		s.profile.SaaS,
		workspaceID,
	)
	if err != nil {
		return err
	}
	if request.GetIdpName() == "" {
		if request.Password != "" {
			if restriction.DisallowPasswordSignin {
				return connect.NewError(connect.CodePermissionDenied, errors.Errorf("password signin is disallowed"))
			}
		}
		if request.EmailCode != nil && *request.EmailCode != "" {
			if !restriction.AllowEmailCodeSignin {
				return connect.NewError(connect.CodeFailedPrecondition, errors.Errorf("email code login is not enabled for this workspace"))
			}
		}
	}

	// Check domain restriction
	return validateEmailWithDomains(ctx, s.licenseService, s.store, workspaceID, user.Email, false)
}

// needResetPassword checks whether the user should be asked to change their password.
func (s *AuthService) needResetPassword(ctx context.Context, user *store.UserMessage, workspaceID string) bool {
	// Reset password restriction only works for end user with email & password login.
	if user.Type != storepb.PrincipalType_END_USER {
		return false
	}

	restriction, err := getAccountRestriction(
		ctx,
		s.store,
		s.licenseService,
		s.profile.SaaS,
		workspaceID,
	)
	if err != nil {
		slog.Error("failed to get workspace restriction", log.BBError(err), slog.String("workspace", workspaceID))
		return false
	}

	// Don't need to reset password if password signin is not allowed.
	if restriction.DisallowPasswordSignin {
		return false
	}

	if restriction.PasswordRestriction == nil {
		return false
	}

	rotation := restriction.PasswordRestriction.GetPasswordRotation()
	if rotation == nil || rotation.AsDuration() <= 0 {
		return false
	}

	lastChangedAt := user.Profile.GetLastChangePasswordTime()
	if lastChangedAt == nil {
		return true
	}

	return time.Since(lastChangedAt.AsTime()) >= rotation.AsDuration()
}

// generateLoginToken generates the appropriate token based on user type.
func (s *AuthService) generateLoginToken(ctx context.Context, user *store.UserMessage, workspaceID string) (string, error) {
	tokenDuration := auth.GetAccessTokenDuration(ctx, s.store, s.licenseService, workspaceID)

	var token string
	var err error
	switch user.Type {
	case storepb.PrincipalType_END_USER:
		token, err = auth.GenerateAccessToken(user.Email, workspaceID, s.secret, tokenDuration)
	case storepb.PrincipalType_SERVICE_ACCOUNT:
		token, err = auth.GenerateAPIToken(user.Email, workspaceID, s.secret)
	default:
		return "", connect.NewError(connect.CodeUnauthenticated, errors.Errorf("user type %s cannot login", user.Type))
	}
	if err != nil {
		return "", err
	}
	return token, nil
}

// finalizeLogin builds the response, sets cookies if needed, and updates the user profile.
func (s *AuthService) finalizeLogin(ctx context.Context, req *connect.Request[v1pb.LoginRequest], user *store.UserMessage, token string, workspaceID string, requireResetPassword bool) (*connect.Response[v1pb.LoginResponse], error) {
	response := &v1pb.LoginResponse{
		RequireResetPassword: requireResetPassword,
	}
	resp := connect.NewResponse(response)

	// Token mode: When the X-Auth-Mode header is "token", return tokens in the
	// response body instead of cookies. This supports standalone frontend
	// deployments where the frontend is served from a different origin.
	authMode := req.Header().Get("X-Auth-Mode")
	if authMode == "token" {
		response.Token = token

		// Also generate a refresh token for the body.
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
		// RefreshToken is not in the LoginResponse protobuf, so we send it
		// as a response header that the frontend token-manager reads.
		resp.Header().Set("X-Refresh-Token", refreshToken)
	} else if req.Msg.Web {
		if user.Type != storepb.PrincipalType_END_USER {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("only users can use web login"))
		}
		origin := req.Header().Get("Origin")
		cookie := auth.GetTokenCookie(ctx, s.store, s.licenseService, workspaceID, origin, token)
		resp.Header().Add("Set-Cookie", cookie.String())

		// Issue refresh token for web login
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
	} else {
		// For non-web clients (CLI, API), return the token in the response body.
		response.Token = token
	}

	if user.Type == storepb.PrincipalType_END_USER {
		if _, err := s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{
			Profile: &storepb.UserProfile{
				LastLoginTime:          timestamppb.Now(),
				LastChangePasswordTime: user.Profile.GetLastChangePasswordTime(),
				Source:                 user.Profile.GetSource(),
				LastLoginWorkspace:     workspaceID,
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
	}

	return resp, nil
}

// getAndVerifyUser verifies email+password and returns the user.
func (s *AuthService) getAndVerifyUser(ctx context.Context, request *v1pb.LoginRequest) (*store.UserMessage, error) {
	// Check if user is locked out due to too many failed password attempts.
	if err := s.checkPasswordLockout(ctx, request.Email); err != nil {
		return nil, err
	}

	// GetAccountByEmail is cross-workspace, which is correct for login.
	// Email is globally unique (PK). The token gets workspace from account.Workspace (SA/WI)
	// or from the default workspace (END_USER).
	account, err := s.store.GetAccountByEmail(ctx, request.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user by email %q", request.Email))
	}
	if account == nil {
		return nil, invalidCredentialsError
	}
	// Compare the stored hashed password, with the hashed version of the password that was received.
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(request.Password)); err != nil {
		// If the two passwords don't match, return a 401 status.
		return nil, invalidCredentialsError
	}

	// Convert AccountMessage to UserMessage for downstream use.
	return s.accountToUser(ctx, account)
}

// accountToUser converts an AccountMessage to a UserMessage.
// For END_USER, loads the full user record. For SA/WI, constructs a minimal UserMessage.
func (s *AuthService) accountToUser(ctx context.Context, account *store.AccountMessage) (*store.UserMessage, error) {
	if account.Type == storepb.PrincipalType_END_USER {
		user, err := s.store.GetUserByEmail(ctx, account.Email)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user %q", account.Email))
		}
		if user == nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("user %q not found", account.Email))
		}
		return user, nil
	}

	// SA/WI: construct a minimal UserMessage with the fields available from AccountMessage.
	return &store.UserMessage{
		Email:         account.Email,
		Name:          account.Name,
		Type:          account.Type,
		MemberDeleted: account.MemberDeleted,
	}, nil
}
