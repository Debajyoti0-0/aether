package graph

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlanNode is one DAG node of a generated plan (mirrors the CLI
// plan JSON shape consumed by `aether run plan`).
type PlanNode struct {
	ID           string   `json:"id"`
	Cmd          string   `json:"cmd"`
	Dependencies []string `json:"dependencies,omitempty"`
	Fallback     string   `json:"fallback,omitempty"`
	Retries      int      `json:"retries,omitempty"`
	Critical     bool     `json:"critical,omitempty"`
}

// Plan is a generated workflow plan.
type Plan struct {
	MaxParallel int        `json:"max_parallel"`
	Nodes       []PlanNode `json:"nodes"`
}

// GeneratePlan converts an executable path into a DAG plan: each
// runbook step becomes a node chained on the previous step. With
// caeFallback, a CAE claims-handler recovery node is prepended and
// wired as the fallback of any auth-dependent node.
func GeneratePlan(run *ExecutablePath, caeFallback bool) (*Plan, error) {
	if len(run.Runbook) == 0 {
		return nil, fmt.Errorf("runbook has no steps")
	}

	plan := &Plan{MaxParallel: 2}

	if caeFallback {
		// Recovery node for CAE/token-revocation mid-chain.
		plan.Nodes = append(plan.Nodes, PlanNode{
			ID:       "cae-recovery",
			Cmd:      "relay cae-handler --refresh-token <rt> --tenant <tenant>",
			Critical: false,
		})
	}

	var prev string
	for i, cmd := range run.Runbook {
		id := fmt.Sprintf("step-%d", i+1)
		node := PlanNode{
			ID:      id,
			Cmd:     cmd,
			Retries: 1,
		}
		if prev != "" {
			node.Dependencies = []string{prev}
		} else {
			// The first step depends on cae-recovery being available
			// (it runs first and independently).
			if caeFallback {
				node.Dependencies = []string{"cae-recovery"}
			}
		}

		// Auth-shaped steps (token, relay, prt) get the CAE fallback.
		if caeFallback && isAuthStep(cmd) {
			node.Fallback = "cae-recovery"
		}

		plan.Nodes = append(plan.Nodes, node)
		prev = id
	}
	return plan, nil
}

// isAuthStep reports whether a command performs an authentication
// exchange (eligible for CAE recovery).
func isAuthStep(cmd string) bool {
	for _, kw := range []string{"prt convert", "token protect", "relay mfa", "relay cae", "relay fido2"} {
		if strings.Contains(cmd, kw) {
			return true
		}
	}
	return false
}

// GeneratePlanFromPath qualifies a node-ID path and generates a plan.
func (e *GraphEngine) GeneratePlanFromPath(pathIDs []string, caeFallback bool) (*Plan, error) {
	run, err := e.QualifyPath(pathIDs)
	if err != nil {
		return nil, err
	}
	return GeneratePlan(run, caeFallback)
}

// PlanJSON renders a plan as indented JSON.
func (p *Plan) PlanJSON() (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RenderPlanSummary prints the plan for terminal review.
func (p *Plan) RenderPlanSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Generated Plan: %d node(s), max_parallel=%d ===\n", len(p.Nodes), p.MaxParallel)
	for _, n := range p.Nodes {
		deps := "-"
		if len(n.Dependencies) > 0 {
			deps = strings.Join(n.Dependencies, ",")
		}
		fb := ""
		if n.Fallback != "" {
			fb = fmt.Sprintf(" fallback→%s", n.Fallback)
		}
		fmt.Fprintf(&b, "  %-14s deps=%-10s retries=%d%s\n    $ %s\n", n.ID, deps, n.Retries, fb, n.Cmd)
	}
	return b.String()
}
