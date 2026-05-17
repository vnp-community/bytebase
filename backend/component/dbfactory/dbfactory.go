// Package dbfactory includes the database driver factory.
package dbfactory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/circuitbreaker"
	secretlib "github.com/bytebase/bytebase/backend/component/secret"
	"github.com/bytebase/bytebase/backend/enterprise"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/db"
	dbutil "github.com/bytebase/bytebase/backend/plugin/db/util"
	"github.com/bytebase/bytebase/backend/store"
	"github.com/bytebase/bytebase/backend/utils"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DBFactory is the factory for building database driver.
type DBFactory struct {
	store          *store.Store
	licenseService *enterprise.LicenseService

	breakers   map[string]*circuitbreaker.CircuitBreaker
	breakersMu sync.RWMutex
	registry   prometheus.Registerer
}

// New creates a new database driver factory.
func New(store *store.Store, licenseService *enterprise.LicenseService, registry prometheus.Registerer) *DBFactory {
	return &DBFactory{
		store:          store,
		licenseService: licenseService,
		breakers:       make(map[string]*circuitbreaker.CircuitBreaker),
		registry:       registry,
	}
}

// GetAdminDatabaseDriver gets the admin database driver using the instance's admin data source.
// Upon successful return, caller must call driver.Close(). Otherwise, it will leak the database connection.
func (d *DBFactory) GetAdminDatabaseDriver(ctx context.Context, instance *store.InstanceMessage, database *store.DatabaseMessage, connectionContext db.ConnectionContext) (db.Driver, error) {
	dataSource := utils.DataSourceFromInstanceWithType(instance, storepb.DataSourceType_ADMIN)
	if dataSource == nil {
		return nil, common.Errorf(common.Internal, "admin data source not found for instance %q", instance.ResourceID)
	}
	if database != nil {
		connectionContext.DatabaseName = database.DatabaseName
	}
	return d.GetDataSourceDriver(ctx, instance, dataSource, connectionContext)
}

// getOrCreateBreaker gets or creates a circuit breaker for an instance.
func (d *DBFactory) getOrCreateBreaker(instanceID string) *circuitbreaker.CircuitBreaker {
	d.breakersMu.RLock()
	cb, ok := d.breakers[instanceID]
	d.breakersMu.RUnlock()
	if ok {
		return cb
	}

	d.breakersMu.Lock()
	defer d.breakersMu.Unlock()
	
	// Double-checked locking
	if cb, ok := d.breakers[instanceID]; ok {
		return cb
	}

	cb = circuitbreaker.New(circuitbreaker.Config{
		Name:             "db_" + instanceID,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}, d.registry)
	d.breakers[instanceID] = cb
	return cb
}

// GetDataSourceDriver returns the database driver for a data source with circuit breaking.
func (d *DBFactory) GetDataSourceDriver(ctx context.Context, instance *store.InstanceMessage, dataSource *storepb.DataSource, connectionContext db.ConnectionContext) (db.Driver, error) {
	breaker := d.getOrCreateBreaker(instance.ResourceID)
	var driver db.Driver

	err := breaker.Execute(ctx, func(ctx context.Context) error {
		var err error
		driver, err = d.getDataSourceDriverInternal(ctx, instance, dataSource, connectionContext)
		return err
	})

	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		return nil, status.Errorf(codes.Unavailable, "instance %s unreachable (circuit breaker open)", instance.ResourceID)
	}
	return driver, err
}

func (d *DBFactory) getDataSourceDriverInternal(ctx context.Context, instance *store.InstanceMessage, dataSource *storepb.DataSource, connectionContext db.ConnectionContext) (db.Driver, error) {
	password := dataSource.GetPassword()
	if err := d.licenseService.IsFeatureEnabledForInstance(ctx, instance.Workspace, v1pb.PlanFeature_FEATURE_EXTERNAL_SECRET_MANAGER, instance); err == nil {
		p, err := secretlib.ReplaceExternalSecret(ctx, dataSource.GetPassword(), dataSource.GetExternalSecret())
		if err != nil {
			return nil, err
		}
		password = p
	}
	connectionContext.InstanceID = instance.ResourceID
	connectionContext.EngineVersion = instance.Metadata.GetVersion()

	resolvedDataSource, err := dbutil.ResolveTLSMaterial(dataSource)
	if err != nil {
		return nil, err
	}

	driver, err := db.Open(
		ctx,
		instance.Metadata.GetEngine(),
		db.ConnectionConfig{
			DataSource:        resolvedDataSource,
			ConnectionContext: connectionContext,
			Password:          password,
		},
	)
	if err != nil {
		return nil, err
	}

	return driver, nil
}
