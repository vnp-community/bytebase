package v1

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/mailer"
	"github.com/bytebase/bytebase/backend/store"
)

// Email code constants.
const (
	emailCodeLength         = 6
	emailCodeExpiry         = 10 * time.Minute
	emailCodeMaxAttempts    = 5
	emailCodeResendCooldown = 60 * time.Second
)

// RequestPasswordReset sends a password reset email. Always returns success to avoid leaking email existence.
func (s *AuthService) RequestPasswordReset(ctx context.Context, req *connect.Request[v1pb.RequestPasswordResetRequest]) (*connect.Response[emptypb.Empty], error) {
	email := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email is required"))
	}

	// Send synchronously, but swallow errors to avoid email enumeration — a fast silent
	// "success" for unknown emails must be indistinguishable from an SMTP failure for
	// known ones. Errors are logged server-side for operator visibility.
	if err := s.sendEmailVerificationCode(
		ctx,
		req.Msg.Workspace,
		email,
		storepb.EmailVerificationCodePurpose_PASSWORD_RESET,
		"[Bytebase] Reset your password",
		"Hi,\n\nYour password reset code is: %s\n\nThis code expires in %d minutes. If you didn't request this, you can safely ignore this email.\n\n— Bytebase",
	); err != nil {
		slog.Warn("failed to send password reset email", slog.String("to", email), log.BBError(err))
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// ResetPassword verifies the 6-digit code and updates the user's password.
// Also revokes all refresh tokens to force re-login.
func (s *AuthService) ResetPassword(ctx context.Context, req *connect.Request[v1pb.ResetPasswordRequest]) (*connect.Response[emptypb.Empty], error) {
	email := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email is required"))
	}
	if req.Msg.Code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("code is required"))
	}
	if req.Msg.NewPassword == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("new_password is required"))
	}

	codeRow, err := s.verifyEmailCode(ctx, email, storepb.EmailVerificationCodePurpose_PASSWORD_RESET, req.Msg.Code)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find user"))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("user not found"))
	}

	// Validate the user is a member of the workspace captured at send time.
	// Reject if forged — prevents bypassing password policy via a weaker workspace.
	if codeRow.Workspace != "" {
		ws, err := s.store.FindWorkspace(ctx, &store.FindWorkspaceMessage{
			WorkspaceID:    &codeRow.Workspace,
			Email:          email,
			IncludeAllUser: !s.profile.SaaS,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to verify workspace membership"))
		}
		if ws == nil {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("user is not a member of the workspace"))
		}
	}
	restriction, err := getAccountRestriction(ctx, s.store, s.licenseService, s.profile.SaaS, codeRow.Workspace)
	if err != nil {
		return nil, err
	}
	if err := validatePasswordWithRestriction(req.Msg.NewPassword, convertToStorePasswordRestriction(restriction.PasswordRestriction)); err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Msg.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to hash password"))
	}
	passwordHashStr := string(passwordHash)
	if _, err := s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{
		Email:        &user.Email,
		PasswordHash: &passwordHashStr,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to update password"))
	}

	if err := s.store.DeleteWebRefreshTokensByUser(ctx, user.Email); err != nil {
		slog.Warn("failed to revoke refresh tokens after password reset", log.BBError(err))
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// SendEmailLoginCode sends a 6-digit verification code. Always returns success
// (no email enumeration). Rate limit: 60-sec resend cooldown enforced atomically
// via the store — effective cap ≈ 60 sends/hour/email.
func (s *AuthService) SendEmailLoginCode(ctx context.Context, req *connect.Request[v1pb.SendEmailLoginCodeRequest]) (*connect.Response[emptypb.Empty], error) {
	email := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email is required"))
	}
	workspaceID, err := parseOptionalWorkspace(req.Msg.Workspace)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Gate on AllowEmailCodeSignin — no point emailing a code the workspace won't accept.
	restriction, err := getAccountRestriction(ctx, s.store, s.licenseService, s.profile.SaaS, workspaceID)
	if err != nil {
		return nil, err
	}
	if !restriction.AllowEmailCodeSignin {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.Errorf("email code login is not enabled for this workspace"))
	}

	// Send synchronously so the caller learns about actionable failures.
	if err := s.sendEmailVerificationCode(
		ctx,
		req.Msg.Workspace,
		email,
		storepb.EmailVerificationCodePurpose_LOGIN,
		"[Bytebase] Your sign-in code",
		"Hi,\n\nYour sign-in code is: %s\n\nThis code expires in %d minutes. If you didn't request this, you can safely ignore this email.\n\n— Bytebase",
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// hashEmailCode returns HMAC-SHA256(code) hex-encoded, keyed with the server's auth secret.
func (s *AuthService) hashEmailCode(code string) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// generateEmailCode returns a cryptographically-random 6-digit numeric code.
func generateEmailCode() (string, error) {
	const digits = "0123456789"
	b := make([]byte, emailCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = digits[int(b[i])%len(digits)]
	}
	return string(b), nil
}

// sendEmailVerificationCode generates a code, atomically stores its hash (subject to cooldown),
// and emails the plain code.
func (s *AuthService) sendEmailVerificationCode(ctx context.Context, workspaceName *string, email string, purpose storepb.EmailVerificationCodePurpose, subject, bodyFmt string) error {
	// For password reset, only send to existing principals — no upsert, no email for unknown addresses.
	if purpose == storepb.EmailVerificationCodePurpose_PASSWORD_RESET {
		account, err := s.store.GetAccountByEmail(ctx, email)
		if err != nil {
			return errors.Wrap(err, "failed to look up account for password reset")
		}
		if account == nil || account.Type != storepb.PrincipalType_END_USER {
			return nil // silent: account doesn't exist
		}
	}

	workspaceID, err := parseOptionalWorkspace(workspaceName)
	if err != nil {
		return errors.Wrap(err, "failed to parse workspace id")
	}

	// Resolve the EMAIL setting FIRST — fail fast if misconfigured.
	emailSetting, err := resolvePreLoginEmailSetting(ctx, s.store, workspaceID)
	if err != nil {
		return err
	}
	if emailSetting == nil {
		return errors.Errorf("cannot found email config for workspace %v", workspaceID)
	}

	code, err := generateEmailCode()
	if err != nil {
		return errors.Wrap(err, "failed to generate code")
	}

	now := time.Now()
	sent, err := s.store.UpsertEmailVerificationCodeIfCooldownExpired(ctx, &store.EmailVerificationCodeMessage{
		Email:      email,
		Purpose:    purpose,
		CodeHash:   s.hashEmailCode(code),
		ExpiresAt:  now.Add(emailCodeExpiry),
		LastSentAt: now,
		Workspace:  workspaceID,
	}, emailCodeResendCooldown)
	if err != nil {
		return errors.Wrap(err, "failed to upsert verification code")
	}
	if !sent {
		return nil // cooldown active — silent skip
	}

	sender, err := mailer.NewSender(emailSetting)
	if err != nil {
		return errors.Wrap(err, "failed to create mail sender")
	}

	body := fmt.Sprintf(bodyFmt, code, int(emailCodeExpiry.Minutes()))
	if err := sender.Send(ctx, &mailer.SendRequest{
		To:       []string{email},
		Subject:  subject,
		TextBody: body,
	}); err != nil {
		// Delete the row so the cooldown doesn't block an immediate retry.
		_ = s.store.DeleteEmailVerificationCodeIfMatch(ctx, email, purpose, s.hashEmailCode(code))
		return errors.Wrap(err, "failed to send email")
	}
	return nil
}

// verifyEmailCode checks a submitted code against the stored row.
// Enforces expiry, attempt limit (5), and constant-time hash compare.
func (s *AuthService) verifyEmailCode(ctx context.Context, email string, purpose storepb.EmailVerificationCodePurpose, submittedCode string) (*store.EmailVerificationCodeMessage, error) {
	row, err := s.store.GetEmailVerificationCode(ctx, email, purpose)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get email verification code"))
	}
	if row == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("invalid or expired code"))
	}
	if time.Now().After(row.ExpiresAt) {
		_ = s.store.DeleteEmailVerificationCodeIfMatch(ctx, email, purpose, row.CodeHash)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("invalid or expired code"))
	}
	if row.Attempts >= emailCodeMaxAttempts {
		_ = s.store.DeleteEmailVerificationCodeIfMatch(ctx, email, purpose, row.CodeHash)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("too many attempts"))
	}
	if subtle.ConstantTimeCompare([]byte(s.hashEmailCode(submittedCode)), []byte(row.CodeHash)) != 1 {
		_ = s.store.IncrementEmailVerificationCodeAttempts(ctx, email, purpose)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.Errorf("invalid or expired code"))
	}
	_ = s.store.DeleteEmailVerificationCodeIfMatch(ctx, email, purpose, row.CodeHash)
	return row, nil
}

// authenticateEmailCodeLogin handles the email + 6-digit code flow.
func (s *AuthService) authenticateEmailCodeLogin(ctx context.Context, request *v1pb.LoginRequest) (*store.UserMessage, error) {
	if request.Password != "" || request.GetIdpName() != "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email_code is mutually exclusive with password and idp_name"))
	}
	email := strings.ToLower(strings.TrimSpace(request.Email))
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf("email is required"))
	}

	codeRow, err := s.verifyEmailCode(ctx, email, storepb.EmailVerificationCodePurpose_LOGIN, *request.EmailCode)
	if err != nil {
		return nil, err
	}

	// Existing user → return.
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to find user"))
	}
	if user != nil {
		return user, nil
	}

	// Unknown email → signup path.
	if err := validateEndUserEmail(email); err != nil {
		return nil, err
	}

	restriction, err := getAccountRestriction(ctx, s.store, s.licenseService, s.profile.SaaS, codeRow.Workspace)
	if err != nil {
		return nil, err
	}
	if !restriction.AllowEmailCodeSignin {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.Errorf("email code login is not enabled for this workspace"))
	}
	if !s.profile.SaaS {
		if restriction.DisallowSignup {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("sign up is disallowed for this workspace"))
		}
	}

	if _, err := s.provisionWorkspaceForNewUser(ctx, email); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to provision workspace"))
	}

	// Create principal with random bcrypt password.
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to generate random password"))
	}
	passwordHash, err := bcrypt.GenerateFromPassword(randomBytes, bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to hash password"))
	}

	// Derive display name from email local-part.
	name := email
	if i := strings.Index(email, "@"); i > 0 {
		name = email[:i]
	}

	newUser, err := s.store.CreateUser(ctx, &store.UserMessage{
		Email:        email,
		Name:         name,
		Type:         storepb.PrincipalType_END_USER,
		PasswordHash: string(passwordHash),
		Profile:      &storepb.UserProfile{},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to create user"))
	}

	return newUser, nil
}

// resolvePreLoginEmailSetting returns the EMAIL setting to use for unauthenticated flows.
func resolvePreLoginEmailSetting(
	ctx context.Context,
	stores *store.Store,
	workspaceID string,
) (*storepb.EmailSetting, error) {
	if workspaceID != "" {
		emailSettingMsg, err := stores.GetSetting(ctx, workspaceID, storepb.SettingName_EMAIL)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load email setting")
		}
		if emailSettingMsg == nil {
			return nil, nil
		}
		es, ok := emailSettingMsg.Value.(*storepb.EmailSetting)
		if !ok {
			return nil, nil
		}
		return es, nil
	}

	if raw := os.Getenv("EMAIL_CONFIG"); raw != "" {
		emailSetting := &storepb.EmailSetting{}
		if err := common.ProtojsonUnmarshaler.Unmarshal([]byte(raw), emailSetting); err != nil {
			return nil, errors.Wrap(err, "failed to parse EMAIL_CONFIG")
		}
		return emailSetting, nil
	}

	return nil, nil
}
