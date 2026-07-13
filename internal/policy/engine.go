// Package policy implements the workloadguard policy engine using Cedar.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
)

// Engine evaluates workload policies using Cedar.
type Engine struct {
	policies  map[string]config.Policy
	policySet *cedar.PolicySet
	logger    *slog.Logger
}

// Decision represents the result of evaluating a policy.
type Decision struct {
	PolicyName  string   `json:"policy_name"`
	Triggered   bool     `json:"triggered"`
	Reason      string   `json:"reason"`
	Actions     []string `json:"actions"`
	PIDs        []int    `json:"pids,omitempty"`
	ProcessName string   `json:"process_name,omitempty"`
}

// NewEngine creates a new policy engine.
func NewEngine(policies map[string]config.Policy, logger *slog.Logger) (*Engine, error) {
	policySet, err := convertToCedar(policies)
	if err != nil {
		return nil, fmt.Errorf("convert policies to cedar: %w", err)
	}

	return &Engine{
		policies:  policies,
		policySet: policySet,
		logger:    logger,
	}, nil
}

// Evaluate evaluates all policies against the given snapshot.
func (e *Engine) Evaluate(ctx context.Context, snapshot *collector.Snapshot) []Decision {
	decisions := make([]Decision, 0, len(e.policies))

	// Sort policies by priority (higher first).
	names := make([]string, 0, len(e.policies))
	for name := range e.policies {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return e.policies[names[i]].Priority > e.policies[names[j]].Priority
	})

	for _, name := range names {
		policy := e.policies[name]

		if !policy.IsEnabled() {
			continue
		}

		decision := e.evaluatePolicy(ctx, name, policy, snapshot)
		decisions = append(decisions, decision)
	}

	return decisions
}

func (e *Engine) evaluatePolicy(
	ctx context.Context,
	name string,
	policy config.Policy,
	snapshot *collector.Snapshot,
) Decision {
	decision := Decision{
		PolicyName:  name,
		Actions:     policy.Actions,
		ProcessName: policy.Process,
	}

	// Get processes matching the policy.
	processes := snapshot.ProcessesByName(policy.Process)
	count := len(processes)

	// Apply parent filtering if specified.
	if policy.Conditions != nil {
		if policy.Conditions.ParentProcess != "" {
			// Filter to only processes with the specified parent.
			var filtered []collector.ProcessInfo
			for _, p := range processes {
				if p.ParentName == policy.Conditions.ParentProcess {
					filtered = append(filtered, p)
				}
			}
			processes = filtered
			count = len(processes)

			e.logger.Debug("filtered by parent",
				"policy", name,
				"parent", policy.Conditions.ParentProcess,
				"count", count,
			)
		}

		if len(policy.Conditions.ExcludeParents) > 0 {
			// Exclude processes with specified parents.
			processes = snapshot.ExcludeByParents(processes, policy.Conditions.ExcludeParents)
			count = len(processes)

			e.logger.Debug("excluded parents",
				"policy", name,
				"excluded", policy.Conditions.ExcludeParents,
				"count", count,
			)
		}
	}

	// Check process count threshold.
	if count < policy.MaxCount {
		decision.Reason = fmt.Sprintf(
			"process count %d below threshold %d",
			count,
			policy.MaxCount,
		)
		return decision
	}

	// Check additional conditions.
	if policy.Conditions != nil {
		if policy.Conditions.MinLoad1 > 0 &&
			snapshot.LoadAverage.Load1 < policy.Conditions.MinLoad1 {
			decision.Reason = fmt.Sprintf(
				"load1 %.2f below condition %.2f",
				snapshot.LoadAverage.Load1,
				policy.Conditions.MinLoad1,
			)
			return decision
		}

		if policy.Conditions.MinLoad5 > 0 &&
			snapshot.LoadAverage.Load5 < policy.Conditions.MinLoad5 {
			decision.Reason = fmt.Sprintf(
				"load5 %.2f below condition %.2f",
				snapshot.LoadAverage.Load5,
				policy.Conditions.MinLoad5,
			)
			return decision
		}
	}

	// Use Cedar for final authorization decision.
	authorized := e.authorizeWithCedar(ctx, name, policy, snapshot, count)
	if !authorized {
		decision.Reason = "cedar policy denied action"
		return decision
	}

	// Policy triggered.
	decision.Triggered = true
	decision.Reason = fmt.Sprintf(
		"process %s count %d >= threshold %d",
		policy.Process,
		count,
		policy.MaxCount,
	)

	// Collect PIDs for termination.
	for _, p := range processes {
		decision.PIDs = append(decision.PIDs, p.PID)
	}

	return decision
}

func (e *Engine) authorizeWithCedar(
	ctx context.Context,
	policyName string,
	policy config.Policy,
	snapshot *collector.Snapshot,
	count int,
) bool {
	// Build Cedar request.
	principal := types.NewEntityUID("WorkloadGuard", types.String("daemon"))
	action := types.NewEntityUID("Action", types.String("terminate"))
	resource := types.NewEntityUID("Process", types.String(policy.Process))

	// Build context with snapshot data.
	cedarContext := types.NewRecord(types.RecordMap{
		"process_name":  types.String(policy.Process),
		"process_count": types.Long(count),
		"max_count":     types.Long(policy.MaxCount),
		"load1":         types.Long(int64(snapshot.LoadAverage.Load1 * 100)),
		"load5":         types.Long(int64(snapshot.LoadAverage.Load5 * 100)),
		"load15":        types.Long(int64(snapshot.LoadAverage.Load15 * 100)),
		"cpu_count":     types.Long(snapshot.CPUCount),
	})

	decision, _ := e.policySet.IsAuthorized(
		cedar.Entities{},
		cedar.Request{
			Principal: principal,
			Action:    action,
			Resource:  resource,
			Context:   cedarContext,
		},
	)

	return decision == cedar.Allow
}

// Policies returns the configured policies.
func (e *Engine) Policies() map[string]config.Policy {
	return e.policies
}

// PolicySet returns the Cedar policy set.
func (e *Engine) PolicySet() *cedar.PolicySet {
	return e.policySet
}
