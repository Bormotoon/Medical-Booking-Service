package db

import (
	"context"
	"database/sql"
	"fmt"
)

// TableNames that should be exported in audit reports.
var AuditTableNames = []string{
	"users",
	"user_settings",
	"cabinets",
	"cabinet_schedules",
	"cabinet_schedule_overrides",
	"hourly_bookings",
}

// GetTableNames returns list of table names to export.
func (db *DB) GetTableNames(ctx context.Context) (names []string, err error) {
	return AuditTableNames, nil
}

// GetTableData returns all rows from a table as maps.
func (db *DB) GetTableData(ctx context.Context, tableName string) (data []map[string]interface{}, columns []string, err error) {
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
	var rows *sql.Rows
	rows, err = db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, nil, err
	}

	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull, pk int
		var dfltValue sql.NullString
		if err = rows.Scan(&cid, &name, &typeName, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			return nil, nil, err
		}
		columns = append(columns, name)
	}
	rows.Close()

	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("table %s has no columns", tableName)
	}

	// Get data
	var dataRows *sql.Rows
	dataRows, err = db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", tableName))
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

		err = dataRows.Scan(valuePtrs...)
		if err != nil {
			return nil, nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		data = append(data, row)
	}

	return data, columns, dataRows.Err()
}

// GetDB returns the underlying sql.DB.
func (db *DB) GetDB() *sql.DB {
	return db.DB
}
