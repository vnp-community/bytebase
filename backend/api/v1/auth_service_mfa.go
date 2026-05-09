package v1

import (
	"context"
	"crypto/subtle"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"github.com/pquerna/otp/totp"

	"github.com/bytebase/bytebase/backend/common/qb"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store"
)

// Password rate limiting configuration.
const (
	// Password phase: 10 failed attempts within 10 minutes.
	passwordMaxFailedAttempts = 10               // Will be used for password rate limiting
	passwordLockoutWindow     = 10 * time.Minute // Will be used for password rate limiting

	// Error messages for password authentication failures.
	errMsgInvalidCredentials = "invalid email or password"
	errMsgTooManyPassword    = "too many failed login attempts, please try again later" // Will be used for password rate limiting
)

var (
	invalidCredentialsError = connect.NewError(connect.CodeUnauthenticated, errors.Errorf(errMsgInvalidCredentials))
)

// countRecentLoginFailures counts the number of failed login attempts for a given email
// within the specified time window, matching any of the provided error messages.
func (s *AuthService) countRecentLoginFailures(ctx context.Context, email string, window time.Duration, errMessages ...string) (int, error) {
	if len(errMessages) == 0 {
		return 0, errors.New("at least one error message is required")
	}

	windowStart := time.Now().Add(-window)

	// Build filter query for login failures.
	filterQ := qb.Q().Space("TRUE")
	filterQ.And("payload->>'method' = ?", "/bytebase.v1.AuthService/Login")
	filterQ.And("payload->>'resource' = ?", email)
	filterQ.And("(payload->'status'->>'code')::int != 0")

	// Build OR condition for error messages.
	if len(errMessages) == 1 {
		filterQ.And("payload->'status'->>'message' = ?", errMessages[0])
	} else {
		// For multiple messages, build: (msg = ? OR msg = ? OR ...)
		orConditions := qb.Q()
		for i, msg := range errMessages {
			if i == 0 {
				orConditions.Space("payload->'status'->>'message' = ?", msg)
			} else {
				orConditions.Or("payload->'status'->>'message' = ?", msg)
			}
		}
		filterQ.And("(?)", orConditions)
	}

	filterQ.And("created_at >= ?", windowStart)

	// Search across all workspaces — lockout is per-email, not per-workspace.
	logs, err := s.store.SearchAuditLogs(ctx, &store.AuditLogFind{
		FilterQ: filterQ,
	})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to search audit logs for login failures")
	}

	return len(logs), nil
}

// checkPasswordLockout checks if the user has exceeded the password failure rate limit.
// Returns an error if the user is locked out due to too many failed password attempts.
func (s *AuthService) checkPasswordLockout(ctx context.Context, email string) error {
	count, err := s.countRecentLoginFailures(ctx, email, passwordLockoutWindow, errMsgInvalidCredentials)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to count recent password failures"))
	}

	if count >= passwordMaxFailedAttempts {
		return connect.NewError(connect.CodeResourceExhausted, errors.Errorf(errMsgTooManyPassword))
	}

	return nil
}

// checkMFALockout checks if the user has exceeded the MFA failure rate limit.
// Returns an error if the user is locked out due to too many failed MFA attempts.
func (s *AuthService) checkMFALockout(ctx context.Context, email string) error {
	count, err := s.countRecentLoginFailures(ctx, email, mfaLockoutWindow, errMsgInvalidMFACode, errMsgInvalidRecoveryCode)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to count recent MFA failures"))
	}

	if count >= mfaMaxFailedAttempts {
		return connect.NewError(connect.CodeResourceExhausted, errors.Errorf(errMsgTooManyMFA))
	}

	return nil
}

func challengeMFACode(user *store.UserMessage, mfaCode string) error {
	if !validateWithCodeAndSecret(mfaCode, user.MFAConfig.OtpSecret) {
		return connect.NewError(connect.CodeUnauthenticated, errors.Errorf(errMsgInvalidMFACode))
	}
	return nil
}

func (s *AuthService) challengeRecoveryCode(ctx context.Context, user *store.UserMessage, recoveryCode string) error {
	for i, code := range user.MFAConfig.RecoveryCodes {
		if subtle.ConstantTimeCompare([]byte(code), []byte(recoveryCode)) == 1 {
			// If the recovery code is valid, delete it from the user's recovery code list.
			user.MFAConfig.RecoveryCodes = slices.Delete(user.MFAConfig.RecoveryCodes, i, i+1)
			_, err := s.store.UpdateUser(ctx, user, &store.UpdateUserMessage{
				MFAConfig: &storepb.MFAConfig{
					OtpSecret:     user.MFAConfig.OtpSecret,
					RecoveryCodes: user.MFAConfig.RecoveryCodes,
				},
			})
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to update user"))
			}
			return nil
		}
	}
	return connect.NewError(connect.CodeUnauthenticated, errors.Errorf(errMsgInvalidRecoveryCode))
}

// validateWithCodeAndSecret validates the given code against the given secret.
func validateWithCodeAndSecret(code, secret string) bool {
	return totp.Validate(code, secret)
}
