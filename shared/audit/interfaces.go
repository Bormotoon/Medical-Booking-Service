package audit

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"
)

// TableExporter provides access to database tables for export.
type TableExporter interface {
	// GetTableNames returns list of table names to export.
	GetTableNames(ctx context.Context) ([]string, error)

	// GetTableData returns rows for a table as maps.
	GetTableData(ctx context.Context, tableName string) ([]map[string]interface{}, []string, error)

	// GetDB returns underlying sql.DB for custom queries.
	GetDB() *sql.DB
}

// ExcelWriter writes data to Excel format.
type ExcelWriter interface {
	// AddSheet adds a new sheet with the given name.
	AddSheet(name string) error

	// WriteHeader writes column headers to current sheet.
	WriteHeader(columns []string) error

	// WriteRow writes a data row to current sheet.
	WriteRow(row []interface{}) error

	// Save writes the Excel file to the writer.
	Save(w io.Writer) error

	// SaveToFile writes the Excel file to disk.
	SaveToFile(path string) error
}

// Notifier sends audit reports to managers.
type Notifier interface {
	// SendDocument sends a document to managers.
	SendDocument(ctx context.Context, filename string, data io.Reader, caption string) error
}

// Logger for audit operations.
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// MonthNames in Russian for filename generation.
var MonthNames = map[time.Month]string{
	time.January:   "Январь",
	time.February:  "Февраль",
	time.March:     "Март",
	time.April:     "Апрель",
	time.May:       "Май",
	time.June:      "Июнь",
	time.July:      "Июль",
	time.August:    "Август",
	time.September: "Сентябрь",
	time.October:   "Октябрь",
	time.November:  "Ноябрь",
	time.December:  "Декабрь",
}

// GenerateFilename creates a filename like "Январь_2026.xlsx"
func GenerateFilename(t time.Time) string {
	monthName := MonthNames[t.Month()]
	return fmt.Sprintf("%s_%d.xlsx", monthName, t.Year())
}

// GenerateFilenameForPreviousMonth creates filename for the previous month.
func GenerateFilenameForPreviousMonth() string {
	now := time.Now()
	prevMonth := now.AddDate(0, -1, 0)
	return GenerateFilename(prevMonth)
}
