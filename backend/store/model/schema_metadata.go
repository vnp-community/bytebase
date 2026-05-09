package model

import (
	"slices"
	"strings"

	"github.com/pkg/errors"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// GetTable gets the schema by name.
func (s *SchemaMetadata) GetTable(name string) *TableMetadata {
	if s == nil {
		return nil
	}
	nameID := normalizeNameByCaseSensitivity(name, s.isObjectCaseSensitive)
	return s.internalTables[nameID]
}

// GetIndex gets the index by name.
// Index names are unique within a schema in most databases.
func (s *SchemaMetadata) GetIndex(name string) *IndexMetadata {
	if s == nil {
		return nil
	}
	for _, table := range s.internalTables {
		if index := table.GetIndex(name); index != nil {
			return index
		}
	}
	// Also search in materialized views
	nameID := normalizeNameByCaseSensitivity(name, s.isObjectCaseSensitive)
	for _, mv := range s.internalMaterializedView {
		for _, idx := range mv.Indexes {
			idxID := normalizeNameByCaseSensitivity(idx.Name, s.isObjectCaseSensitive)
			if idxID == nameID {
				// Return a wrapper IndexMetadata for the materialized view index
				return &IndexMetadata{
					proto:      idx,
					tableProto: &storepb.TableMetadata{Name: mv.Name},
				}
			}
		}
	}
	return nil
}

// GetView gets the view by name.
func (s *SchemaMetadata) GetView(name string) *storepb.ViewMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isObjectCaseSensitive)
	return s.internalViews[nameID]
}

func (s *SchemaMetadata) GetProcedure(name string) *storepb.ProcedureMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isDetailCaseSensitive)
	return s.internalProcedures[nameID]
}

func (s *SchemaMetadata) GetPackage(name string) *storepb.PackageMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isDetailCaseSensitive)
	return s.internalPackages[nameID]
}

// GetMaterializedView gets the materialized view by name.
func (s *SchemaMetadata) GetMaterializedView(name string) *storepb.MaterializedViewMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isObjectCaseSensitive)
	return s.internalMaterializedView[nameID]
}

// GetExternalTable gets the external table by name.
func (s *SchemaMetadata) GetExternalTable(name string) *ExternalTableMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isObjectCaseSensitive)
	return s.internalExternalTable[nameID]
}

// GetFunction gets the function by name.
// Note: For overloaded functions, this returns the first match by name only.
// Use signature-based lookup for precise matching.
func (s *SchemaMetadata) GetFunction(name string) *storepb.FunctionMetadata {
	for _, function := range s.proto.GetFunctions() {
		if s.isDetailCaseSensitive {
			if function.Name == name {
				return function
			}
		} else {
			if strings.EqualFold(function.Name, name) {
				return function
			}
		}
	}
	return nil
}

// GetSequence gets the sequence by name.
func (s *SchemaMetadata) GetSequence(name string) *storepb.SequenceMetadata {
	nameID := normalizeNameByCaseSensitivity(name, s.isDetailCaseSensitive)
	return s.internalSequences[nameID]
}

func (s *SchemaMetadata) GetSequencesByOwnerTable(name string) []*storepb.SequenceMetadata {
	var result []*storepb.SequenceMetadata
	for _, sequence := range s.internalSequences {
		if s.isObjectCaseSensitive {
			if sequence.OwnerTable == name {
				result = append(result, sequence)
			}
		} else {
			if strings.EqualFold(sequence.OwnerTable, name) {
				result = append(result, sequence)
			}
		}
	}
	return result
}

// GetProto gets the proto of SchemaMetadata.
func (s *SchemaMetadata) GetProto() *storepb.SchemaMetadata {
	return s.proto
}

// GetCatalog gets the catalog of SchemaMetadata.
func (s *SchemaMetadata) GetCatalog() *storepb.SchemaCatalog {
	return s.config
}

// ListTableNames lists the table names.
func (s *SchemaMetadata) ListTableNames() []string {
	var result []string
	for _, table := range s.internalTables {
		result = append(result, table.GetProto().GetName())
	}

	slices.Sort(result)
	return result
}

// ListProcedureNames lists the procedure names.
func (s *SchemaMetadata) ListProcedureNames() []string {
	var result []string
	for _, procedure := range s.internalProcedures {
		result = append(result, procedure.GetName())
	}

	slices.Sort(result)
	return result
}

// ListViewNames lists the view names.
func (s *SchemaMetadata) ListViewNames() []string {
	var result []string
	for _, view := range s.internalViews {
		result = append(result, view.GetName())
	}

	slices.Sort(result)
	return result
}

// ListForeignTableNames lists the foreign table names.
func (s *SchemaMetadata) ListForeignTableNames() []string {
	var result []string
	for _, table := range s.internalExternalTable {
		result = append(result, table.GetProto().GetName())
	}

	slices.Sort(result)
	return result
}

// ListMaterializedViewNames lists the materialized view names.
func (s *SchemaMetadata) ListMaterializedViewNames() []string {
	var result []string
	for _, view := range s.internalMaterializedView {
		result = append(result, view.GetName())
	}

	slices.Sort(result)
	return result
}

// ListSequenceNames lists the sequence names.
func (s *SchemaMetadata) ListSequenceNames() []string {
	var result []string
	for _, sequence := range s.internalSequences {
		result = append(result, sequence.GetName())
	}

	slices.Sort(result)
	return result
}

// CreateTable creates a new table in the schema.
// Returns the created TableMetadata or an error if the table already exists.
func (s *SchemaMetadata) CreateTable(tableName string) (*TableMetadata, error) {
	// Check if table already exists
	if s.GetTable(tableName) != nil {
		return nil, errors.Errorf("table %q already exists in schema %q", tableName, s.proto.Name)
	}

	// Create new table proto
	newTableProto := &storepb.TableMetadata{
		Name:    tableName,
		Columns: []*storepb.ColumnMetadata{},
		Indexes: []*storepb.IndexMetadata{},
	}

	// Add to proto's table list
	s.proto.Tables = append(s.proto.Tables, newTableProto)

	// Create TableMetadata wrapper
	tableMeta := &TableMetadata{
		isDetailCaseSensitive: s.isDetailCaseSensitive,
		internalColumn:        make(map[string]*ColumnMetadata),
		internalIndexes:       make(map[string]*IndexMetadata),
		proto:                 newTableProto,
	}

	// Add to internal map
	tableID := normalizeNameByCaseSensitivity(tableName, s.isObjectCaseSensitive)
	s.internalTables[tableID] = tableMeta

	return tableMeta, nil
}

// DropTable drops a table from the schema.
// Returns an error if the table does not exist.
func (s *SchemaMetadata) DropTable(tableName string) error {
	// Check if table exists
	if s.GetTable(tableName) == nil {
		return errors.Errorf("table %q does not exist in schema %q", tableName, s.proto.Name)
	}

	// Remove from internal map
	tableID := normalizeNameByCaseSensitivity(tableName, s.isObjectCaseSensitive)
	delete(s.internalTables, tableID)

	// Remove from proto's table list
	newTables := make([]*storepb.TableMetadata, 0, len(s.proto.Tables)-1)
	for _, table := range s.proto.Tables {
		if s.isObjectCaseSensitive {
			if table.Name != tableName {
				newTables = append(newTables, table)
			}
		} else {
			if !strings.EqualFold(table.Name, tableName) {
				newTables = append(newTables, table)
			}
		}
	}
	s.proto.Tables = newTables

	return nil
}

// RenameTable renames a table in the schema.
// Returns an error if the old table doesn't exist or new table already exists.
func (s *SchemaMetadata) RenameTable(oldName string, newName string) error {
	if oldName == newName {
		return nil
	}

	// Check if old table exists
	oldTable := s.GetTable(oldName)
	if oldTable == nil {
		return errors.Errorf("table %q does not exist in schema %q", oldName, s.proto.Name)
	}

	// Check if new table already exists
	if s.GetTable(newName) != nil {
		return errors.Errorf("table %q already exists in schema %q", newName, s.proto.Name)
	}

	// Remove from internal map using old name
	oldTableID := normalizeNameByCaseSensitivity(oldName, s.isObjectCaseSensitive)
	delete(s.internalTables, oldTableID)

	// Update the table name in the proto
	oldTable.proto.Name = newName

	// Add back to internal map using new name
	newTableID := normalizeNameByCaseSensitivity(newName, s.isObjectCaseSensitive)
	s.internalTables[newTableID] = oldTable

	return nil
}

// CreateView creates a new view in the schema.
// Returns an error if the view already exists.
func (s *SchemaMetadata) CreateView(viewName string, definition string, dependencyColumns []*storepb.DependencyColumn) (*storepb.ViewMetadata, error) {
	// Check if view already exists
	if s.GetView(viewName) != nil {
		return nil, errors.Errorf("view %q already exists in schema %q", viewName, s.proto.Name)
	}

	// Create new view proto
	newViewProto := &storepb.ViewMetadata{
		Name:              viewName,
		Definition:        definition,
		DependencyColumns: dependencyColumns,
	}

	// Add to proto's view list
	s.proto.Views = append(s.proto.Views, newViewProto)

	// Add to internal map
	viewID := normalizeNameByCaseSensitivity(viewName, s.isObjectCaseSensitive)
	s.internalViews[viewID] = newViewProto

	return newViewProto, nil
}

// DropView drops a view from the schema.
// Returns an error if the view does not exist.
func (s *SchemaMetadata) DropView(viewName string) error {
	// Check if view exists
	if s.GetView(viewName) == nil {
		return errors.Errorf("view %q does not exist in schema %q", viewName, s.proto.Name)
	}

	// Remove from internal map
	viewID := normalizeNameByCaseSensitivity(viewName, s.isObjectCaseSensitive)
	delete(s.internalViews, viewID)

	// Remove from proto's view list
	newViews := make([]*storepb.ViewMetadata, 0, len(s.proto.Views)-1)
	for _, view := range s.proto.Views {
		if s.isObjectCaseSensitive {
			if view.Name != viewName {
				newViews = append(newViews, view)
			}
		} else {
			if !strings.EqualFold(view.Name, viewName) {
				newViews = append(newViews, view)
			}
		}
	}
	s.proto.Views = newViews

	return nil
}

// RenameView renames a view in the schema.
// Returns an error if the old view doesn't exist or new view already exists.
func (s *SchemaMetadata) RenameView(oldName string, newName string) error {
	if oldName == newName {
		return nil
	}

	// Check if old view exists
	oldView := s.GetView(oldName)
	if oldView == nil {
		return errors.Errorf("view %q does not exist in schema %q", oldName, s.proto.Name)
	}

	// Check if new view already exists
	if s.GetView(newName) != nil {
		return errors.Errorf("view %q already exists in schema %q", newName, s.proto.Name)
	}

	// Remove from internal map using old name
	oldViewID := normalizeNameByCaseSensitivity(oldName, s.isObjectCaseSensitive)
	delete(s.internalViews, oldViewID)

	// Update the view name in the proto
	oldView.Name = newName

	// Add back to internal map using new name
	newViewID := normalizeNameByCaseSensitivity(newName, s.isObjectCaseSensitive)
	s.internalViews[newViewID] = oldView

	return nil
}

// CreateMaterializedView creates a new materialized view in the schema.
// Returns an error if the materialized view already exists.
func (s *SchemaMetadata) CreateMaterializedView(viewName string, definition string) (*storepb.MaterializedViewMetadata, error) {
	// Check if materialized view already exists
	if s.GetMaterializedView(viewName) != nil {
		return nil, errors.Errorf("materialized view %q already exists in schema %q", viewName, s.proto.Name)
	}

	// Create new materialized view proto
	newViewProto := &storepb.MaterializedViewMetadata{
		Name:       viewName,
		Definition: definition,
	}

	// Add to proto's materialized view list
	s.proto.MaterializedViews = append(s.proto.MaterializedViews, newViewProto)

	// Add to internal map
	viewID := normalizeNameByCaseSensitivity(viewName, s.isObjectCaseSensitive)
	s.internalMaterializedView[viewID] = newViewProto

	return newViewProto, nil
}

// DropMaterializedView drops a materialized view from the schema.
// Returns an error if the materialized view does not exist.
func (s *SchemaMetadata) DropMaterializedView(viewName string) error {
	// Check if materialized view exists
	if s.GetMaterializedView(viewName) == nil {
		return errors.Errorf("materialized view %q does not exist in schema %q", viewName, s.proto.Name)
	}

	// Remove from internal map
	viewID := normalizeNameByCaseSensitivity(viewName, s.isObjectCaseSensitive)
	delete(s.internalMaterializedView, viewID)

	// Remove from proto's materialized view list
	newViews := make([]*storepb.MaterializedViewMetadata, 0, len(s.proto.MaterializedViews)-1)
	for _, view := range s.proto.MaterializedViews {
		if s.isObjectCaseSensitive {
			if view.Name != viewName {
				newViews = append(newViews, view)
			}
		} else {
			if !strings.EqualFold(view.Name, viewName) {
				newViews = append(newViews, view)
			}
		}
	}
	s.proto.MaterializedViews = newViews

	return nil
}

// DropMaterializedViewIndex drops an index from a materialized view.
func (s *SchemaMetadata) DropMaterializedViewIndex(viewName, indexName string) error {
	mv := s.GetMaterializedView(viewName)
	if mv == nil {
		return errors.Errorf("materialized view %q does not exist in schema %q", viewName, s.proto.Name)
	}

	// Remove from indexes
	newIndexes := make([]*storepb.IndexMetadata, 0, len(mv.Indexes))
	found := false
	for _, idx := range mv.Indexes {
		if s.isObjectCaseSensitive {
			if idx.Name != indexName {
				newIndexes = append(newIndexes, idx)
			} else {
				found = true
			}
		} else {
			if !strings.EqualFold(idx.Name, indexName) {
				newIndexes = append(newIndexes, idx)
			} else {
				found = true
			}
		}
	}
	if !found {
		return errors.Errorf("index %q does not exist on materialized view %q", indexName, viewName)
	}
	mv.Indexes = newIndexes
	return nil
}

// GetDependentViews returns all views that depend on the given table and column.
// This is used to check if a column can be dropped or if a table can be dropped.
func (s *SchemaMetadata) GetDependentViews(tableName string, columnName string) []string {
	var dependentViews []string

	for _, view := range s.internalViews {
		viewProto := view
		for _, dep := range viewProto.DependencyColumns {
			// Schema is implicitly the same schema, or explicitly matches
			tableMatches := false
			if s.isObjectCaseSensitive {
				tableMatches = dep.Table == tableName
			} else {
				tableMatches = strings.EqualFold(dep.Table, tableName)
			}

			if tableMatches {
				// If columnName is empty, we're checking for table dependency
				if columnName == "" {
					dependentViews = append(dependentViews, viewProto.Name)
					break
				}

				// Check column dependency
				if s.isDetailCaseSensitive {
					if dep.Column == columnName {
						dependentViews = append(dependentViews, viewProto.Name)
						break
					}
				} else {
					if strings.EqualFold(dep.Column, columnName) {
						dependentViews = append(dependentViews, viewProto.Name)
						break
					}
				}
			}
		}
	}

	return dependentViews
}

func buildTablesMetadata(table *storepb.TableMetadata, tableCatalog *storepb.TableCatalog, isDetailCaseSensitive bool) ([]*TableMetadata, []string) {
	if table == nil {
		return nil, nil
	}

	// Build a map of column catalogs
	columnCatalogMap := make(map[string]*storepb.ColumnCatalog)
	if tableCatalog != nil {
		for _, columnCatalog := range tableCatalog.Columns {
			columnCatalogMap[columnCatalog.Name] = columnCatalog
		}
	}

	var result []*TableMetadata
	var name []string
	tableMetadata := &TableMetadata{
		isDetailCaseSensitive: isDetailCaseSensitive,
		internalColumn:        make(map[string]*ColumnMetadata),
		internalIndexes:       make(map[string]*IndexMetadata),
		proto:                 table,
		config:                tableCatalog,
	}
	for _, column := range table.Columns {
		columnCatalog := columnCatalogMap[column.Name]
		columnID := normalizeNameByCaseSensitivity(column.Name, isDetailCaseSensitive)
		tableMetadata.internalColumn[columnID] = &ColumnMetadata{
			proto:  column,
			config: columnCatalog,
		}
	}
	indexes := buildIndexesMetadata(table)
	for _, index := range indexes {
		indexID := normalizeNameByCaseSensitivity(index.proto.Name, isDetailCaseSensitive)
		tableMetadata.internalIndexes[indexID] = index
	}
	result = append(result, tableMetadata)
	name = append(name, table.Name)

	if table.Partitions != nil {
		partitionTables, partitionNames := buildTablesMetadataRecursive(table.Columns, columnCatalogMap, table.Partitions, tableMetadata, table, isDetailCaseSensitive)
		result = append(result, partitionTables...)
		name = append(name, partitionNames...)
	}
	return result, name
}

func buildIndexesMetadata(table *storepb.TableMetadata) []*IndexMetadata {
	if table == nil {
		return nil
	}

	var result []*IndexMetadata

	for _, index := range table.Indexes {
		result = append(result, &IndexMetadata{
			tableProto: table,
			proto:      index,
		})
	}

	return result
}

// buildTablesMetadataRecursive builds the partition tables recursively,
// returns the table metadata and the partition names, the length of them must be the same.
func buildTablesMetadataRecursive(originalColumn []*storepb.ColumnMetadata, columnCatalogMap map[string]*storepb.ColumnCatalog, partitions []*storepb.TablePartitionMetadata, root *TableMetadata, proto *storepb.TableMetadata, isDetailCaseSensitive bool) ([]*TableMetadata, []string) {
	if partitions == nil {
		return nil, nil
	}

	var tables []*TableMetadata
	var names []string

	for _, partition := range partitions {
		partitionMetadata := &TableMetadata{
			partitionOf:           root,
			isDetailCaseSensitive: isDetailCaseSensitive,
			internalColumn:        make(map[string]*ColumnMetadata),
			internalIndexes:       make(map[string]*IndexMetadata),
			proto:                 proto,
		}
		for _, column := range originalColumn {
			columnCatalog := columnCatalogMap[column.Name]
			columnID := normalizeNameByCaseSensitivity(column.Name, isDetailCaseSensitive)
			partitionMetadata.internalColumn[columnID] = &ColumnMetadata{
				proto:  column,
				config: columnCatalog,
			}
		}
		tables = append(tables, partitionMetadata)
		names = append(names, partition.Name)
		if partition.Subpartitions != nil {
			subTables, subNames := buildTablesMetadataRecursive(originalColumn, columnCatalogMap, partition.Subpartitions, partitionMetadata, proto, isDetailCaseSensitive)
			tables = append(tables, subTables...)
			names = append(names, subNames...)
		}
	}
	return tables, names
}

