package db

import (
	"context"
	"database/sql"
	"io"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/store"
)

type RetryDriver struct {
	inner    Driver
	retryCfg store.RetryConfig
}

func NewRetryDriver(inner Driver, cfg store.RetryConfig) *RetryDriver {
	return &RetryDriver{inner: inner, retryCfg: cfg}
}

func (d *RetryDriver) Ping(ctx context.Context) error {
	return store.RetryableExec(ctx, d.retryCfg, func() error {
		return d.inner.Ping(ctx)
	})
}

func (d *RetryDriver) Execute(ctx context.Context, stmt string, opts ExecuteOptions) (int64, error) {
	var affected int64
	err := store.RetryableExec(ctx, d.retryCfg, func() error {
		var err error
		affected, err = d.inner.Execute(ctx, stmt, opts)
		return err
	})
	return affected, err
}

func (d *RetryDriver) QueryConn(ctx context.Context, conn *sql.Conn, statement string, queryContext QueryContext) ([]*v1pb.QueryResult, error) {
	return d.inner.QueryConn(ctx, conn, statement, queryContext)
}

// Passthrough (no retry)
func (d *RetryDriver) Close(ctx context.Context) error { return d.inner.Close(ctx) }
func (d *RetryDriver) GetDB() *sql.DB                  { return d.inner.GetDB() }
func (d *RetryDriver) Open(ctx context.Context, dbType storepb.Engine, config ConnectionConfig) (Driver, error) {
	return d.inner.Open(ctx, dbType, config)
}
func (d *RetryDriver) SyncInstance(ctx context.Context) (*InstanceMetadata, error) {
	return d.inner.SyncInstance(ctx)
}
func (d *RetryDriver) SyncDBSchema(ctx context.Context) (*storepb.DatabaseSchemaMetadata, error) {
	return d.inner.SyncDBSchema(ctx)
}
func (d *RetryDriver) Dump(ctx context.Context, out io.Writer, dbMetadata *storepb.DatabaseSchemaMetadata) error {
	return d.inner.Dump(ctx, out, dbMetadata)
}
