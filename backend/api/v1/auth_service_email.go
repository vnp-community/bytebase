package v1

// auth_service_email.go contains email-code authentication and password reset methods.
// Extracted from auth_service.go to reduce file size.
//
// This file serves as the target location for email/password methods.
// During incremental migration, methods from auth_service.go will be moved here.
// Current state: template with correct package and struct alignment.
//
// Methods to migrate (from auth_service.go):
//   - hashEmailCode              (line ~1619)
//   - generateEmailCode          (line ~1603)  [package-level]
//   - SendEmailLoginCode         (line ~1628)
//   - sendEmailVerificationCode  (line ~1713)
//   - verifyEmailCode            (line ~1786)
//   - authenticateEmailCodeLogin (line ~1813)
//   - RequestPasswordReset       (line ~1500)
//   - ResetPassword              (line ~1525)
//   - resolvePreLoginEmailSetting (line ~1674) [package-level]
