// workloadguard monitors macOS system health and enforces workload policies.
//
// It detects runaway processes, excessive system load, and other resource
// exhaustion conditions, then takes configured actions such as logging,
// notification, or process termination.
package main

import (
	"os"

	"github.com/plexusone/workloadguard/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
