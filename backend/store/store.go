// Package store is the implementation for managing Bytebase's own metadata in a PostgreSQL database.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/store/model"
)

// Store provides database access to all raw objects.
type Store struct {
	poolManager *PoolManager
	enableCache   bool

	// Cache.
	Secret            string
	userEmailCache    *lru.Cache[string, *UserMessage]
	instanceCache     *lru.Cache[string, *InstanceMessage]
	databaseCache     *lru.Cache[string, *DatabaseMessage]
	projectCache      *lru.Cache[string, *ProjectMessage]
	policyCache       *lru.Cache[string, *PolicyMessage]
	settingCache      *lru.Cache[string, *SettingMessage]
	rolesCache        *expirable.LRU[string, *RoleMessage]
	groupCache        *expirable.LRU[string, *GroupMessage]
	groupMembersCache *expirable.LRU[string, map[string]bool]
	memberGroupsCache *expirable.LRU[string, []string]
	dbSchemaCache     *expirable.LRU[string, *model.DatabaseMetadata]
	dbSchemaL2Cache   *CompressedSchemaCache
	iamPolicyCache    *expirable.LRU[string, *IamPolicyMessage]

	// Large objects.
	sheetFullCache *lru.Cache[string, *SheetMessage]
}

// New creates a new instance of Store.
// pgURL can be either a direct PostgreSQL URL or a file path containing the URL.
func New(ctx context.Context, pgURL string, enableCache bool) (*Store, error) {
	// Initialize dual pool manager
	poolManager, err := NewPoolManager(ctx, pgURL, PoolConfig{})
	if err != nil {
		return nil, err
	}

	dbCount := getEntityCount(ctx, poolManager.GetDefaultDB(), "db", "deleted = false")
	instanceCount := getEntityCount(ctx, poolManager.GetDefaultDB(), "instance", "deleted = false")

	dbCacheSize := adaptiveCacheSize(dbCount, 32768, 500000, 50)
	schemaL1Size := adaptiveCacheSize(dbCount, 512, 5000, 2)
	schemaL2Size := adaptiveCacheSize(dbCount, 5000, 100000, 25)
	instanceCacheSize := adaptiveCacheSize(instanceCount, 1024, 32768, 80)

	userEmailCache, err := lru.New[string, *UserMessage](32768)
	if err != nil {
		return nil, err
	}
	instanceCache, err := lru.New[string, *InstanceMessage](instanceCacheSize)
	if err != nil {
		return nil, err
	}
	databaseCache, err := lru.New[string, *DatabaseMessage](dbCacheSize)
	if err != nil {
		return nil, err
	}
	projectCache, err := lru.New[string, *ProjectMessage](32768)
	if err != nil {
		return nil, err
	}
	policyCache, err := lru.New[string, *PolicyMessage](4096)
	if err != nil {
		return nil, err
	}
	settingCache, err := lru.New[string, *SettingMessage](1024)
	if err != nil {
		return nil, err
	}
	rolesCache := expirable.NewLRU[string, *RoleMessage](128, nil, time.Minute)
	sheetFullCache, err := lru.New[string, *SheetMessage](10)
	if err != nil {
		return nil, err
	}
	groupCache := expirable.NewLRU[string, *GroupMessage](1024, nil, time.Minute)
	groupMembersCache := expirable.NewLRU[string, map[string]bool](1024, nil, time.Minute)
	memberGroupsCache := expirable.NewLRU[string, []string](4096, nil, time.Minute)
	dbSchemaCache := expirable.NewLRU[string, *model.DatabaseMetadata](schemaL1Size, nil, 10*time.Minute)
	dbSchemaL2Cache := NewCompressedSchemaCache(schemaL2Size, 30*time.Minute)
	iamPolicyCache := expirable.NewLRU[string, *IamPolicyMessage](1024, nil, time.Minute)

	s := &Store{
		poolManager: poolManager,
		enableCache:   enableCache,

		// Cache.
		userEmailCache:    userEmailCache,
		instanceCache:     instanceCache,
		databaseCache:     databaseCache,
		projectCache:      projectCache,
		policyCache:       policyCache,
		settingCache:      settingCache,
		rolesCache:        rolesCache,
		sheetFullCache:    sheetFullCache,
		groupCache:        groupCache,
		groupMembersCache: groupMembersCache,
		memberGroupsCache: memberGroupsCache,
		dbSchemaCache:     dbSchemaCache,
		dbSchemaL2Cache:   dbSchemaL2Cache,
		iamPolicyCache:    iamPolicyCache,
	}

	return s, nil
}

// Close closes underlying db.
func (s *Store) Close() error {
	return s.poolManager.Close()
}

// GetDB returns the default (API) database pool for backward compatibility.
func (s *Store) GetDB() *sql.DB {
	return s.poolManager.GetDefaultDB()
}

// GetRunnerDB returns the isolated runner database pool for background tasks.
func (s *Store) GetRunnerDB() *sql.DB {
	return s.poolManager.GetDB(PoolRunner)
}

// DeleteCache deletes the cache.
func (s *Store) DeleteCache() {
	s.settingCache.Purge()
	s.policyCache.Purge()
	s.userEmailCache.Purge()
}

// PurgeGroupCaches purges all group-related caches.
func (s *Store) PurgeGroupCaches() {
	s.groupCache.Purge()
	s.groupMembersCache.Purge()
	s.memberGroupsCache.Purge()
}

// PurgeIamPolicyCaches purges all IAM policy caches.
func (s *Store) PurgeIamPolicyCaches() {
	s.iamPolicyCache.Purge()
}

// removeGroupMembersCache removes all possible cache entries for a group's members,
// covering both groups/{email} and groups/{id} lookups.
func (s *Store) removeGroupMembersCache(workspace string, group *GroupMessage) {
	cacheKey := getGroupMembersCacheKey(workspace, formatGroupName(group))
	s.groupMembersCache.Remove(cacheKey)
}

func getInstanceCacheKey(instanceID string) string {
	return instanceID
}

func getSettingCacheKey(workspace string, name storepb.SettingName) string {
	return fmt.Sprintf("workspaces/%s/settings/%s", workspace, name.String())
}

// formatGroupName returns "groups/{email}" if email is set, otherwise "groups/{id}".
// This mirrors utils.FormatGroupName but avoids a circular import.
func formatGroupName(group *GroupMessage) string {
	if group.Email != "" {
		return "groups/" + group.Email
	}
	return "groups/" + group.ID
}

func getGroupCacheKey(workspace string, group *GroupMessage) string {
	return fmt.Sprintf("workspaces/%s/%s", workspace, formatGroupName(group))
}

func getGroupMembersCacheKey(workspace, groupName string) string {
	return fmt.Sprintf("workspaces/%s/%s/members", workspace, groupName)
}

func getMemberGroupsCacheKey(workspace, userName string) string {
	return fmt.Sprintf("workspaces/%s/memberGroups/%s", workspace, userName)
}

func getPolicyCacheKey(workspace string, resourceType storepb.Policy_Resource, resource string, policyType storepb.Policy_Type) string {
	return fmt.Sprintf("workspaces/%s/policies/%s/%s/%s", workspace, resourceType.String(), resource, policyType.String())
}

func getDatabaseCacheKey(workspace, instanceID, databaseName string) string {
	return fmt.Sprintf("workspaces/%s/%s/%s", workspace, instanceID, databaseName)
}

func getDBSchemaCacheKey(instanceID, databaseName string) string {
	return fmt.Sprintf("%s/%s", instanceID, databaseName)
}

func adaptiveCacheSize(entityCount, minSize, maxSize, coveragePct int) int {
	target := entityCount * coveragePct / 100
	if target < minSize {
		return minSize
	}
	if target > maxSize {
		return maxSize
	}
	return target
}

func getEntityCount(ctx context.Context, db *sql.DB, table, condition string) int {
	var count int
	query := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", table, condition)
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0
	}
	return count
}
