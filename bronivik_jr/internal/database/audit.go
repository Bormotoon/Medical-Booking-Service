package database

import (
	"context"
	"database/sql"
	"fmt"
)

// TableNames that should be exported in audit reports.
var AuditTableNames = []string{
	"users",
	"user_settings",
	"items",
	"bookings",
	"sync_queue",
}

// GetTableNames returns list of table names to export.
func (db *DB) GetTableNames(ctx context.Context) ([]string, error) {
	return AuditTableNames, nil
}

// GetTableData returns all rows from a table as maps.
func (db *DB) GetTableData(ctx context.Context, tableName string) (result []map[string]interface{}, columns []string, err error) {
	// Validate table name to prevent SQL injection
	validTable := false
	for _, t := range AuditTableNames {
		if t == tableName {
			validTable = true
			break
		}
	}
	if !validTable {
		return nil, nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	// Get column names
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, nil, err
	}

	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull, pk int
		var dfltValue sql.NullString
		if errScan := rows.Scan(&cid, &name, &typeName, &notNull, &dfltValue, &pk); errScan != nil {
			rows.Close()
			return nil, nil, errScan
		}
		columns = append(columns, name)
	}
	rows.Close()

	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("table %s has no columns", tableName)
	}

	// Get data
	dataRows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return nil, nil, err
	}
	defer dataRows.Close()

	for dataRows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if errScan := dataRows.Scan(valuePtrs...); errScan != nil {
			return nil, nil, errScan
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	return result, columns, dataRows.Err()
}

// GetDB returns the underlying sql.DB.
func (db *DB) GetDB() *sql.DB {
	return db.DB
}
