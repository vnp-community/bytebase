// Package mock provides mock implementations for IAM interfaces.
//
// To regenerate mocks after interface changes, run from the project root:
//
//	go generate ./backend/component/iam/mock/...
//
// Prerequisites:
//
//	go install go.uber.org/mock/mockgen@latest
package mock

//go:generate mockgen -package mock -destination mock_iam.go github.com/bytebase/bytebase/backend/component/iam PermissionChecker,PermissionProvider,GroupResolver,CacheReloader,IAMService
