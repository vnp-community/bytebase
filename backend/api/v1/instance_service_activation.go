package v1

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/bytebase/bytebase/backend/common"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	v1pb "github.com/bytebase/bytebase/backend/generated-go/v1"
	"github.com/bytebase/bytebase/backend/plugin/db"
	"github.com/bytebase/bytebase/backend/store"
)

// AddDataSource adds a data source to an instance.
func (s *InstanceService) AddDataSource(ctx context.Context, req *connect.Request[v1pb.AddDataSourceRequest]) (*connect.Response[v1pb.Instance], error) {
	if req.Msg.DataSource == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("data sources is required"))
	}
	// We only support add RO type datasouce to instance now, see more details in instance_service.proto.
	if req.Msg.DataSource.Type != v1pb.DataSourceType_READ_ONLY {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("only support adding read-only data source"))
	}

	dataSource, err := convertV1DataSource(req.Msg.DataSource)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("failed to convert data source"))
	}

	instance, err := getInstanceMessage(ctx, s.store, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	if instance.Deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q has been deleted", req.Msg.Name))
	}
	for _, ds := range instance.Metadata.GetDataSources() {
		if ds.GetId() == req.Msg.DataSource.Id {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("data source already exists with the same name"))
		}
	}
	if err := s.checkDataSource(ctx, instance, dataSource); err != nil {
		return nil, err
	}
	if err := validateAndSanitizeDataSourceTLS(dataSource); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	metadata := proto.CloneOf(instance.Metadata)
	metadata.DataSources = append(metadata.DataSources, dataSource)
	if err := s.checkInstanceDataSources(ctx, instance, metadata.GetDataSources()); err != nil {
		return nil, err
	}

	if err := s.licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_INSTANCE_READ_ONLY_CONNECTION, instance); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	// Test connection.
	if req.Msg.ValidateOnly {
		err := func() error {
			driver, err := s.dbFactory.GetDataSourceDriver(
				ctx, instance, dataSource,
				db.ConnectionContext{
					ReadOnly: dataSource.GetType() == storepb.DataSourceType_READ_ONLY,
				},
			)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database driver"))
			}
			defer driver.Close(ctx)
			if err := driver.Ping(ctx); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "invalid datasource %s", dataSource.GetType()))
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
		result := s.convertToV1Instance(ctx, instanceWithMetadata(instance, metadata))
		return connect.NewResponse(result), nil
	}

	if dataSource.GetType() != storepb.DataSourceType_READ_ONLY {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("only read-only data source can be added"))
	}

	instance, err = s.store.UpdateInstance(ctx, &store.UpdateInstanceMessage{
		ResourceID: &instance.ResourceID,
		Workspace:  instance.Workspace,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := s.convertToV1Instance(ctx, instance)
	return connect.NewResponse(result), nil
}

// UpdateDataSource updates a data source of an instance.
func (s *InstanceService) UpdateDataSource(ctx context.Context, req *connect.Request[v1pb.UpdateDataSourceRequest]) (*connect.Response[v1pb.Instance], error) {
	if req.Msg.DataSource == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("datasource is required"))
	}
	if req.Msg.UpdateMask == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("update_mask must be set"))
	}

	instance, err := getInstanceMessage(ctx, s.store, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	if instance.Deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q has been deleted", req.Msg.Name))
	}
	metadata := proto.CloneOf(instance.Metadata)
	var dataSource *storepb.DataSource
	for _, ds := range metadata.GetDataSources() {
		if ds.GetId() == req.Msg.DataSource.Id {
			dataSource = ds
			break
		}
	}
	if dataSource == nil {
		if req.Msg.AllowMissing {
			return s.AddDataSource(ctx, connect.NewRequest(&v1pb.AddDataSourceRequest{
				Name:         req.Msg.Name,
				DataSource:   req.Msg.DataSource,
				ValidateOnly: req.Msg.ValidateOnly,
			}))
		}
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf(`cannot found data source "%s"`, req.Msg.DataSource.Id))
	}

	if dataSource.GetType() == storepb.DataSourceType_READ_ONLY {
		if err := s.licenseService.IsFeatureEnabledForInstance(ctx, common.GetWorkspaceIDFromContext(ctx), v1pb.PlanFeature_FEATURE_INSTANCE_READ_ONLY_CONNECTION, instance); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
	}

	for _, path := range req.Msg.UpdateMask.Paths {
		switch path {
		case "username":
			dataSource.Username = req.Msg.DataSource.Username
		case "password":
			dataSource.Password = req.Msg.DataSource.Password
		case "ssl_ca":
			dataSource.SslCa = req.Msg.DataSource.SslCa
		case "ssl_ca_path":
			dataSource.SslCaPath = req.Msg.DataSource.SslCaPath
		case "ssl_cert":
			dataSource.SslCert = req.Msg.DataSource.SslCert
		case "ssl_cert_path":
			dataSource.SslCertPath = req.Msg.DataSource.SslCertPath
		case "ssl_key":
			dataSource.SslKey = req.Msg.DataSource.SslKey
		case "ssl_key_path":
			dataSource.SslKeyPath = req.Msg.DataSource.SslKeyPath
		case "host":
			dataSource.Host = req.Msg.DataSource.Host
		case "port":
			dataSource.Port = req.Msg.DataSource.Port
		case "database":
			dataSource.Database = req.Msg.DataSource.Database
		case "srv":
			dataSource.Srv = req.Msg.DataSource.Srv
		case "authentication_database":
			dataSource.AuthenticationDatabase = req.Msg.DataSource.AuthenticationDatabase
		case "sid":
			dataSource.Sid = req.Msg.DataSource.Sid
		case "service_name":
			dataSource.ServiceName = req.Msg.DataSource.ServiceName
		case "ssh_host":
			dataSource.SshHost = req.Msg.DataSource.SshHost
		case "ssh_port":
			dataSource.SshPort = req.Msg.DataSource.SshPort
		case "ssh_user":
			dataSource.SshUser = req.Msg.DataSource.SshUser
		case "ssh_password":
			dataSource.SshPassword = req.Msg.DataSource.SshPassword
		case "ssh_private_key":
			dataSource.SshPrivateKey = req.Msg.DataSource.SshPrivateKey
		case "authentication_private_key":
			dataSource.AuthenticationPrivateKey = req.Msg.DataSource.AuthenticationPrivateKey
		case "authentication_private_key_passphrase":
			dataSource.AuthenticationPrivateKeyPassphrase = req.Msg.DataSource.AuthenticationPrivateKeyPassphrase
		case "external_secret":
			externalSecret, err := convertV1DataSourceExternalSecret(req.Msg.DataSource.ExternalSecret)
			if err != nil {
				return nil, err
			}
			dataSource.ExternalSecret = externalSecret
		case "sasl_config":
			dataSource.SaslConfig = convertV1DataSourceSaslConfig(req.Msg.DataSource.SaslConfig)
		case "authentication_type":
			dataSource.AuthenticationType = convertV1AuthenticationType(req.Msg.DataSource.AuthenticationType)
		case "additional_addresses":
			dataSource.AdditionalAddresses = convertAdditionalAddresses(req.Msg.DataSource.AdditionalAddresses)
		case "replica_set":
			dataSource.ReplicaSet = req.Msg.DataSource.ReplicaSet
		case "direct_connection":
			dataSource.DirectConnection = req.Msg.DataSource.DirectConnection
		case "region":
			dataSource.Region = req.Msg.DataSource.Region
		case "warehouse_id":
			dataSource.WarehouseId = req.Msg.DataSource.WarehouseId
		case "use_ssl":
			dataSource.UseSsl = req.Msg.DataSource.UseSsl
		case "verify_tls_certificate":
			dataSource.VerifyTlsCertificate = req.Msg.DataSource.VerifyTlsCertificate
		case "redis_type":
			dataSource.RedisType = convertV1RedisType(req.Msg.DataSource.RedisType)
		case "master_name":
			dataSource.MasterName = req.Msg.DataSource.MasterName
		case "master_username":
			dataSource.MasterUsername = req.Msg.DataSource.MasterUsername
		case "master_password":
			dataSource.MasterPassword = req.Msg.DataSource.MasterPassword
		case "extra_connection_parameters":
			dataSource.ExtraConnectionParameters = req.Msg.DataSource.ExtraConnectionParameters
		case "azure_credential", "aws_credential", "gcp_credential":
			switch req.Msg.DataSource.AuthenticationType {
			case v1pb.DataSource_AZURE_IAM:
				if azureCredential := req.Msg.DataSource.GetAzureCredential(); azureCredential != nil {
					dataSource.IamExtension = &storepb.DataSource_AzureCredential_{
						AzureCredential: &storepb.DataSource_AzureCredential{
							TenantId:     azureCredential.TenantId,
							ClientId:     azureCredential.ClientId,
							ClientSecret: azureCredential.ClientSecret,
						},
					}
					dataSource.AuthenticationType = storepb.DataSource_AZURE_IAM
				} else {
					dataSource.IamExtension = nil
				}
			case v1pb.DataSource_AWS_RDS_IAM:
				if awsCredential := req.Msg.DataSource.GetAwsCredential(); awsCredential != nil {
					dataSource.IamExtension = &storepb.DataSource_AwsCredential{
						AwsCredential: &storepb.DataSource_AWSCredential{
							AccessKeyId:     awsCredential.AccessKeyId,
							SecretAccessKey: awsCredential.SecretAccessKey,
							SessionToken:    awsCredential.SessionToken,
							RoleArn:         awsCredential.RoleArn,
							ExternalId:      awsCredential.ExternalId,
						},
					}
					dataSource.AuthenticationType = storepb.DataSource_AWS_RDS_IAM
				} else {
					dataSource.IamExtension = nil
				}
			case v1pb.DataSource_GOOGLE_CLOUD_SQL_IAM:
				if gcpCredential := req.Msg.DataSource.GetGcpCredential(); gcpCredential != nil {
					dataSource.IamExtension = &storepb.DataSource_GcpCredential{
						GcpCredential: &storepb.DataSource_GCPCredential{
							Content: gcpCredential.Content,
						},
					}
					dataSource.AuthenticationType = storepb.DataSource_GOOGLE_CLOUD_SQL_IAM
				} else {
					dataSource.IamExtension = nil
				}
			default:
			}
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.Errorf(`unsupported update_mask "%s"`, path))
		}
	}

	clearDataSourceAuthentication(dataSource)
	if err := validateAndSanitizeDataSourceTLS(dataSource); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.checkInstanceDataSources(ctx, instance, metadata.GetDataSources()); err != nil {
		return nil, err
	}

	// Test connection.
	if req.Msg.ValidateOnly {
		err := func() error {
			driver, err := s.dbFactory.GetDataSourceDriver(
				ctx, instance, dataSource,
				db.ConnectionContext{ReadOnly: dataSource.GetType() == storepb.DataSourceType_READ_ONLY},
			)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.Wrapf(err, "failed to get database driver"))
			}
			defer driver.Close(ctx)
			if err := driver.Ping(ctx); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, errors.Wrapf(err, "invalid datasource %s", dataSource.GetType()))
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
		result := s.convertToV1Instance(ctx, instanceWithMetadata(instance, metadata))
		return connect.NewResponse(result), nil
	}

	instance, err = s.store.UpdateInstance(ctx, &store.UpdateInstanceMessage{
		ResourceID: &instance.ResourceID,
		Workspace:  instance.Workspace,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result := s.convertToV1Instance(ctx, instance)
	return connect.NewResponse(result), nil
}

// RemoveDataSource removes a data source to an instance.
func (s *InstanceService) RemoveDataSource(ctx context.Context, req *connect.Request[v1pb.RemoveDataSourceRequest]) (*connect.Response[v1pb.Instance], error) {
	if req.Msg.DataSource == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("data sources is required"))
	}

	instance, err := getInstanceMessage(ctx, s.store, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	if instance.Deleted {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q has been deleted", req.Msg.Name))
	}

	metadata := proto.CloneOf(instance.Metadata)
	var updatedDataSources []*storepb.DataSource
	var dataSource *storepb.DataSource
	for _, ds := range instance.Metadata.GetDataSources() {
		if ds.GetId() == req.Msg.DataSource.Id {
			dataSource = ds
		} else {
			updatedDataSources = append(updatedDataSources, ds)
		}
	}
	if dataSource == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("data source not found"))
	}

	// We only support remove RO type datasource to instance now, see more details in instance_service.proto.
	if dataSource.GetType() != storepb.DataSourceType_READ_ONLY {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("only support remove read-only data source"))
	}

	metadata.DataSources = updatedDataSources
	instance, err = s.store.UpdateInstance(ctx, &store.UpdateInstanceMessage{
		ResourceID: &instance.ResourceID,
		Workspace:  instance.Workspace,
		Metadata:   metadata,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := s.convertToV1Instance(ctx, instance)
	return connect.NewResponse(result), nil
}

func getInstanceMessage(ctx context.Context, stores *store.Store, name string) (*store.InstanceMessage, error) {
	instanceID, err := common.GetInstanceID(name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	find := &store.FindInstanceMessage{
		Workspace:  common.GetWorkspaceIDFromContext(ctx),
		ResourceID: &instanceID,
	}
	instance, err := stores.GetInstance(ctx, find)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.Errorf("instance %q not found", name))
	}

	return instance, nil
}

// buildInstanceName builds the instance name with the given instance ID.
func buildInstanceName(instanceID string) string {
	var b strings.Builder
	b.Grow(len(common.InstanceNamePrefix) + len(instanceID))
	_, _ = b.WriteString(common.InstanceNamePrefix)
	_, _ = b.WriteString(instanceID)
	return b.String()
}

// buildEnvironmentName builds the environment name with the given environment ID.
func buildEnvironmentName(environmentID *string) *string {
	if environmentID == nil || *environmentID == "" {
		return nil
	}
	var b strings.Builder
	b.Grow(len("environments/") + len(*environmentID))
	_, _ = b.WriteString("environments/")
	_, _ = b.WriteString(*environmentID)
	result := b.String()
	return &result
}

func (s *InstanceService) instanceCountGuard(ctx context.Context) error {
	workspaceID := common.GetWorkspaceIDFromContext(ctx)
	instanceLimit := s.licenseService.GetInstanceLimit(ctx, workspaceID)

	count, err := s.store.CountActiveInstances(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if count >= instanceLimit {
		return connect.NewError(connect.CodeResourceExhausted, errors.Errorf("reached the maximum instance count %d", instanceLimit))
	}

	return nil
}

// validateExtraConnectionParameters validates extra connection parameters for security risks.
func validateExtraConnectionParameters(engine storepb.Engine, params map[string]string) error {
	// Validate MySQL-compatible engines
	switch engine {
	case storepb.Engine_MYSQL, storepb.Engine_MARIADB, storepb.Engine_OCEANBASE, storepb.Engine_TIDB:
		for key := range params {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if normalizedKey == "allowallfiles" {
				// Disables file allowlist for LOAD DATA LOCAL INFILE and allows all files (might be insecure)
				return errors.Errorf("connection parameter %q is not allowed for security reasons. This parameter can allow a malicious database server to read arbitrary files from the client", key)
			}
		}
	default:
		// No validation needed for other engines
	}
	return nil
}
