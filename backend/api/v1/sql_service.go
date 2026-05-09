package v1

import (
	"context"

	"connectrpc.com/connect"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/component/dbfactory"
	"github.com/bytebase/bytebase/backend/component/iam"
	"github.com/bytebase/bytebase/backend/enterprise"
	"github.com/bytebase/bytebase/backend/generated-go/v1/v1connect"
	"github.com/bytebase/bytebase/backend/runner/schemasync"
	"github.com/bytebase/bytebase/backend/store"
)

// SQLService is the service for SQL.
type SQLService struct {
	v1connect.UnimplementedSQLServiceHandler
	store          *store.Store
	schemaSyncer   *schemasync.Syncer
	dbFactory      *dbfactory.DBFactory
	licenseService *enterprise.LicenseService
	iamManager     *iam.Manager
}

// NewSQLService creates a SQLService.
func NewSQLService(
	store *store.Store,
	schemaSyncer *schemasync.Syncer,
	dbFactory *dbfactory.DBFactory,
	licenseService *enterprise.LicenseService,
	iamManager *iam.Manager,
) *SQLService {
	return &SQLService{
		store:          store,
		schemaSyncer:   schemaSyncer,
		dbFactory:      dbFactory,
		licenseService: licenseService,
		iamManager:     iamManager,
	}
}

func (*SQLService) getUser(ctx context.Context) (*store.UserMessage, error) {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.Errorf("user not found"))
	}
	if user.MemberDeleted {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.Errorf("the user has been deactivated"))
	}
	return user, nil
}
