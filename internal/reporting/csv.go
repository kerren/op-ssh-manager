package reporting

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// CSVWriter writes reports in CSV format
type CSVWriter struct {
	IncludeHeaders bool
}

// NewCSVWriter creates a new CSV writer
func NewCSVWriter() *CSVWriter {
	return &CSVWriter{
		IncludeHeaders: true,
	}
}

// Write writes a report to a writer
func (w *CSVWriter) Write(report *Report, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write headers
	if w.IncludeHeaders {
		if err := csvWriter.Write(GetHeaders()); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}

	// Write rows
	for _, row := range report.Rows {
		record := rowToCSVRecord(row)
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// WriteFile writes a report to a file
func (w *CSVWriter) WriteFile(report *Report, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return w.Write(report, file)
}

// rowToCSVRecord converts an InventoryRow to a CSV record
func rowToCSVRecord(row *InventoryRow) []string {
	return []string{
		row.Alias,
		row.Hostname,
		strconv.Itoa(row.Port),
		row.RemoteUser,
		row.UserEmail,
		row.JumpAlias,
		row.JumpPath,
		strings.Join(row.KeyTitles, "; "),
		strconv.Itoa(row.KeysResolved),
		strconv.Itoa(row.KeysMissing),
		row.Environment,
		row.OwnerTeam,
		strings.Join(row.Tags, "; "),
	}
}
