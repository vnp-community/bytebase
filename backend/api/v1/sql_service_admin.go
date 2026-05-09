package v1

import (
	"context"
	"database/sql"
	"io"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/common/log"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/db"
	"github.com/bytebase/bytebase/backend/store"
)

// AdminExecute executes the SQL statement.
func (s *SQLService) AdminExecute(ctx context.Context, stream *connect.BidiStream[v1pb.AdminExecuteRequest, v1pb.AdminExecuteResponse]) error {
	if err := s.licenseService.IsFeatureEnabled(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_SQL_EDITOR_ADMIN_MODE); err != nil {
		return connect.NewError(connect.CodePermissionDenied, err)
	}
	var driver db.Driver
	var conn *sql.Conn
	var connectionName string

	clean := func() {
		if conn != nil {
			if err := conn.Close(); err != nil {
				slog.Warn("failed to close connection", log.BBError(err))
			}
		}
		if driver != nil {
			driver.Close(ctx)
		}
	}
	defer clean()
	for {
		request, err := stream.Receive()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return connect.NewError(connect.CodeInternal, errors.Errorf("failed to receive request: %v", err))
		}

		user, instance, database, err := s.prepareRelatedMessage(ctx, request.Name)
		if err != nil {
			return err
		}

		// We only need to get the driver and connection once.
		if driver == nil || connectionName != request.Name {
			clean()
			connectionName = request.Name
			driver, err = s.dbFactory.GetAdminDatabaseDriver(ctx, instance, database, db.ConnectionContext{})
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database driver: %v", err))
			}
			sqlDB := driver.GetDB()
			if sqlDB != nil {
				conn, err = sqlDB.Conn(ctx)
				if err != nil {
					return connect.NewError(connect.CodeInternal, errors.Errorf("failed to get database connection: %v", err))
				}
			}
		}

		queryRestriction := getEffectiveQueryDataPolicy(
			ctx,
			s.store,
			s.licenseService,
			request.Limit,
			database.ProjectID,
		)
		queryContext := db.QueryContext{
			OperatorEmail:        user.Email,
			Container:            request.GetContainer(),
			MaximumSQLResultSize: queryRestriction.MaximumResultSize,
			Limit:                int(queryRestriction.MaximumResultRows),
		}
		if request.Schema != nil {
			queryContext.Schema = *request.Schema
		}
		if queryRestriction.MaxQueryTimeoutInSeconds > 0 {
			queryContext.Timeout = &durationpb.Duration{Seconds: queryRestriction.MaxQueryTimeoutInSeconds}
		}

		result, duration, queryErr := executeWithTimeout(
			ctx,
			driver,
			conn,
			request.Statement,
			queryContext,
		)

		s.createQueryHistory(database, store.QueryHistoryTypeQuery, request.Statement, user.Email, duration, queryErr)
		response := &v1pb.AdminExecuteResponse{}
		if queryErr != nil {
			response.Results = []*v1pb.QueryResult{
				{
					Error: queryErr.Error(),
				},
			}
		} else {
			response.Results = result
		}

		if err := stream.Send(response); err != nil {
			return connect.NewError(connect.CodeInternal, errors.Errorf("failed to send response: %v", err))
		}
	}
}
