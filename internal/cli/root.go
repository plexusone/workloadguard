// Package cli implements the workloadguard command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "workloadguard",
	Short: "macOS system health daemon",
	Long: `WorkloadGuard monitors macOS system health and enforces workload policies.

It detects runaway processes (e.g., excessive rg, git, node), high system load,
memory pressure, and other resource exhaustion conditions. When thresholds are
exceeded, it logs diagnostics, sends notifications, and optionally terminates
offending processes.

Triggers:
  - Periodic: checks run at configured intervals (e.g., every 60 seconds)
  - Load-triggered: immediate check when 1-minute load average exceeds threshold`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&cfgFile,
		"config",
		"c",
		"",
		"config file (default: $HOME/.config/workloadguard/config.toml)",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false,
		"enable verbose output",
	)
}

func configPath() string {
	if cfgFile != "" {
		return cfgFile
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}

	return fmt.Sprintf("%s/.config/workloadguard/config.toml", home)
}
