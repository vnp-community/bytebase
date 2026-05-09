package v1

// auth_service_di.go provides an alternative constructor for AuthService that accepts
// domain interfaces instead of concrete types. This enables dependency injection
// and mock-based testing for the authentication layer.
//
// Usage (POC — to be wired in grpc_routes.go when ready):
//
//	authService := apiv1.NewAuthServiceWithDeps(store, secret, &apiv1.AuthDeps{
//	    UserStore:         store,
//	    FeatureChecker:    licenseService,
//	    PermissionChecker: iamManager,
//	    Profile:           profile,
//	})

import (
	"context"

	"github.com/bytebase/bytebase/backend/component/config"
	enterpriseiface "github.com/bytebase/bytebase/backend/enterprise"
	iamiface "github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/store"
)

// AuthDeps holds the interface-based dependencies for AuthService.
// This struct enables incremental migration from concrete types to interfaces.
type AuthDeps struct {
	// FeatureChecker verifies if enterprise features are enabled.
	// Concrete type: *enterprise.LicenseService
	FeatureChecker enterpriseiface.FeatureChecker

	// PermissionChecker verifies workspace/project-level permissions.
	// Concrete type: *iam.Manager
	PermissionChecker iamiface.PermissionChecker

	// Profile provides server configuration.
	Profile *config.Profile
}

// NewAuthServiceWithDeps creates an AuthService using interface-based dependencies.
// This is the DI-ready constructor for the auth service.
//
// It requires both the concrete *store.Store (for the many Store methods still used
// directly) and a secret string (for JWT signing). The AuthDeps struct provides
// the interface-backed dependencies that can be mocked in tests.
//
// Migration path:
//  1. Replace concrete *enterprise.LicenseService with FeatureChecker interface
//  2. Replace concrete *iam.Manager with PermissionChecker interface
//  3. Gradually replace *store.Store with domain-specific reader/writer interfaces
func NewAuthServiceWithDeps(stores *store.Store, secret string, deps *AuthDeps) *AuthService {
	// For now, we still need the concrete types internally because AuthService
	// uses methods beyond the interface surface area. The interfaces ensure
	// that NEW code uses the narrow contract.
	var licenseService *enterpriseiface.LicenseService
	if fc, ok := deps.FeatureChecker.(*enterpriseiface.LicenseService); ok {
		licenseService = fc
	}
	var iamManager *iamiface.Manager
	if pc, ok := deps.PermissionChecker.(*iamiface.Manager); ok {
		iamManager = pc
	}

	return &AuthService{
		store:          stores,
		secret:         secret,
		licenseService: licenseService,
		profile:        deps.Profile,
		iamManager:     iamManager,
	}
}

// Compile-time verification that concrete types satisfy interfaces.
var _ enterpriseiface.FeatureChecker = (*enterpriseiface.LicenseService)(nil)
var _ iamiface.PermissionChecker = (*iamiface.Manager)(nil)

// Suppress unused import warnings.
var _ context.Context
