package v1

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/api/auth"
	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/idp/wif"
	"github.com/bytebase/bytebase/backend/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Logout is the auth logout method.
func (s *AuthService) Logout(ctx context.Context, req *connect.Request[v1pb.LogoutRequest]) (*connect.Response[emptypb.Empty], error) {
	// Delete refresh token from database if present
	if refreshToken := auth.GetRefreshTokenFromCookie(req.Header()); refreshToken != "" {
		if err := s.store.DeleteWebRefreshToken(ctx, auth.HashToken(refreshToken)); err != nil {
			slog.Error("failed to delete refresh token on logout", log.BBError(err))
		}
	}

	resp := connect.NewResponse(&emptypb.Empty{})

	origin := req.Header().Get("Origin")
	// Clear access token cookie
	resp.Header().Add("Set-Cookie", auth.GetTokenCookie(ctx, s.store, s.licenseService, common.GetWorkspaceIDFromContext(ctx), origin, "").String())
	// Clear refresh token cookie
	resp.Header().Add("Set-Cookie", auth.GetRefreshTokenCookie(origin, "", 0).String())
	return resp, nil
}

// Refresh exchanges a refresh token for new access and refresh tokens.
func (s *AuthService) Refresh(ctx context.Context, req *connect.Request[v1pb.RefreshRequest]) (*connect.Response[v1pb.RefreshResponse], error) {
	// 1. Extract refresh token from cookie
	refreshToken := auth.GetRefreshTokenFromCookie(req.Header())
	if refreshToken == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("refresh token not found"))
	}

	// 2. Look up and delete atomically
	tokenHash := auth.HashToken(refreshToken)
	stored, err := s.store.GetAndDeleteWebRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to get refresh token"))
	}
	if stored == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid refresh token"))
	}

	// 3. Check expiration
	if time.Now().After(stored.ExpiresAt) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("refresh token expired"))
	}

	// 4. Get user
	user, err := s.store.GetUserByEmail(ctx, stored.UserEmail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to get user"))
	}
	if user == nil || user.MemberDeleted {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not found"))
	}

	// 5. Extract workspace from the access token cookie (still present because cookie
	// outlives the JWT by 30 seconds). This ensures per-session workspace isolation.
	// Also verify the token's subject matches the refresh token's user to prevent
	// pairing a refresh token with an access token from a different session.
	accessTokenStr, err := auth.GetTokenFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Wrap(err, "invalid access token header"))
	}
	if accessTokenStr == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("access token cookie required for refresh"))
	}
	tokenClaims, err := auth.ExtractClaimsFromExpiredToken(accessTokenStr, s.secret)
	if err != nil || tokenClaims.WorkspaceID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("failed to extract workspace from access token"))
	}
	if tokenClaims.Subject != stored.UserEmail {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("access token does not match refresh token"))
	}
	workspaceID := tokenClaims.WorkspaceID

	// Verify the user is still a member of the workspace.
	ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
		WorkspaceID:    &workspaceID,
		Email:          user.Email,
		IncludeAllUser: !s.profile.SaaS,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to verify workspace membership"))
	}
	if ws == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("user %q is no longer a member of workspace %q", user.Email, workspaceID))
	}

	accessTokenDuration := auth.GetAccessTokenDuration(ctx, s.store, s.licenseService, workspaceID)
	accessToken, err := auth.GenerateAccessToken(user.Email, workspaceID, s.secret, accessTokenDuration)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate access token"))
	}

	newRefreshToken, err := auth.GenerateOpaqueToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate refresh token"))
	}

	// Inherit expiration from the original token (absolute session lifetime)
	if err := s.store.CreateWebRefreshToken(ctx, &store.WebRefreshTokenMessage{
		TokenHash: auth.HashToken(newRefreshToken),
		UserEmail: user.Email,
		ExpiresAt: stored.ExpiresAt,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to create refresh token"))
	}

	// 6. Set cookies and return
	resp := connect.NewResponse(&v1pb.RefreshResponse{})
	origin := req.Header().Get("Origin")
	resp.Header().Add("Set-Cookie", auth.GetTokenCookie(ctx, s.store, s.licenseService, workspaceID, origin, accessToken).String())
	resp.Header().Add("Set-Cookie", auth.GetRefreshTokenCookie(origin, newRefreshToken, time.Until(stored.ExpiresAt)).String())

	return resp, nil
}

// SwitchWorkspace switches the current user's active workspace and issues new tokens.
func (s *AuthService) SwitchWorkspace(ctx context.Context, req *connect.Request[v1pb.SwitchWorkspaceRequest]) (*connect.Response[v1pb.LoginResponse], error) {
	request := req.Msg
	if request.Workspace == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace is required"))
	}

	workspaceID, err := common.GetWorkspaceID(request.Workspace)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrap(err, "invalid workspace name"))
	}

	user, ok := GetUserFromContext(ctx)
	if !ok || user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user not found"))
	}
	if user.Type != storepb.PrincipalType_END_USER {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("only end users can switch workspaces"))
	}

	// Reject OAuth2 tokens — they are bound to a specific workspace via the OAuth client
	// and must not be used to mint plain user tokens for other workspaces.
	accessTokenStr, _ := auth.GetTokenFromHeaders(req.Header())
	if accessTokenStr != "" {
		tokenClaims, err := auth.ExtractClaimsFromExpiredToken(accessTokenStr, s.secret)
		if err == nil && slices.Contains(tokenClaims.Audience, auth.OAuth2AccessTokenAudience) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("OAuth2 tokens cannot be used to switch workspaces"))
		}
	}

	// Verify the user is a member of the target workspace.
	ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
		WorkspaceID:    &workspaceID,
		Email:          user.Email,
		IncludeAllUser: !s.profile.SaaS,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to find workspace"))
	}
	if ws == nil {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("not a member of workspace %q", workspaceID))
	}

	// Validate the target workspace's sign-in policies.
	if user.MemberDeleted {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user has been deactivated"))
	}
	if err := validateEmailWithDomains(ctx, s.licenseService, s.store, workspaceID, user.Email, false); err != nil {
		return nil, err
	}

	// Check MFA requirement for the target workspace.
	mfaSecondStep := request.GetMfaTempToken() != ""
	if mfaSecondStep {
		// Check MFA lockout before verifying.
		if err := s.checkMFALockout(ctx, user.Email); err != nil {
			return nil, err
		}
		// Verify the MFA temp token and OTP/recovery code.
		mfaEmail, err := auth.GetUserEmailFromMFATempToken(*request.MfaTempToken, s.secret)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid MFA temp token"))
		}
		if mfaEmail != user.Email {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("MFA token does not match user"))
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
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("OTP or recovery code required"))
		}
	} else {
		// First step: check if MFA is required for the target workspace.
		if resp, err := s.checkMFARequired(ctx, user, workspaceID, false); err != nil {
			return nil, err
		} else if resp != nil {
			return resp, nil
		}
	}

	// Generate new token with target workspace.
	token, err := s.generateLoginToken(ctx, user, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate token"))
	}

	// Update last login workspace.
	if _, err := s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{
		Profile: &storepb.UserProfile{
			LastLoginTime:          user.Profile.GetLastLoginTime(),
			LastChangePasswordTime: user.Profile.GetLastChangePasswordTime(),
			Source:                 user.Profile.GetSource(),
			LastLoginWorkspace:     workspaceID,
		},
	}); err != nil {
		slog.Error("failed to update user profile", log.BBError(err))
	}

	// Build response.
	response := &v1pb.LoginResponse{}
	resp := connect.NewResponse(response)

	if request.Web {
		// Require a valid refresh token cookie — prevents non-web clients from
		// upgrading a short-lived bearer token into a long-lived web session.
		oldRefreshToken := auth.GetRefreshTokenFromCookie(req.Header())
		if oldRefreshToken == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("refresh token cookie required for web workspace switch"))
		}
		oldStored, err := s.store.GetAndDeleteWebRefreshToken(ctx, auth.HashToken(oldRefreshToken))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to consume refresh token"))
		}
		if oldStored == nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired refresh token"))
		}
		if oldStored.UserEmail != user.Email {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("refresh token does not belong to current user"))
		}
		sessionExpiresAt := oldStored.ExpiresAt
		if sessionExpiresAt.IsZero() {
			sessionExpiresAt = time.Now().Add(auth.GetRefreshTokenDuration(ctx, s.store, s.licenseService, workspaceID))
		}

		origin := req.Header().Get("Origin")
		cookie := auth.GetTokenCookie(ctx, s.store, s.licenseService, workspaceID, origin, token)
		resp.Header().Add("Set-Cookie", cookie.String())

		newRefreshToken, err := auth.GenerateOpaqueToken()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate refresh token"))
		}
		if err := s.store.CreateWebRefreshToken(ctx, &store.WebRefreshTokenMessage{
			TokenHash: auth.HashToken(newRefreshToken),
			UserEmail: user.Email,
			ExpiresAt: sessionExpiresAt,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to create refresh token"))
		}
		refreshCookie := auth.GetRefreshTokenCookie(origin, newRefreshToken, time.Until(sessionExpiresAt))
		resp.Header().Add("Set-Cookie", refreshCookie.String())
	} else {
		response.Token = token
	}

	v1User, err := convertToUser(ctx, s.iamManager, user)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to convert user"))
	}
	v1User.Workspace = common.FormatWorkspace(workspaceID)
	response.User = v1User

	return resp, nil
}

// ExchangeToken exchanges an external OIDC token for a Bytebase access token.
// Used by CI/CD pipelines with Workload Identity Federation.
func (s *AuthService) ExchangeToken(ctx context.Context, req *connect.Request[v1pb.ExchangeTokenRequest]) (*connect.Response[v1pb.ExchangeTokenResponse], error) {
	request := req.Msg

	if request.Token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}
	if request.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}

	if err := validateWorkloadIdentityEmail(request.Email); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			invalidAccountEmailError("workload identity", request.Email, err))
	}

	// Find workload identity by email (cross-workspace lookup since this is unauthenticated).
	wi, err := s.store.GetWorkloadIdentityByEmail(ctx, request.Email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to find workload identity"))
	}
	if wi == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("workload identity %q not found", request.Email))
	}
	// Announce the workspace as soon as we know it (from the WI record) so
	// that a deactivated-WI attempt — which compliance wants to see — still
	// lands in the audit log.
	common.SetAuditWorkspaceID(ctx, wi.Workspace)
	if wi.MemberDeleted {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			errors.New("workload identity has been deactivated"))
	}

	// Get workload identity config
	wicConfig := wi.Config
	if wicConfig == nil {
		return nil, connect.NewError(connect.CodeInternal,
			errors.New("workload identity config not found"))
	}

	// Validate OIDC token
	if _, err = wif.ValidateToken(ctx, request.Token, wicConfig); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated,
			errors.Wrap(err, "token validation failed"))
	}

	// Generate Bytebase API token using workspace from the WI record.
	token, err := auth.GenerateAPIToken(wi.Email, wi.Workspace, s.secret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			errors.Wrap(err, "failed to generate access token"))
	}

	return connect.NewResponse(&v1pb.ExchangeTokenResponse{
		AccessToken: token,
	}), nil
}
