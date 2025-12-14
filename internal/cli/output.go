package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
)

// Output handles formatted output to the console
type Output struct {
	writer   io.Writer
	jsonMode bool
}

// NewOutput creates a new Output instance
func NewOutput(jsonMode bool) *Output {
	return &Output{
		writer:   os.Stdout,
		jsonMode: jsonMode,
	}
}

// NewOutputWithWriter creates a new Output instance with a custom writer
func NewOutputWithWriter(w io.Writer, jsonMode bool) *Output {
	return &Output{
		writer:   w,
		jsonMode: jsonMode,
	}
}

// PrintJSON outputs data as formatted JSON
func (o *Output) PrintJSON(data interface{}) error {
	enc := json.NewEncoder(o.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// PrintLine outputs a simple line
func (o *Output) PrintLine(format string, args ...interface{}) {
	fmt.Fprintf(o.writer, format+"\n", args...)
}

// PrintSuccess outputs a success message
func (o *Output) PrintSuccess(format string, args ...interface{}) {
	if o.jsonMode {
		o.PrintJSON(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf(format, args...),
		})
		return
	}
	fmt.Fprintf(o.writer, "✓ "+format+"\n", args...)
}

// PrintWarning outputs a warning message
func (o *Output) PrintWarning(format string, args ...interface{}) {
	if o.jsonMode {
		o.PrintJSON(map[string]interface{}{
			"status":  "warning",
			"message": fmt.Sprintf(format, args...),
		})
		return
	}
	fmt.Fprintf(o.writer, "⚠ "+format+"\n", args...)
}

// PrintError outputs an error message
func (o *Output) PrintError(err error) {
	if o.jsonMode {
		errOutput := map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}

		// Add additional context for AppError
		if appErr, ok := err.(*apperrors.AppError); ok {
			if len(appErr.Details) > 0 {
				errOutput["details"] = appErr.Details
			}
			if appErr.Remediation != "" {
				errOutput["remediation"] = appErr.Remediation
			}
		}

		o.PrintJSON(errOutput)
		return
	}

	fmt.Fprintf(o.writer, "✗ Error: %v\n", err)

	// Print remediation for AppError
	if appErr, ok := err.(*apperrors.AppError); ok {
		if appErr.Remediation != "" {
			fmt.Fprintf(o.writer, "  Hint: %s\n", appErr.Remediation)
		}
	}
}

// PrintValidationErrors outputs multiple validation errors
func (o *Output) PrintValidationErrors(errs *apperrors.ValidationErrors) {
	if o.jsonMode {
		errList := make([]string, len(errs.Errors))
		for i, e := range errs.Errors {
			errList[i] = e.Error()
		}
		o.PrintJSON(map[string]interface{}{
			"status": "error",
			"errors": errList,
		})
		return
	}

	fmt.Fprintf(o.writer, "✗ Found %d validation errors:\n", len(errs.Errors))
	for i, e := range errs.Errors {
		fmt.Fprintf(o.writer, "  %d. %v\n", i+1, e)
	}
}

// Table represents a tabular output
type Table struct {
	writer  *tabwriter.Writer
	headers []string
}

// PrintTable creates and prints a table
func (o *Output) PrintTable(headers []string, rows [][]string) {
	if o.jsonMode {
		// Convert to JSON array of objects
		jsonRows := make([]map[string]string, len(rows))
		for i, row := range rows {
			jsonRows[i] = make(map[string]string)
			for j, cell := range row {
				if j < len(headers) {
					jsonRows[i][headers[j]] = cell
				}
			}
		}
		o.PrintJSON(jsonRows)
		return
	}

	tw := tabwriter.NewWriter(o.writer, 0, 0, 2, ' ', 0)

	// Print headers
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Print separator
	sep := make([]string, len(headers))
	for i, h := range headers {
		sep[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))

	// Print rows
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	tw.Flush()
}

// Summary holds summary statistics for an operation
type Summary struct {
	Operation   string
	Items       map[string]int
	Warnings    []string
	DryRun      bool
}

// PrintSummary outputs an operation summary
func (o *Output) PrintSummary(s *Summary) {
	if o.jsonMode {
		o.PrintJSON(map[string]interface{}{
			"operation": s.Operation,
			"dry_run":   s.DryRun,
			"items":     s.Items,
			"warnings":  s.Warnings,
		})
		return
	}

	if s.DryRun {
		fmt.Fprintf(o.writer, "\n[DRY RUN] %s\n", s.Operation)
	} else {
		fmt.Fprintf(o.writer, "\n%s\n", s.Operation)
	}
	fmt.Fprintln(o.writer, strings.Repeat("-", len(s.Operation)+10))

	for label, count := range s.Items {
		fmt.Fprintf(o.writer, "  %s: %d\n", label, count)
	}

	if len(s.Warnings) > 0 {
		fmt.Fprintln(o.writer, "\nWarnings:")
		for _, w := range s.Warnings {
			fmt.Fprintf(o.writer, "  ⚠ %s\n", w)
		}
	}
}

// ProgressReporter reports progress for long-running operations
type ProgressReporter struct {
	output  *Output
	total   int
	current int
	label   string
}

// NewProgressReporter creates a new progress reporter
func (o *Output) NewProgressReporter(label string, total int) *ProgressReporter {
	return &ProgressReporter{
		output: o,
		total:  total,
		label:  label,
	}
}

// Update updates the progress
func (p *ProgressReporter) Update(current int, message string) {
	p.current = current
	if p.output.jsonMode {
		p.output.PrintJSON(map[string]interface{}{
			"progress": map[string]interface{}{
				"label":   p.label,
				"current": current,
				"total":   p.total,
				"message": message,
			},
		})
		return
	}
	fmt.Fprintf(p.output.writer, "\r%s: %d/%d - %s", p.label, current, p.total, message)
}

// Done marks the progress as complete
func (p *ProgressReporter) Done() {
	if !p.output.jsonMode {
		fmt.Fprintln(p.output.writer)
	}
}
