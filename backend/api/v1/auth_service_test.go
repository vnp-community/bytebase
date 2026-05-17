package v1

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// =============================================================================
// Signup Input Validation Tests
// Tests for the guard clauses in Signup that run before any store interaction.
// =============================================================================

func TestSignup_EmptyEmail(t *testing.T) {
	t.Parallel()
	svc := &AuthService{}
	_, err := svc.Signup(context.Background(), connect.NewRequest(&v1pb.SignupRequest{
		Email: "", Title: "Test User", Password: "strongpassword",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email must be set")
}

func TestSignup_EmptyTitle(t *testing.T) {
	t.Parallel()
	svc := &AuthService{}
	_, err := svc.Signup(context.Background(), connect.NewRequest(&v1pb.SignupRequest{
		Email: "test@example.com", Title: "", Password: "strongpassword",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title must be set")
}

func TestSignup_EmptyPassword(t *testing.T) {
	t.Parallel()
	svc := &AuthService{}
	_, err := svc.Signup(context.Background(), connect.NewRequest(&v1pb.SignupRequest{
		Email: "test@example.com", Title: "Test User", Password: "",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password must be set")
}

func TestSignup_ServiceAccountEmailRejected(t *testing.T) {
	t.Parallel()
	// SA email suffix is "service.bytebase.com"
	svc := &AuthService{}
	_, err := svc.Signup(context.Background(), connect.NewRequest(&v1pb.SignupRequest{
		Email: "sa-test@service.bytebase.com", Title: "Test", Password: "strongpassword",
	}))
	require.Error(t, err, "service account email should be rejected")
	assert.Contains(t, err.Error(), "end users")
}

// =============================================================================
// Email Validation Helper Tests
// =============================================================================

func TestValidateEndUserEmail_ServiceAccount(t *testing.T) {
	t.Parallel()
	err := validateEndUserEmail("sa-test@service.bytebase.com")
	require.Error(t, err, "service account email should be rejected")
}

func TestValidateEndUserEmail_ValidEmail(t *testing.T) {
	t.Parallel()
	err := validateEndUserEmail("user@example.com")
	require.NoError(t, err, "valid email should pass")
}

func TestValidateEndUserEmail_EmptyEmail(t *testing.T) {
	t.Parallel()
	// validateEndUserEmail does NOT reject empty — that's handled by Signup guard
	err := validateEndUserEmail("")
	require.NoError(t, err, "empty email is not rejected by this validator")
}

// =============================================================================
// Password Validation Tests
// =============================================================================

func TestValidatePassword_TooShort(t *testing.T) {
	t.Parallel()
	restriction := &storepb.WorkspaceProfileSetting_PasswordRestriction{MinLength: 8}
	err := validatePasswordWithRestriction("short", restriction)
	require.Error(t, err, "password shorter than min length should be rejected")
}

func TestValidatePassword_MeetsMinLength(t *testing.T) {
	t.Parallel()
	restriction := &storepb.WorkspaceProfileSetting_PasswordRestriction{MinLength: 8}
	err := validatePasswordWithRestriction("longpassword", restriction)
	require.NoError(t, err, "password meeting min length should pass")
}

func TestValidatePassword_RequiresNumber(t *testing.T) {
	t.Parallel()
	restriction := &storepb.WorkspaceProfileSetting_PasswordRestriction{MinLength: 8, RequireNumber: true}

	err := validatePasswordWithRestriction("nonnumber", restriction)
	require.Error(t, err, "password without number should be rejected")

	err = validatePasswordWithRestriction("pass1word", restriction)
	require.NoError(t, err, "password with number should pass")
}

func TestValidatePassword_RequiresUppercase(t *testing.T) {
	t.Parallel()
	restriction := &storepb.WorkspaceProfileSetting_PasswordRestriction{
		MinLength: 8, RequireUppercaseLetter: true,
	}

	err := validatePasswordWithRestriction("alllowercase", restriction)
	require.Error(t, err, "password without uppercase should be rejected")

	err = validatePasswordWithRestriction("hasUppercase", restriction)
	require.NoError(t, err, "password with uppercase should pass")
}

func TestValidatePassword_RequiresSpecialChar(t *testing.T) {
	t.Parallel()
	restriction := &storepb.WorkspaceProfileSetting_PasswordRestriction{
		MinLength: 8, RequireSpecialCharacter: true,
	}

	err := validatePasswordWithRestriction("nothingspecial1A", restriction)
	require.Error(t, err, "password without special char should be rejected")

	err = validatePasswordWithRestriction("pass!word", restriction)
	require.NoError(t, err, "password with special char should pass")
}

// =============================================================================
// Workspace Helper Tests
// =============================================================================

func TestParseOptionalWorkspace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		workspace *string
		want      string
		wantErr   bool
	}{
		{"nil workspace", nil, "", false},
		{"empty workspace", strPtr(""), "", false},
		{"valid workspace", strPtr("workspaces/ws-1"), "ws-1", false},
		{"invalid format", strPtr("invalid"), "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseOptionalWorkspace(tc.workspace)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// =============================================================================
// Login Permission Validation Tests
// =============================================================================

func TestValidateLoginPermissions_DeactivatedUser(t *testing.T) {
	t.Parallel()
	svc := &AuthService{}
	user := &store.UserMessage{MemberDeleted: true, Email: "deleted@example.com"}
	err := svc.validateLoginPermissions(context.Background(), user, "ws-1", &v1pb.LoginRequest{})
	require.Error(t, err, "deactivated user should be rejected")
	assert.Contains(t, err.Error(), "deactivated")
}

// =============================================================================
// Domain Extraction Tests
// =============================================================================

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   string
	}{
		{"www.google.com", "google.com"},
		{"code.google.com", "google.com"},
		{"code.google.com.cn", "google.com.cn"},
		{"google.com", "google.com"},
	}
	for _, test := range tests {
		got := extractDomain(test.domain)
		if got != test.want {
			t.Errorf("extractDomain %s, got %s, want %s", test.domain, got, test.want)
		}
	}
}

func strPtr(s string) *string { return &s }
