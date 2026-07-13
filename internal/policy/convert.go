package policy

import (
	"fmt"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/plexusone/workloadguard/internal/config"
)

// convertToCedar converts TOML policies to a Cedar PolicySet.
func convertToCedar(policies map[string]config.Policy) (*cedar.PolicySet, error) {
	var cedarPolicies []string

	for name, policy := range policies {
		if !policy.IsEnabled() {
			continue
		}

		cedarPolicy := generateCedarPolicy(name, policy)
		cedarPolicies = append(cedarPolicies, cedarPolicy)
	}

	// Add default permit policy for threshold checks.
	// The actual threshold logic is in Go; Cedar authorizes the action.
	defaultPolicy := `
@id("default-permit")
permit (
    principal == WorkloadGuard::"daemon",
    action == Action::"terminate",
    resource
);
`
	cedarPolicies = append(cedarPolicies, defaultPolicy)

	policyText := strings.Join(cedarPolicies, "\n")

	policySet, err := cedar.NewPolicySetFromBytes("policies.cedar", []byte(policyText))
	if err != nil {
		return nil, fmt.Errorf("parse cedar policies: %w", err)
	}

	return policySet, nil
}

// generateCedarPolicy generates a Cedar policy from a config policy.
func generateCedarPolicy(name string, policy config.Policy) string {
	var conditions []string

	// Add process name condition.
	conditions = append(conditions,
		fmt.Sprintf(`context.process_name == "%s"`, policy.Process),
	)

	// Add count threshold condition.
	conditions = append(conditions,
		fmt.Sprintf(`context.process_count >= %d`, policy.MaxCount),
	)

	// Add load conditions if specified.
	if policy.Conditions != nil {
		if policy.Conditions.MinLoad1 > 0 {
			// Load is stored as percentage * 100 to avoid float in Cedar.
			minLoad := int64(policy.Conditions.MinLoad1 * 100)
			conditions = append(conditions,
				fmt.Sprintf(`context.load1 >= %d`, minLoad),
			)
		}

		if policy.Conditions.MinLoad5 > 0 {
			minLoad := int64(policy.Conditions.MinLoad5 * 100)
			conditions = append(conditions,
				fmt.Sprintf(`context.load5 >= %d`, minLoad),
			)
		}
	}

	whenClause := strings.Join(conditions, " &&\n        ")

	return fmt.Sprintf(`
@id("%s")
permit (
    principal == WorkloadGuard::"daemon",
    action == Action::"terminate",
    resource == Process::"%s"
)
when {
    %s
};
`, name, policy.Process, whenClause)
}

// PolicyToCedar converts a single policy to Cedar text for debugging.
func PolicyToCedar(name string, policy config.Policy) string {
	return generateCedarPolicy(name, policy)
}

// GenerateCedarSchema generates a Cedar schema for workloadguard entities.
func GenerateCedarSchema() string {
	return `
namespace WorkloadGuard {
    entity Daemon;
}

namespace Action {
    entity Terminate;
    entity Notify;
    entity Log;
    entity Diagnose;
}

namespace Process {
    entity rg;
    entity node;
    entity git;
    entity python;
    entity ruby;
}

// Context attributes available during policy evaluation
type Context = {
    process_name: String,
    process_count: Long,
    max_count: Long,
    load1: Long,      // load * 100 (e.g., 250 = 2.50)
    load5: Long,
    load15: Long,
    cpu_count: Long,
};
`
}

// ValidateCedarPolicies validates that all policies can be converted to Cedar.
func ValidateCedarPolicies(policies map[string]config.Policy) error {
	_, err := convertToCedar(policies)
	return err
}

// PolicySetFromConfig creates a Cedar PolicySet from config policies.
func PolicySetFromConfig(policies map[string]config.Policy) (*cedar.PolicySet, error) {
	return convertToCedar(policies)
}
