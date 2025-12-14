package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/inventory"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/reporting"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate inventory reports",
	Long: `Generate reports of the SSH server inventory.

Supported formats:
  - csv: Comma-separated values (opens in Excel/Sheets)
  - html: HTML table (opens in browser)
  - xlsx: Excel spreadsheet (requires additional library)

Examples:
  op-ssh-manager report --format csv --output inventory.csv
  op-ssh-manager report --format html --output inventory.html
  op-ssh-manager report --format csv  # outputs to stdout`,
	RunE: runReport,
}

var (
	reportFormat string
	reportOutput string
)

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVarP(&reportFormat, "format", "f", "csv", "output format (csv, html)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output file path (default: stdout)")
}

func runReport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	// Validate format
	format := strings.ToLower(reportFormat)
	if format != "csv" && format != "html" {
		return fmt.Errorf("unsupported format: %s (supported: csv, html)", format)
	}

	// Build inventory
	opClient := op.NewClient()
	if !opClient.IsInstalled() {
		return fmt.Errorf("1Password CLI (op) not found")
	}
	if !opClient.IsSignedIn(ctx) {
		return fmt.Errorf("not signed in to 1Password")
	}

	tags := inventory.TagConfig{
		Server: cfg.Tags.Server,
		Jump:   cfg.Tags.Jump,
		Key:    cfg.Tags.Key,
	}

	builder := inventory.NewBuilder(opClient, tags, cfg.Vaults)
	buildResult, _, err := builder.BuildWithKeys(ctx)
	if err != nil {
		return err
	}

	// Get account info for metadata
	var accountName, userEmail string
	if account, err := opClient.Account(ctx); err == nil {
		accountName = account.Name
	}
	if whoami, err := opClient.Whoami(ctx); err == nil {
		userEmail = whoami.Email
	}

	// Build report
	meta := reporting.ReportMetadata{
		GeneratedAt: time.Now(),
		GeneratedBy: userEmail,
		AccountName: accountName,
		VaultScope:  cfg.Vaults,
	}

	report := reporting.BuildReport(buildResult.Inventory, meta)

	// Determine output destination
	var writer *os.File
	if reportOutput == "" {
		writer = os.Stdout
	} else {
		var err error
		writer, err = os.Create(reportOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer writer.Close()
	}

	// Write report
	switch format {
	case "csv":
		csvWriter := reporting.NewCSVWriter()
		if err := csvWriter.Write(report, writer); err != nil {
			return err
		}
	case "html":
		htmlWriter := reporting.NewHTMLWriter()
		if err := htmlWriter.Write(report, writer); err != nil {
			return err
		}
	}

	if reportOutput != "" {
		output.PrintSuccess("report written to %s", reportOutput)
	}

	return nil
}
