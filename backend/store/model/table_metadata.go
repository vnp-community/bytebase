package model

import (
	"strings"

	"github.com/pkg/errors"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

func (t *TableMetadata) GetOwner() string {
	return t.proto.Owner
}

func (t *TableMetadata) GetTableComment() string {
	return t.proto.Comment
}

// GetColumn gets the column by name.
func (t *TableMetadata) GetColumn(name string) *ColumnMetadata {
	if t == nil {
		return nil
	}
	nameID := normalizeNameByCaseSensitivity(name, t.isDetailCaseSensitive)
	return t.internalColumn[nameID]
}

func (t *TableMetadata) GetIndex(name string) *IndexMetadata {
	if t == nil {
		return nil
	}
	nameID := normalizeNameByCaseSensitivity(name, t.isDetailCaseSensitive)
	return t.internalIndexes[nameID]
}

func (t *TableMetadata) ListIndexes() []*IndexMetadata {
	var result []*IndexMetadata
	for _, index := range t.internalIndexes {
		result = append(result, index)
	}
	return result
}

func (t *TableMetadata) GetPrimaryKey() *IndexMetadata {
	for _, index := range t.internalIndexes {
		if index.proto.Primary {
			return index
		}
	}
	return nil
}

func (t *TableMetadata) GetProto() *storepb.TableMetadata {
	return t.proto
}

func (t *TableMetadata) GetCatalog() *storepb.TableCatalog {
	return t.config
}

// CreateColumn creates a new column in the table.
// Returns an error if the column already exists.
func (t *TableMetadata) CreateColumn(columnProto *storepb.ColumnMetadata, columnCatalog *storepb.ColumnCatalog) error {
	// Check if column already exists
	if t.GetColumn(columnProto.Name) != nil {
		return errors.Errorf("column %q already exists in table %q", columnProto.Name, t.proto.Name)
	}

	// Add to proto's column list
	t.proto.Columns = append(t.proto.Columns, columnProto)

	// Create ColumnMetadata wrapper and add to internal map
	columnID := normalizeNameByCaseSensitivity(columnProto.Name, t.isDetailCaseSensitive)
	t.internalColumn[columnID] = &ColumnMetadata{
		proto:  columnProto,
		config: columnCatalog,
	}

	return nil
}

// DropColumn drops a column from the table.
// Returns an error if the column does not exist.
func (t *TableMetadata) DropColumn(columnName string) error {
	return t.dropColumnInternal(columnName, true)
}

// dropColumnInternal is the internal implementation that allows controlling position renumbering.
func (t *TableMetadata) dropColumnInternal(columnName string, renumberPositions bool) error {
	// Check if column exists
	if t.GetColumn(columnName) == nil {
		return errors.Errorf("column %q does not exist in table %q", columnName, t.proto.Name)
	}

	// Remove from internal map
	columnID := normalizeNameByCaseSensitivity(columnName, t.isDetailCaseSensitive)
	delete(t.internalColumn, columnID)

	// Remove from proto's column list
	newColumns := make([]*storepb.ColumnMetadata, 0, len(t.proto.Columns)-1)
	for _, column := range t.proto.Columns {
		if t.isDetailCaseSensitive {
			if column.Name != columnName {
				newColumns = append(newColumns, column)
			}
		} else {
			if !strings.EqualFold(column.Name, columnName) {
				newColumns = append(newColumns, column)
			}
		}
	}
	t.proto.Columns = newColumns

	// Renumber positions to be sequential (1-indexed) if requested
	// MySQL/TiDB: renumber positions (1, 2, 3, ...)
	// PostgreSQL: keep original positions (gaps are allowed)
	if renumberPositions {
		for i, col := range newColumns {
			col.Position = int32(i + 1)
		}
	}

	// Remove column from indexes that reference it
	for _, index := range t.internalIndexes {
		var newExpressions []string
		for _, expr := range index.proto.Expressions {
			if t.isDetailCaseSensitive {
				if expr != columnName {
					newExpressions = append(newExpressions, expr)
				}
			} else {
				if !strings.EqualFold(expr, columnName) {
					newExpressions = append(newExpressions, expr)
				}
			}
		}
		index.proto.Expressions = newExpressions
	}

	// Remove empty indexes (indexes that had all columns dropped)
	var indexesToRemove []string
	for indexName, index := range t.internalIndexes {
		if len(index.proto.Expressions) == 0 {
			indexesToRemove = append(indexesToRemove, indexName)
		}
	}
	for _, indexName := range indexesToRemove {
		delete(t.internalIndexes, indexName)
	}

	// Remove empty indexes from proto
	newIndexes := make([]*storepb.IndexMetadata, 0)
	for _, index := range t.proto.Indexes {
		if len(index.Expressions) > 0 {
			newIndexes = append(newIndexes, index)
		}
	}
	t.proto.Indexes = newIndexes

	return nil
}

// DropColumnWithoutRenumbering drops a column from the table without renumbering positions.
// This is used for PostgreSQL where column positions are stable (attnum) and shouldn't be renumbered.
// Returns an error if the column doesn't exist.
func (t *TableMetadata) DropColumnWithoutRenumbering(columnName string) error {
	return t.dropColumnInternal(columnName, false)
}

// DropColumnWithoutUpdatingIndexes drops a column from the table without updating index expressions.
// This is used when changing a column definition (MODIFY/CHANGE COLUMN) where we want to:
// 1. Drop the old column from the column list
// 2. Manually rename it in index expressions
// 3. Create a new column with the new definition
// Returns an error if the column doesn't exist.
func (t *TableMetadata) DropColumnWithoutUpdatingIndexes(columnName string) error {
	// Check if column exists
	if t.GetColumn(columnName) == nil {
		return errors.Errorf("column %q does not exist in table %q", columnName, t.proto.Name)
	}

	// Remove from internal map
	columnID := normalizeNameByCaseSensitivity(columnName, t.isDetailCaseSensitive)
	delete(t.internalColumn, columnID)

	// Remove from proto's column list
	newColumns := make([]*storepb.ColumnMetadata, 0, len(t.proto.Columns)-1)
	for _, column := range t.proto.Columns {
		if t.isDetailCaseSensitive {
			if column.Name != columnName {
				newColumns = append(newColumns, column)
			}
		} else {
			if !strings.EqualFold(column.Name, columnName) {
				newColumns = append(newColumns, column)
			}
		}
	}
	t.proto.Columns = newColumns

	// NOTE: We intentionally do NOT renumber positions here
	// The caller (tidbCompleteTableChangeColumn) will handle position adjustments
	// as part of the column reordering logic.

	// NOTE: We intentionally do NOT update index expressions here
	// The caller is responsible for updating index expressions as needed

	return nil
}

// RenameColumn renames a column in the table.
// Returns an error if the old column doesn't exist or new column already exists.
func (t *TableMetadata) RenameColumn(oldName string, newName string) error {
	if oldName == newName {
		return nil
	}

	// Check if old column exists
	oldColumn := t.GetColumn(oldName)
	if oldColumn == nil {
		return errors.Errorf("column %q does not exist in table %q", oldName, t.proto.Name)
	}

	// Check if new column already exists
	if t.GetColumn(newName) != nil {
		return errors.Errorf("column %q already exists in table %q", newName, t.proto.Name)
	}

	// Remove from internal map using old name
	oldColumnID := normalizeNameByCaseSensitivity(oldName, t.isDetailCaseSensitive)
	delete(t.internalColumn, oldColumnID)

	// Update the column name in the proto
	oldColumn.proto.Name = newName

	// Add back to internal map using new name
	newColumnID := normalizeNameByCaseSensitivity(newName, t.isDetailCaseSensitive)
	t.internalColumn[newColumnID] = oldColumn

	// Update column references in indexes
	for _, index := range t.internalIndexes {
		for i, expr := range index.proto.Expressions {
			if t.isDetailCaseSensitive {
				if expr == oldName {
					index.proto.Expressions[i] = newName
				}
			} else {
				if strings.EqualFold(expr, oldName) {
					index.proto.Expressions[i] = newName
				}
			}
		}
	}

	return nil
}

func (t *ExternalTableMetadata) GetProto() *storepb.ExternalTableMetadata {
	return t.proto
}

// GetColumn gets the column by name.
func (t *ExternalTableMetadata) GetColumn(name string) *storepb.ColumnMetadata {
	nameID := normalizeNameByCaseSensitivity(name, t.isDetailCaseSensitive)
	return t.internal[nameID]
}

func (i *IndexMetadata) GetProto() *storepb.IndexMetadata {
	return i.proto
}

func (i *IndexMetadata) GetTableProto() *storepb.TableMetadata {
	return i.tableProto
}

func (c *ColumnMetadata) GetProto() *storepb.ColumnMetadata {
	return c.proto
}

func (c *ColumnMetadata) GetCatalog() *storepb.ColumnCatalog {
	return c.config
}

// CreateIndex creates a new index in the table.
// Returns an error if the index already exists.
func (t *TableMetadata) CreateIndex(indexProto *storepb.IndexMetadata) error {
	// Check if index already exists
	if t.GetIndex(indexProto.Name) != nil {
		return errors.Errorf("index %q already exists in table %q", indexProto.Name, t.proto.Name)
	}

	// Add to proto's index list
	t.proto.Indexes = append(t.proto.Indexes, indexProto)

	// Add to internal map
	indexID := normalizeNameByCaseSensitivity(indexProto.Name, t.isDetailCaseSensitive)
	t.internalIndexes[indexID] = &IndexMetadata{
		tableProto: t.proto,
		proto:      indexProto,
	}

	return nil
}

// DropIndex drops an index from the table.
// Returns an error if the index does not exist.
func (t *TableMetadata) DropIndex(indexName string) error {
	// Check if index exists
	if t.GetIndex(indexName) == nil {
		return errors.Errorf("index %q does not exist in table %q", indexName, t.proto.Name)
	}

	// Remove from internal map
	indexID := normalizeNameByCaseSensitivity(indexName, t.isDetailCaseSensitive)
	delete(t.internalIndexes, indexID)

	// Remove from proto's index list
	newIndexes := make([]*storepb.IndexMetadata, 0, len(t.proto.Indexes)-1)
	for _, index := range t.proto.Indexes {
		if t.isDetailCaseSensitive {
			if index.Name != indexName {
				newIndexes = append(newIndexes, index)
			}
		} else {
			if !strings.EqualFold(index.Name, indexName) {
				newIndexes = append(newIndexes, index)
			}
		}
	}
	t.proto.Indexes = newIndexes

	return nil
}

// RenameIndex renames an index in the table.
// Returns an error if the old index doesn't exist or new index already exists.
func (t *TableMetadata) RenameIndex(oldName string, newName string) error {
	if oldName == newName {
		return nil
	}

	// Check if old index exists
	oldIndex := t.GetIndex(oldName)
	if oldIndex == nil {
		return errors.Errorf("index %q does not exist in table %q", oldName, t.proto.Name)
	}

	// Check if new index already exists
	if t.GetIndex(newName) != nil {
		return errors.Errorf("index %q already exists in table %q", newName, t.proto.Name)
	}

	// Remove from internal map using old name
	oldIndexID := normalizeNameByCaseSensitivity(oldName, t.isDetailCaseSensitive)
	delete(t.internalIndexes, oldIndexID)

	// Update the index name in the proto
	oldIndex.proto.Name = newName

	// Add back to internal map using new name
	newIndexID := normalizeNameByCaseSensitivity(newName, t.isDetailCaseSensitive)
	t.internalIndexes[newIndexID] = oldIndex

	return nil
}

// getIsDetailCaseSensitive is a special case for MySQL, MariaDB, and TiDB.
// From MySQL documentation:
// Partition, subpartition, column, index, stored routine, event, and resource group names are not case-sensitive on any platform, nor are column aliases.
func getIsDetailCaseSensitive(engine storepb.Engine) bool {
	switch engine {
	case storepb.Engine_MYSQL, storepb.Engine_MARIADB, storepb.Engine_TIDB, storepb.Engine_MSSQL, storepb.Engine_OCEANBASE:
		return false
	default:
		return true
	}
}
