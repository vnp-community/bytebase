package model

import (
	"slices"
	"strings"

	"github.com/pkg/errors"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// DatabaseMetadata is the unified database schema including metadata, config, and raw dump.
// This struct combines what were previously separate types: DatabaseMetadata, and DatabaseConfig.
type DatabaseMetadata struct {
	// Proto representations for serialization
	proto   *storepb.DatabaseSchemaMetadata
	config  *storepb.DatabaseConfig
	rawDump []byte

	// Case sensitivity flags
	isObjectCaseSensitive bool
	isDetailCaseSensitive bool

	// Metadata fields (formerly in DatabaseMetadata)
	searchPath     []PGSearchPathItem
	internal       map[string]*SchemaMetadata
	linkedDatabase map[string]*storepb.LinkedDatabaseMetadata
}

// SchemaMetadata is the unified metadata for a schema, combining proto metadata and catalog config.
type SchemaMetadata struct {
	isObjectCaseSensitive    bool
	isDetailCaseSensitive    bool
	internalTables           map[string]*TableMetadata
	internalExternalTable    map[string]*ExternalTableMetadata
	internalViews            map[string]*storepb.ViewMetadata
	internalMaterializedView map[string]*storepb.MaterializedViewMetadata
	internalProcedures       map[string]*storepb.ProcedureMetadata
	internalSequences        map[string]*storepb.SequenceMetadata
	internalPackages         map[string]*storepb.PackageMetadata

	proto  *storepb.SchemaMetadata
	config *storepb.SchemaCatalog
}

// TableMetadata is the unified metadata for a table, combining proto metadata and catalog config.
type TableMetadata struct {
	// If partitionOf is not nil, it means this table is a partition table.
	partitionOf *TableMetadata

	isDetailCaseSensitive bool
	internalColumn        map[string]*ColumnMetadata
	internalIndexes       map[string]*IndexMetadata

	proto  *storepb.TableMetadata
	config *storepb.TableCatalog
}

// ExternalTableMetadata is the metadata for a external table.
type ExternalTableMetadata struct {
	isDetailCaseSensitive bool
	internal              map[string]*storepb.ColumnMetadata
	proto                 *storepb.ExternalTableMetadata
}

type IndexMetadata struct {
	tableProto *storepb.TableMetadata
	proto      *storepb.IndexMetadata
}

// ColumnMetadata is the unified metadata for a column, combining proto metadata and catalog config.
type ColumnMetadata struct {
	proto  *storepb.ColumnMetadata
	config *storepb.ColumnCatalog
}

// normalizeNameByCaseSensitivity normalizes a name based on case sensitivity.
// If caseSensitive is true, returns the name as-is; otherwise returns lowercase.
func normalizeNameByCaseSensitivity(name string, caseSensitive bool) string {
	if caseSensitive {
		return name
	}
	return strings.ToLower(name)
}

func NewDatabaseMetadata(
	metadata *storepb.DatabaseSchemaMetadata,
	schema []byte,
	config *storepb.DatabaseConfig,
	engine storepb.Engine,
	isObjectCaseSensitive bool,
) *DatabaseMetadata {
	isDetailCaseSensitive := getIsDetailCaseSensitive(engine)
	dbMetadata := &DatabaseMetadata{
		proto:                 metadata,
		rawDump:               schema,
		config:                config,
		isObjectCaseSensitive: isObjectCaseSensitive,
		isDetailCaseSensitive: isDetailCaseSensitive,
		searchPath:            ParsePGConfiguredSearchPath(metadata.SearchPath),
		internal:              make(map[string]*SchemaMetadata),
		linkedDatabase:        make(map[string]*storepb.LinkedDatabaseMetadata),
	}

	// Build a map of schema catalogs for quick lookup
	schemaCatalogMap := make(map[string]*storepb.SchemaCatalog)
	if config != nil {
		for _, schemaCatalog := range config.Schemas {
			schemaCatalogMap[schemaCatalog.Name] = schemaCatalog
		}
	}

	// Build schema metadata maps
	for _, s := range metadata.Schemas {
		// Get matching schema catalog if it exists
		schemaCatalog := schemaCatalogMap[s.Name]

		// Build a map of table catalogs for this schema
		tableCatalogMap := make(map[string]*storepb.TableCatalog)
		if schemaCatalog != nil {
			for _, tableCatalog := range schemaCatalog.Tables {
				tableCatalogMap[tableCatalog.Name] = tableCatalog
			}
		}

		schemaMetadata := &SchemaMetadata{
			isObjectCaseSensitive:    isObjectCaseSensitive,
			isDetailCaseSensitive:    isDetailCaseSensitive,
			internalTables:           make(map[string]*TableMetadata),
			internalExternalTable:    make(map[string]*ExternalTableMetadata),
			internalViews:            make(map[string]*storepb.ViewMetadata),
			internalMaterializedView: make(map[string]*storepb.MaterializedViewMetadata),
			internalProcedures:       make(map[string]*storepb.ProcedureMetadata),
			internalPackages:         make(map[string]*storepb.PackageMetadata),
			internalSequences:        make(map[string]*storepb.SequenceMetadata),
			proto:                    s,
			config:                   schemaCatalog,
		}
		for _, table := range s.Tables {
			tableCatalog := tableCatalogMap[table.Name]
			tables, names := buildTablesMetadata(table, tableCatalog, isDetailCaseSensitive)
			for i, table := range tables {
				tableID := normalizeNameByCaseSensitivity(names[i], isObjectCaseSensitive)
				schemaMetadata.internalTables[tableID] = table
			}
		}
		for _, externalTable := range s.ExternalTables {
			externalTableMetadata := &ExternalTableMetadata{
				isDetailCaseSensitive: isDetailCaseSensitive,
				internal:              make(map[string]*storepb.ColumnMetadata),
				proto:                 externalTable,
			}
			for _, column := range externalTable.Columns {
				columnID := normalizeNameByCaseSensitivity(column.Name, isDetailCaseSensitive)
				externalTableMetadata.internal[columnID] = column
			}
			tableID := normalizeNameByCaseSensitivity(externalTable.Name, isObjectCaseSensitive)
			schemaMetadata.internalExternalTable[tableID] = externalTableMetadata
		}
		for _, view := range s.Views {
			viewID := normalizeNameByCaseSensitivity(view.Name, isObjectCaseSensitive)
			schemaMetadata.internalViews[viewID] = view
		}
		for _, materializedView := range s.MaterializedViews {
			viewID := normalizeNameByCaseSensitivity(materializedView.Name, isObjectCaseSensitive)
			schemaMetadata.internalMaterializedView[viewID] = materializedView
		}
		for _, procedure := range s.Procedures {
			procedureID := normalizeNameByCaseSensitivity(procedure.Name, isDetailCaseSensitive)
			schemaMetadata.internalProcedures[procedureID] = procedure
		}
		for _, p := range s.Packages {
			packageID := normalizeNameByCaseSensitivity(p.Name, isDetailCaseSensitive)
			schemaMetadata.internalPackages[packageID] = p
		}
		for _, sequence := range s.Sequences {
			sequenceID := normalizeNameByCaseSensitivity(sequence.Name, isDetailCaseSensitive)
			schemaMetadata.internalSequences[sequenceID] = sequence
		}
		schemaID := normalizeNameByCaseSensitivity(s.Name, isObjectCaseSensitive)
		dbMetadata.internal[schemaID] = schemaMetadata
	}

	for _, dbLink := range metadata.LinkedDatabases {
		dbLinkID := normalizeNameByCaseSensitivity(dbLink.Name, isObjectCaseSensitive)
		dbMetadata.linkedDatabase[dbLinkID] = dbLink
	}

	return dbMetadata
}

func NewDatabaseMetadataFromProto(metadata *storepb.DatabaseSchemaMetadata) *DatabaseMetadata {
	return NewDatabaseMetadata(metadata, nil, nil, storepb.Engine_ENGINE_UNSPECIFIED, false)
}

func (d *DatabaseMetadata) GetProto() *storepb.DatabaseSchemaMetadata {
	return d.proto
}

// ReplaceFrom replaces the internal state of this DatabaseMetadata with that
// of another. The receiver pointer remains stable so callers that hold a
// reference to it continue to see the updated state.
func (d *DatabaseMetadata) ReplaceFrom(other *DatabaseMetadata) {
	d.proto = other.proto
	d.config = other.config
	d.rawDump = other.rawDump
	d.isObjectCaseSensitive = other.isObjectCaseSensitive
	d.isDetailCaseSensitive = other.isDetailCaseSensitive
	d.searchPath = other.searchPath
	d.internal = other.internal
	d.linkedDatabase = other.linkedDatabase
}

func (d *DatabaseMetadata) GetRawDump() []byte {
	return d.rawDump
}

func (d *DatabaseMetadata) GetConfig() *storepb.DatabaseConfig {
	return d.config
}

func (d *DatabaseMetadata) GetConfiguredSearchPath() []PGSearchPathItem {
	return slices.Clone(d.searchPath)
}

func (d *DatabaseMetadata) GetSearchPath() []string {
	return ResolvePGSearchPath(d.searchPath, "", nil)
}

func (d *DatabaseMetadata) GetSearchPathForCurrentUser(currentUser string) []string {
	return ResolvePGSearchPath(d.searchPath, currentUser, d.hasSchema)
}

func (d *DatabaseMetadata) hasSchema(name string) bool {
	return d.GetSchemaMetadata(name) != nil
}

func (d *DatabaseMetadata) GetSchemaMetadata(name string) *SchemaMetadata {
	schemaID := normalizeNameByCaseSensitivity(name, d.isObjectCaseSensitive)
	return d.internal[schemaID]
}

func (d *DatabaseMetadata) DatabaseName() string {
	if d.proto == nil {
		return ""
	}
	return d.proto.Name
}

func (d *DatabaseMetadata) HasNoTable() bool {
	for _, schema := range d.internal {
		if schema != nil && schema.proto != nil && len(schema.proto.Tables) > 0 {
			return false
		}
	}
	return true
}

func (d *DatabaseMetadata) ListSchemaNames() []string {
	var result []string
	for _, schema := range d.internal {
		result = append(result, schema.GetProto().Name)
	}
	return result
}

func (d *DatabaseMetadata) GetLinkedDatabase(name string) *storepb.LinkedDatabaseMetadata {
	nameID := normalizeNameByCaseSensitivity(name, d.isObjectCaseSensitive)
	return d.linkedDatabase[nameID]
}

func (d *DatabaseMetadata) GetIsObjectCaseSensitive() bool {
	return d.isObjectCaseSensitive
}

func (d *DatabaseMetadata) CreateSchema(schemaName string) *SchemaMetadata {
	// Create new schema proto
	newSchemaProto := &storepb.SchemaMetadata{
		Name:   schemaName,
		Tables: []*storepb.TableMetadata{},
		Views:  []*storepb.ViewMetadata{},
	}

	// Add to proto's schema list
	d.proto.Schemas = append(d.proto.Schemas, newSchemaProto)

	// Create SchemaMetadata wrapper
	schemaMeta := &SchemaMetadata{
		isObjectCaseSensitive:    d.isObjectCaseSensitive,
		isDetailCaseSensitive:    d.isDetailCaseSensitive,
		internalTables:           make(map[string]*TableMetadata),
		internalExternalTable:    make(map[string]*ExternalTableMetadata),
		internalViews:            make(map[string]*storepb.ViewMetadata),
		internalMaterializedView: make(map[string]*storepb.MaterializedViewMetadata),
		internalProcedures:       make(map[string]*storepb.ProcedureMetadata),
		internalPackages:         make(map[string]*storepb.PackageMetadata),
		internalSequences:        make(map[string]*storepb.SequenceMetadata),
		proto:                    newSchemaProto,
	}

	// Add to internal map
	schemaID := normalizeNameByCaseSensitivity(schemaName, d.isObjectCaseSensitive)
	d.internal[schemaID] = schemaMeta

	return schemaMeta
}

func (d *DatabaseMetadata) DropSchema(schemaName string) error {
	// Check if schema exists
	if d.GetSchemaMetadata(schemaName) == nil {
		return errors.Errorf("schema %q does not exist in database %q", schemaName, d.proto.GetName())
	}

	// Remove from internal map
	schemaID := normalizeNameByCaseSensitivity(schemaName, d.isObjectCaseSensitive)
	delete(d.internal, schemaID)

	// Remove from proto's schema list
	newSchemas := make([]*storepb.SchemaMetadata, 0, len(d.proto.Schemas)-1)
	for _, schema := range d.proto.Schemas {
		if d.isObjectCaseSensitive {
			if schema.Name != schemaName {
				newSchemas = append(newSchemas, schema)
			}
		} else {
			if !strings.EqualFold(schema.Name, schemaName) {
				newSchemas = append(newSchemas, schema)
			}
		}
	}
	d.proto.Schemas = newSchemas

	return nil
}

