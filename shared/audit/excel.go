package audit

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// ExcelizeWriter implements ExcelWriter using excelize library.
type ExcelizeWriter struct {
	file         *excelize.File
	currentSheet string
	currentRow   int
}

// NewExcelizeWriter creates a new Excel writer.
func NewExcelizeWriter() ExcelWriter {
	return &ExcelizeWriter{
		file: excelize.NewFile(),
	}
}

// AddSheet adds a new sheet with the given name.
func (w *ExcelizeWriter) AddSheet(name string) error {
	// Truncate sheet name to 31 chars (Excel limit)
	if len(name) > 31 {
		name = name[:31]
	}

	// Check if it's the first sheet (Sheet1 exists by default)
	if w.currentSheet == "" {
		// Rename default sheet
		w.file.SetSheetName("Sheet1", name)
	} else {
		// Create new sheet
		_, err := w.file.NewSheet(name)
		if err != nil {
			return fmt.Errorf("create sheet %s: %w", name, err)
		}
	}

	w.currentSheet = name
	w.currentRow = 1
	return nil
}

// WriteHeader writes column headers to current sheet.
func (w *ExcelizeWriter) WriteHeader(columns []string) error {
	if w.currentSheet == "" {
		return fmt.Errorf("no active sheet")
	}

	for i, col := range columns {
		cell, err := excelize.CoordinatesToCellName(i+1, w.currentRow)
		if err != nil {
			return err
		}
		if err := w.file.SetCellValue(w.currentSheet, cell, col); err != nil {
			return err
		}
	}

	// Apply bold style to header
	style, err := w.file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err == nil {
		startCell, _ := excelize.CoordinatesToCellName(1, w.currentRow)
		endCell, _ := excelize.CoordinatesToCellName(len(columns), w.currentRow)
		_ = w.file.SetCellStyle(w.currentSheet, startCell, endCell, style)
	}

	w.currentRow++
	return nil
}

// WriteRow writes a data row to current sheet.
func (w *ExcelizeWriter) WriteRow(row []interface{}) error {
	if w.currentSheet == "" {
		return fmt.Errorf("no active sheet")
	}

	for i, val := range row {
		cell, err := excelize.CoordinatesToCellName(i+1, w.currentRow)
		if err != nil {
			return err
		}
		if err := w.file.SetCellValue(w.currentSheet, cell, val); err != nil {
			return err
		}
	}

	w.currentRow++
	return nil
}

// Save writes the Excel file to the writer.
func (w *ExcelizeWriter) Save(wr io.Writer) error {
	return w.file.Write(wr)
}

// SaveToFile writes the Excel file to disk.
func (w *ExcelizeWriter) SaveToFile(path string) error {
	return w.file.SaveAs(path)
}

// Close releases resources.
func (w *ExcelizeWriter) Close() error {
	return w.file.Close()
}
