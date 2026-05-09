package enterprise

import (
	"context"

	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

// FeatureChecker verifies whether a plan feature is enabled.
type FeatureChecker interface {
	IsFeatureEnabled(ctx context.Context, workspaceID string, f v1pb.PlanFeature) error
	IsFeatureEnabledForInstance(ctx context.Context, workspaceID string, f v1pb.PlanFeature, instance *store.InstanceMessage) error
}

// PlanReader retrieves the effective plan and subscription info.
type PlanReader interface {
	GetEffectivePlan(ctx context.Context, workspaceID string) v1pb.PlanType
	LoadSubscription(ctx context.Context, workspaceID string) *v1pb.Subscription
}

// LimitReader retrieves resource limits.
type LimitReader interface {
	GetUserLimit(ctx context.Context, workspaceID string) int
	GetInstanceLimit(ctx context.Context, workspaceID string) int
	GetActivatedInstanceLimit(ctx context.Context, workspaceID string) int
}

// LicenseManager provides full license service operations.
type LicenseManager interface {
	FeatureChecker
	PlanReader
	LimitReader
	StoreLicense(ctx context.Context, workspaceID string, license string) error
	InvalidateCache(workspaceID string)
}

// Compile-time verification.
var _ FeatureChecker = (*LicenseService)(nil)
var _ PlanReader = (*LicenseService)(nil)
var _ LimitReader = (*LicenseService)(nil)
var _ LicenseManager = (*LicenseService)(nil)
