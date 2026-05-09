package iam

import (
	"context"

	"github.com/bytebase/bytebase/backend/common/permission"
	"github.com/bytebase/bytebase/backend/store"
)

// PermissionChecker provides permission verification.
type PermissionChecker interface {
	CheckPermission(ctx context.Context, p permission.Permission, user *store.UserMessage, workspaceID string, projectIDs ...string) (bool, error)
}

// PermissionProvider retrieves role permissions.
type PermissionProvider interface {
	GetPermissions(ctx context.Context, workspaceID string, roleName string) (map[permission.Permission]bool, error)
}

// GroupResolver looks up user group memberships.
type GroupResolver interface {
	GetUserGroups(ctx context.Context, workspaceID string, email string) ([]string, error)
}

// CacheReloader reloads the IAM cache.
type CacheReloader interface {
	ReloadCache(ctx context.Context) error
}

// IAMService is the aggregate interface for IAM operations.
// Services should depend on the narrowest sub-interface they need.
type IAMService interface {
	PermissionChecker
	PermissionProvider
	GroupResolver
	CacheReloader
}

// Compile-time verification that *Manager satisfies all interfaces.
var _ PermissionChecker = (*Manager)(nil)
var _ PermissionProvider = (*Manager)(nil)
var _ GroupResolver = (*Manager)(nil)
var _ CacheReloader = (*Manager)(nil)
var _ IAMService = (*Manager)(nil)
