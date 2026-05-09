package v1

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/idp/ldap"
	"github.com/bytebase/bytebase/backend/plugin/idp/oauth2"
	"github.com/bytebase/bytebase/backend/plugin/idp/oidc"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/utils"
)

// getOrCreateUserWithIDP authenticates a user via an identity provider (SSO).
// Login API has allow_without_credential, so there's no workspace in the token context.
// We resolve workspace from the IDP entity (IDP resource_id is globally unique).
func (s *AuthService) getOrCreateUserWithIDP(ctx context.Context, request *v1pb.LoginRequest) (*store.UserMessage, error) {
	idpID, err := common.GetIdentityProviderID(request.IdpName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "failed to get identity provider ID"))
	}
	// Look up IDP without workspace filter — IDP resource_id is globally unique.
	// The workspace is resolved from the IDP entity.
	idp, err := s.store.GetIdentityProviderByID(ctx, idpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get identity provider"))
	}
	if idp == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("identity provider not found"))
	}

	// For workspace-scoped IDPs, use the IDP's workspace.
	// For global IDPs (SaaS), workspace is resolved after authentication from user membership.
	workspaceID := idp.Workspace
	externalURL, err := utils.GetEffectiveExternalURL(ctx, s.store, s.profile, workspaceID)
	if err != nil {
		return nil, err
	}

	var userInfo *storepb.IdentityProviderUserInfo
	switch idp.Type {
	case storepb.IdentityProviderType_OAUTH2:
		oauth2Context := request.IdpContext.GetOauth2Context()
		if oauth2Context == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("missing OAuth2 context"))
		}
		oauth2IdentityProvider, err := oauth2.NewIdentityProvider(idp.Config.GetOauth2Config())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create new OAuth2 identity provider"))
		}
		redirectURL := fmt.Sprintf("%s/oauth/callback", externalURL)
		token, err := oauth2IdentityProvider.ExchangeToken(ctx, redirectURL, oauth2Context.Code)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to exchange token"))
		}
		userInfo, _, err = oauth2IdentityProvider.UserInfo(token)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user info"))
		}
	case storepb.IdentityProviderType_OIDC:
		oidcContext := request.IdpContext.GetOidcContext()
		if oidcContext == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("missing OIDC context"))
		}

		oidcIDP, err := oidc.NewIdentityProvider(ctx, idp.Config.GetOidcConfig())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create new OIDC identity provider"))
		}

		redirectURL := fmt.Sprintf("%s/oidc/callback", externalURL)
		token, err := oidcIDP.ExchangeToken(ctx, redirectURL, oidcContext.Code)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to exchange token"))
		}

		userInfo, _, err = oidcIDP.UserInfo(ctx, token, "")
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user info"))
		}
	case storepb.IdentityProviderType_LDAP:
		idpConfig := idp.Config.GetLdapConfig()
		ldapIDP, err := ldap.NewIdentityProvider(
			ldap.IdentityProviderConfig{
				Host:             idpConfig.Host,
				Port:             int(idpConfig.Port),
				SkipTLSVerify:    idpConfig.SkipTlsVerify,
				BindDN:           idpConfig.BindDn,
				BindPassword:     idpConfig.BindPassword,
				BaseDN:           idpConfig.BaseDn,
				UserFilter:       idpConfig.UserFilter,
				SecurityProtocol: idpConfig.SecurityProtocol,
				FieldMapping:     idpConfig.FieldMapping,
			},
		)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create new LDAP identity provider"))
		}

		userInfo, err = ldapIDP.Authenticate(request.Email, request.Password)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get user info"))
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("identity provider type %s not supported", idp.Type.String()))
	}
	if userInfo == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("failed to get user info from identity provider %q", idp.Title))
	}
	if userInfo.Identifier == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("missing identifier in user info from identity provider %q", idp.Title))
	}
	// The userinfo's email comes from identity provider, it has to be converted to lower-case.
	email := strings.ToLower(userInfo.Identifier)
	if err := common.ValidateEmail(email); err != nil {
		// If the email is invalid, we will try to use the domain and identifier to construct the email.
		domain := extractDomain(idp.Domain)
		if domain != "" {
			email = strings.ToLower(fmt.Sprintf("%s@%s", email, domain))
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "invalid email %q", userInfo.Identifier))
		}
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list users by email %s", email))
	}
	// User login through global SSO, then we should auth-resolve the workspace id.
	if user != nil && workspaceID == "" {
		wsID, err := s.resolveWorkspaceForLogin(ctx, user, "")
		if err != nil {
			slog.Warn("failed to resolve workspace", slog.String("user", user.Email), log.BBError(err))
		}
		workspaceID = wsID
	}
	// First time login through SSO
	if user == nil {
		if workspaceID != "" {
			if err := validateEmailWithDomains(ctx, s.licenseService, s.store, workspaceID, email, false); err != nil {
				return nil, err
			}

			// We will only block new create creation and still allow SSO login from existing users.
			featurePlan := v1pb.PlanFeature_FEATURE_ENTERPRISE_SSO
			if idp.Type == storepb.IdentityProviderType_OAUTH2 && googleGitHubDomains[idp.Domain] {
				featurePlan = v1pb.PlanFeature_FEATURE_GOOGLE_AND_GITHUB_SSO
			}
			if err := s.licenseService.IsFeatureEnabled(ctx, workspaceID, featurePlan); err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, err)
			}

			if err := userCountGuard(ctx, s.store, s.licenseService, workspaceID, nil, s.profile.SaaS); err != nil {
				return nil, err
			}
		}

		// Create new user from identity provider.
		password, err := common.RandomString(20)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to generate random password"))
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Errorf("failed to generate password hash"))
		}

		newUser, err := s.store.CreateUser(ctx, &store.UserMessage{
			Name:         userInfo.DisplayName,
			Email:        email,
			Phone:        userInfo.Phone,
			Type:         storepb.PrincipalType_END_USER,
			PasswordHash: string(passwordHash),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create user"))
		}

		user = newUser
	}

	if workspaceID == "" {
		// Global IDP: create a new workspace for the user (same as Signup flow).
		wsID, err := common.RandomString(16)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to generate workspace ID"))
		}
		ws, err := s.store.CreateWorkspace(ctx, &store.WorkspaceMessage{
			ResourceID:         wsID,
			Payload:            &storepb.WorkspacePayload{Title: "Default Workspace"},
			AdditionalSettings: s.getAdditionalWorkspaceSettings(),
		}, email)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create workspace"))
		}
		workspaceID = ws.ResourceID
	} else {
		// Workspace-scoped IDP: add user as member only if not already in the workspace.
		ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
			WorkspaceID:    &workspaceID,
			Email:          email,
			IncludeAllUser: !s.profile.SaaS,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrap(err, "failed to check workspace membership"))
		}
		if ws == nil {
			if _, err := s.store.PatchWorkspaceIamPolicy(ctx, &store.PatchIamPolicyMessage{
				Workspace: workspaceID,
				Member:    common.FormatUserEmail(email),
				Roles:     []string{common.FormatRole(store.WorkspaceMemberRole)},
			}); err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to add user to workspace"))
			}
		}
	}

	if user.MemberDeleted {
		if err := userCountGuard(ctx, s.store, s.licenseService, workspaceID, nil, s.profile.SaaS); err != nil {
			return nil, err
		}
		// Undelete the user when login via SSO.
		user, err = s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{Delete: &undeletePatch})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to undelete user"))
		}
	}

	if userInfo.HasGroups {
		if err := s.syncUserGroups(ctx, user, workspaceID, userInfo.Groups); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to sync user groups"))
		}
	}
	return user, nil
}

// syncUserGroups syncs the user groups with the given groups.
// The given groups are the groups that the user belongs to in the identity provider.
// Supported groups format: ["group1", "group2", ...], ["dev@bb.com", ...]
func (s *AuthService) syncUserGroups(ctx context.Context, user *store.UserMessage, workspaceID string, groups []string) error {
	bbGroups, err := s.store.ListGroups(ctx, &store.FindGroupMessage{Workspace: workspaceID})
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to list groups"))
	}

	groupEmailMap := make(map[string]*store.GroupMessage)
	groupTitleMap := make(map[string]*store.GroupMessage)
	for _, g := range bbGroups {
		groupEmailMap[g.Email] = g
		groupTitleMap[g.Title] = g
	}

	matchedGroups := make(map[string]bool)
	for _, g := range groups {
		if group, ok := groupEmailMap[g]; ok {
			matchedGroups[group.Email] = true
		} else if group, ok := groupTitleMap[g]; ok {
			matchedGroups[group.Email] = true
		}
	}

	for _, g := range bbGroups {
		isMember := false
		for _, m := range g.Payload.Members {
			if m.Member == common.FormatUserEmail(user.Email) {
				isMember = true
				break
			}
		}

		shouldBeMember := matchedGroups[g.Email]
		if shouldBeMember && !isMember {
			g.Payload.Members = append(g.Payload.Members, &storepb.GroupMember{
				Member: common.FormatUserEmail(user.Email),
				Role:   storepb.GroupMember_MEMBER,
			})
			if _, err := s.store.UpdateGroup(ctx, &store.UpdateGroupMessage{
				ID:        g.ID,
				Workspace: g.Workspace,
				Payload:   g.Payload,
			}); err != nil {
				return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to add user to group %q", g.Email))
			}
		}
	}

	return nil
}
