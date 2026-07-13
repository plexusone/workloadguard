package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/plexusone/workloadguard/internal/app"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/logging"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run a one-shot policy evaluation",
	Long: `Run a one-shot policy evaluation and print the results.

This command collects a system snapshot, evaluates all configured policies,
and outputs the decisions in JSON format. No actions are taken unless
--execute is specified.`,
	RunE: runCheck,
}

var (
	executeActions bool
	outputFormat   string
)

func init() {
	checkCmd.Flags().BoolVar(
		&executeActions,
		"execute",
		false,
		"execute actions for triggered policies",
	)
	checkCmd.Flags().StringVarP(
		&outputFormat,
		"output",
		"o",
		"json",
		"output format (json, text)",
	)

	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.NewStderr("warn")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := app.Check(ctx, app.CheckOptions{
		Config:  cfg,
		Logger:  logger,
		Execute: executeActions,
	})
	if err != nil {
		return err
	}

	if outputFormat == "json" {
		return result.FormatJSON(os.Stdout)
	}

	return result.FormatText(os.Stdout)
}
