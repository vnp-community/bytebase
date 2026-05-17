// Package mock provides mock implementations for enterprise interfaces.
//
// To regenerate mocks after interface changes, run from the project root:
//
//	go generate ./backend/enterprise/mock/...
//
// Prerequisites:
//
//	go install go.uber.org/mock/mockgen@latest
package mock

//go:generate mockgen -package mock -destination mock_enterprise.go github.com/bytebase/bytebase/backend/enterprise FeatureChecker,PlanReader,LimitReader,LicenseManager
