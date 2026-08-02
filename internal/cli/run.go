package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/plexusone/workloadguard/internal/app"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/logging"
	"github.com/plexusone/workloadguard/internal/metrics"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the workloadguard daemon",
	Long: `Start the workloadguard daemon with dual-trigger monitoring.

The daemon monitors system health using two trigger mechanisms:

1. Periodic checks: Run at the configured interval (default: 60 seconds)
2. Load-triggered checks: Run immediately when 1-minute load average
   exceeds the configured threshold (default: 400)

The load monitor polls more frequently (every 5 seconds) to detect
sudden load spikes between periodic checks.`,
	RunE: runDaemon,
}

var (
	dryRun        bool
	enableMetrics bool
	enableAPI     bool
	addr          string
)

func init() {
	runCmd.Flags().BoolVar(
		&dryRun,
		"dry-run",
		false,
		"log actions without executing them",
	)
	runCmd.Flags().BoolVar(
		&enableMetrics,
		"metrics",
		false,
		"enable Prometheus metrics endpoint",
	)
	runCmd.Flags().BoolVar(
		&enableAPI,
		"api",
		false,
		"enable JSON API server",
	)
	runCmd.Flags().StringVar(
		&addr,
		"addr",
		":9090",
		"HTTP server address for --metrics and/or --api",
	)

	rootCmd.AddCommand(runCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if dryRun {
		cfg.DryRun = true
	}

	logger := logging.NewStdout(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	opts := app.RunOptions{
		Config:        cfg,
		Logger:        logger,
		EnableMetrics: enableMetrics,
		EnableAPI:     enableAPI,
		Addr:          addr,
	}

	if enableMetrics {
		opts.Metrics = metrics.New()
	}

	return app.Run(ctx, opts)
}
