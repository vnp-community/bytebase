package v1

import (
	"context"
	"log/slog"
	"os"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// parseOptionalWorkspace extracts the workspace ID from an optional "workspaces/{id}"
// resource name. Returns empty when the caller has no workspace context yet.
func parseOptionalWorkspace(name *string) (string, error) {
	if name == nil || *name == "" {
		return "", nil
	}
	return common.GetWorkspaceID(*name)
}

// resolveWorkspaceForLogin determines the workspace for a login token.
// For SA/WI: looks up the account record to get workspace.
// For END_USER: resolution order:
//  1. preferredWorkspaceID (from the login request's ?workspace= hint, e.g. invite links)
//  2. Last login workspace (from user profile)
//  3. First workspace from IAM membership
//
// Each candidate is validated for membership before use.
func (s *AuthService) resolveWorkspaceForLogin(ctx context.Context, user *store.UserMessage, preferredWorkspaceID string) (string, error) {
	// Determine member name format based on user type.
	switch user.Type {
	case storepb.PrincipalType_SERVICE_ACCOUNT:
		// SA has workspace on its record — look it up directly.
		sa, err := s.store.GetServiceAccountByEmail(ctx, user.Email)
		if err != nil {
			return "", errors.Wrap(err, "failed to get service account")
		}
		if sa != nil {
			return sa.Workspace, nil
		}
		return "", errors.Errorf("service account %q not found", user.Email)
	case storepb.PrincipalType_END_USER:
		includeAllUser := !s.profile.SaaS

		// Prefer the workspace from the login request hint (e.g. invite link).
		if preferredWorkspaceID != "" {
			ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
				WorkspaceID:    &preferredWorkspaceID,
				Email:          user.Email,
				IncludeAllUser: includeAllUser,
			})
			if err != nil {
				return "", errors.Wrap(err, "failed to find workspace")
			}
			if ws != nil {
				return ws.ResourceID, nil
			}
			// Not a member of preferred workspace — fall through.
		}

		// Prefer the last login workspace if it's still valid.
		if lastWS := user.Profile.GetLastLoginWorkspace(); lastWS != "" {
			ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
				WorkspaceID:    &lastWS,
				Email:          user.Email,
				IncludeAllUser: includeAllUser,
			})
			if err != nil {
				return "", errors.Wrap(err, "failed to find workspace")
			}
			if ws != nil {
				return ws.ResourceID, nil
			}
			// Last login workspace no longer valid — fall through to default.
		}

		// Use the first workspace the user is a member of.
		ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
			Email:          user.Email,
			IncludeAllUser: includeAllUser,
		})
		if err != nil {
			return "", errors.Wrap(err, "failed to find workspace")
		}
		if ws == nil {
			return "", errors.Errorf("%q is not a member of any workspace", user.Email)
		}
		return ws.ResourceID, nil
	default:
		return "", errors.Errorf("unsupported user type %s for login", user.Type)
	}
}

// resolveWorkspaceIDByEmail determines which workspace a signing-up email would
// land in WITHOUT mutating anything. Used by signup/signup-via-code to look up the
// applicable workspace restriction before creating a user or provisioning workspaces, so
// a rejected signup doesn't leave orphan state behind. Returns empty for SaaS brand-new
// signup (no pre-invite, no workspace) — the caller should apply default restriction.
// resolveWorkspaceIDByEmail returns (workspaceID, isMember).
// isMember is true when the user already has an IAM binding in the returned workspace.
// When false, the returned workspaceID is the self-host singleton (user needs to be added).
func (s *AuthService) resolveWorkspaceIDByEmail(ctx context.Context, email string) (string, bool, error) {
	existingWS, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
		Email:          email,
		IncludeAllUser: !s.profile.SaaS,
	})
	if err != nil {
		return "", false, errors.Wrapf(err, "failed to find workspaces")
	}
	if existingWS != nil {
		return existingWS.ResourceID, true, nil
	}
	if !s.profile.SaaS {
		singletonID, err := s.store.GetWorkspaceID(ctx)
		if err != nil {
			return "", false, errors.Wrapf(err, "failed to resolve singleton workspace")
		}
		return singletonID, false, nil
	}
	return "", false, nil
}

// provisionWorkspaceForNewUser returns a workspace ID for a freshly-created user.
// If the email was pre-invited to existing workspaces (via IAM), returns the first one.
// Otherwise creates a new workspace (SaaS: per-user; self-hosted: joins the singleton).
// Called by both the Signup RPC and the email-code signup branch of Login.
func (s *AuthService) provisionWorkspaceForNewUser(ctx context.Context, email string) (string, error) {
	// Step 1: Resolve the target workspace. isMember indicates whether the user already has
	// an IAM binding. For pre-invited users we must NOT patch IAM — PatchWorkspaceIamPolicy
	// is a set-replacement that would downgrade an admin to member.
	workspaceID, isMember, err := s.resolveWorkspaceIDByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	if workspaceID != "" {
		if !s.profile.SaaS && !isMember {
			// Self-hosted, new user joining the singleton workspace — add as member.
			if _, err := s.store.PatchWorkspaceIamPolicy(ctx, &store.PatchIamPolicyMessage{
				Workspace: workspaceID,
				Member:    common.FormatUserEmail(email),
				Roles:     []string{common.FormatRole(store.WorkspaceMemberRole)},
			}); err != nil {
				return "", errors.Wrapf(err, "failed to add user to workspace")
			}
		}
		return workspaceID, nil
	}

	// Step 2: No existing workspace — create a new one with the user as admin.
	wsID, err := common.RandomString(16)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate workspace ID")
	}
	ws, err := s.store.CreateWorkspace(ctx, &store.WorkspaceMessage{
		ResourceID:         wsID,
		Payload:            &storepb.WorkspacePayload{Title: "Default workspace"},
		AdditionalSettings: s.getAdditionalWorkspaceSettings(),
	}, email)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create workspace")
	}

	return ws.ResourceID, nil
}

func getAccountRestriction(
	ctx context.Context,
	stores *store.Store,
	licenseService *enterprise.LicenseService,
	saas bool,
	workspaceID string,
) (*v1pb.Restriction, error) {
	defaultPasswordRestriction := &v1pb.WorkspaceProfileSetting_PasswordRestriction{
		MinLength: 8,
	}
	restriction := &v1pb.Restriction{
		DisallowSignup:         false,
		DisallowPasswordSignin: false,
		AllowEmailCodeSignin:   false,
		PasswordResetEnabled:   false,
		PasswordRestriction:    defaultPasswordRestriction,
	}

	emailSetting, err := resolvePreLoginEmailSetting(ctx, stores, workspaceID)
	if err != nil {
		return nil, err
	}

	if workspaceID != "" {
		setting, err := stores.GetWorkspaceProfileSetting(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find profile setting for workspace %v", workspaceID))
		}

		restriction = &v1pb.Restriction{
			PasswordRestriction:    convertToV1PasswordRestriction(setting.GetPasswordRestriction()),
			DisallowSignup:         setting.DisallowSignup,
			DisallowPasswordSignin: setting.DisallowPasswordSignin,
			AllowEmailCodeSignin:   setting.AllowEmailCodeSignin,
		}

		// Override if features are not enabled
		if licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_DISALLOW_SELF_SERVICE_SIGNUP) != nil {
			restriction.DisallowSignup = false
		}
		if licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_DISALLOW_PASSWORD_SIGNIN) != nil {
			restriction.DisallowPasswordSignin = false
		}
		if licenseService.IsFeatureEnabled(ctx, workspaceID, v1pb.PlanFeature_FEATURE_PASSWORD_RESTRICTIONS) != nil {
			restriction.PasswordRestriction = defaultPasswordRestriction
		}
	}

	// Override for SaaS
	if saas {
		restriction.DisallowSignup = true
		restriction.DisallowPasswordSignin = true
		restriction.AllowEmailCodeSignin = true
	}

	if !restriction.DisallowPasswordSignin {
		restriction.PasswordResetEnabled = emailSetting != nil
	}
	if emailSetting == nil {
		restriction.AllowEmailCodeSignin = false
	}

	return restriction, nil
}

// getAdditionalWorkspaceSettings returns extra settings to inject during workspace creation.
// In SaaS mode with Gemini API key configured, injects AI settings.
func (*AuthService) getAdditionalWorkspaceSettings() []store.AdditionalSetting {
	var settings []store.AdditionalSetting
	if geminiAPIKey := os.Getenv("GEMINI_API_KEY"); geminiAPIKey != "" {
		settings = append(settings, store.AdditionalSetting{
			Name: storepb.SettingName_AI,
			Payload: &storepb.AISetting{
				Enabled:  true,
				Provider: storepb.AISetting_GEMINI,
				ApiKey:   geminiAPIKey,
				Endpoint: "https://generativelanguage.googleapis.com/v1beta",
				Model:    "gemini-2.5-pro",
			},
		})
	}
	if raw := os.Getenv("EMAIL_CONFIG"); raw != "" { //nolint:nestif
		emailSetting := &storepb.EmailSetting{}
		if err := common.ProtojsonUnmarshaler.Unmarshal([]byte(raw), emailSetting); err != nil {
			slog.Error("failed to parse EMAIL_CONFIG env var", log.BBError(err))
		} else if err := validateEmailSetting(emailSetting); err != nil {
			slog.Error("invalid EMAIL_CONFIG env var", log.BBError(err))
		} else {
			settings = append(settings, store.AdditionalSetting{
				Name:    storepb.SettingName_EMAIL,
				Payload: emailSetting,
			})
		}
	}
	return settings
}
