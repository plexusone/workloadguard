package cli

import (
	"fmt"

	"github.com/plexusone/workloadguard/internal/app"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file",
	Long: `Validate the configuration file syntax and policy definitions.

This command parses the configuration file, validates all policy definitions,
and reports any errors. It does not start the daemon or execute any actions.`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	path := configPath()

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if err := app.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Printf("Configuration valid: %s\n", path)
	fmt.Printf("  Policies defined: %d\n", len(cfg.Policies))
	fmt.Printf("  Periodic interval: %s\n", cfg.PeriodicInterval.Duration())
	fmt.Printf("  Load threshold: %.1f\n", cfg.LoadThreshold)

	for name, p := range cfg.Policies {
		fmt.Printf("  - %s: process=%s max_count=%d actions=%v\n",
			name, p.Process, p.MaxCount, p.Actions,
		)
	}

	return nil
}
