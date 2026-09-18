package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
	"github.com/Debajyoti0-0/aether/pkg/plugins/gcp"
	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// newIntentFlagSet builds a quiet flag parser for intent strings.
func newIntentFlagSet(name string) *pflag.FlagSet {
	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// Governance helpers for Stage 1/2 (forensic F5/F7): every mutating
// command opens a workspace and executes through the Action spine, so
// each external effect produces signed audit entries, an evidence
// record, and a rollback record.

// govWorkspace is the shared --workspace flag for mutating commands.
var govWorkspace string

// openGovernedWorkspace resolves the workspace a mutating command must
// record into. Fail-closed: no workspace → no mutation.
func openGovernedWorkspace(name string) (*workspace.Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf(
			"this command mutates external state and requires --workspace: " +
				"audit and rollback records are mandatory (pass --workspace <name>; " +
				"passphrase via AETHER_PASSPHRASE)")
	}
	w, err := workspaceOpen(name)
	if err != nil {
		return nil, fmt.Errorf("open workspace %q (governance requires a passphrase-protected workspace): %w", name, err)
	}
	return w, nil
}

// cliMutation adapts CLI handlers to the spine.Mutation contract.
type cliMutation struct {
	kindV    string
	targetV  string
	beforeFn func() ([]byte, error)
	execFn   func(ctx context.Context) (string, error)
	afterFn  func() ([]byte, error)
	undoV    *mutation.UndoSpec
	riskV    int
}

func (m *cliMutation) Kind() string   { return m.kindV }
func (m *cliMutation) Target() string { return m.targetV }
func (m *cliMutation) BeforeState() ([]byte, error) {
	if m.beforeFn == nil {
		return nil, mutation.ErrStateUnavailable
	}
	return m.beforeFn()
}
func (m *cliMutation) Execute(ctx context.Context) (string, error) {
	return m.execFn(ctx)
}
func (m *cliMutation) AfterState() ([]byte, error) {
	if m.afterFn == nil {
		return nil, mutation.ErrStateUnavailable
	}
	return m.afterFn()
}
func (m *cliMutation) UndoRecipe() *mutation.UndoSpec { return m.undoV }

// irreversibleShell marks shell-style mutations with no safe reversal
// path (remote command execution, telemetry injection, CI dispatch).
func irreversibleShell() *mutation.UndoSpec {
	return &mutation.UndoSpec{Irreversible: true, Detail: "remote command execution has no safe automatic reversal"}
}

// runIntent executes a command-string intent through the Action spine.
// This is the ONLY execution path for DAG plan nodes and rollback undo
// commands (Stage 2, forensic F7): the CLI root is never re-entered.
//
// The intent grammar is a strict whitelist of mutating command shapes.
// Anything else fails closed — a plan/undo command that the spine
// cannot represent is refused, never silently executed.
func runIntent(ctx context.Context, ws *workspace.Workspace, command string, actor string) (*spine.Result, error) {
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(command), "aether "))
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty intent")
	}
	if fields[0] == "aether" {
		fields = fields[1:]
	}
	if len(fields) < 2 {
		return nil, fmt.Errorf("intent %q: want a whitelisted mutating command (exec azure|aws|github|gcp, simulate stream)", strings.Join(fields, " "))
	}

	mut, kind, target, err := buildIntentMutation(fields)
	if err != nil {
		return nil, fmt.Errorf("intent %q refused: %w", strings.Join(fields, " "), err)
	}

	s := spine.New(ws)
	// Charter fix (M1): the estimator's riskV was minted but never mapped
	// onto the Action — the spine's risk stage ran against 0 for every
	// intent-driven action, so the operator ceiling never fired.
	estimated := 50
	if mc, ok := mut.(*cliMutation); ok {
		estimated = mc.riskV
	}
	return s.Run(ctx, &spine.Action{
		Kind:         kind,
		Target:       target,
		Actor:        actor,
		Mutation:     mut,
		RiskScore:    estimated,
		ApprovalMode: spine.ApprovalAuto,
	})
}

// mutatingIntents lists the command shapes that MUST go through the
// spine wherever they appear (plans, undo commands, runbooks).
//
// Charter fix (M1): narrow the whitelist to exactly what buildIntentMutation
// implements. The previous entries (simulate stream, plugins install,
// prt import, pivot cloud-to-onprem) were declared but refused downstream
// with "unsupported action kind" — intents the spine cannot represent must
// fail closed here, not after the spine is entered.
var mutatingIntents = [][2]string{
	{"exec", "azure"}, {"exec", "aws"}, {"exec", "github"}, {"exec", "gcp"},
}

// isMutatingIntent reports whether a parsed command line is a mutating
// intent (and therefore spine-mandatory).
func isMutatingIntent(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	for _, m := range mutatingIntents {
		if fields[0] == m[0] && fields[1] == m[1] {
			return true
		}
	}
	return false
}

// runIntentOrDispatch executes a command line: mutating intents go
// through the Action spine; everything else (read-only analysis
// commands: validate, cap, graph, prt convert, relay) dispatches
// through the CLI root — they perform no external mutation, so the
// spine's mutation governance does not apply to them.
func runIntentOrDispatch(ctx context.Context, ws *workspace.Workspace, line string) error {
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "aether ")))
	if isMutatingIntent(fields) {
		_, err := runIntent(ctx, ws, line, "intent")
		return err
	}
	root := NewRootCommand()
	root.SetArgs(fields)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root.Execute()
}

// buildIntentMutation parses the whitelisted intent grammar into a
// spine mutation. Flags mirror the corresponding CLI commands.
func buildIntentMutation(fields []string) (mutation.Mutation, string, string, error) {
	switch fields[0] + " " + fields[1] {
	case "exec azure":
		fs := newIntentFlagSet("exec azure")
		var token, subID, group, vmID, cmdStr, preset string
		var timeout int
		fs.StringVar(&token, "token", "", "")
		fs.StringVar(&subID, "subscription-id", "", "")
		fs.StringVar(&group, "resource-group", "", "")
		fs.StringVar(&vmID, "vm-id", "", "")
		fs.StringVar(&cmdStr, "cmd", "", "")
		fs.StringVar(&preset, "browser-preset", "chrome", "")
		fs.IntVar(&timeout, "timeout", 300, "")
		if err := fs.Parse(fields[2:]); err != nil {
			return nil, "", "", err
		}
		if token == "" || subID == "" || group == "" || vmID == "" || cmdStr == "" {
			return nil, "", "", fmt.Errorf("exec azure requires --token, --subscription-id, --resource-group, --vm-id, --cmd")
		}
		hc, err := transport.NewClient(preset, time.Duration(timeout)*time.Second)
		if err != nil {
			return nil, "", "", err
		}
		e := exec.NewAzureExecutor(subID, token, hc)
		return &cliMutation{
			kindV:   "exec.azure",
			targetV: fmt.Sprintf("%s/%s (sub %s)", group, vmID, subID),
			undoV:   irreversibleShell(),
			riskV:   70,
			execFn: func(ctx context.Context) (string, error) {
				result, err := e.ExecuteOnAzureVM(ctx, group, vmID, cmdStr)
				if err != nil {
					return "", err
				}
				printResult(result)
				return "", nil
			},
		}, "exec.azure", fmt.Sprintf("%s/%s (sub %s)", group, vmID, subID), nil

	case "exec aws":
		fs := newIntentFlagSet("exec aws")
		var ak, sk, sessTok, region, instance, cmdStr string
		var timeout int
		fs.StringVar(&ak, "access-key", "", "")
		fs.StringVar(&sk, "secret-key", "", "")
		fs.StringVar(&sessTok, "session-token", "", "")
		fs.StringVar(&region, "region", "us-east-1", "")
		fs.StringVar(&instance, "instance-id", "", "")
		fs.StringVar(&cmdStr, "cmd", "", "")
		fs.IntVar(&timeout, "timeout", 300, "")
		if err := fs.Parse(fields[2:]); err != nil {
			return nil, "", "", err
		}
		if ak == "" || sk == "" || instance == "" || cmdStr == "" {
			return nil, "", "", fmt.Errorf("exec aws requires --access-key, --secret-key, --instance-id, --cmd")
		}
		e := exec.NewAWSExecutor(region, ak, sk, sessTok, nil)
		return &cliMutation{
			kindV:   "exec.aws",
			targetV: fmt.Sprintf("%s@%s", instance, region),
			undoV:   irreversibleShell(),
			riskV:   70,
			execFn: func(ctx context.Context) (string, error) {
				result, err := e.ExecuteOnEC2(ctx, instance, cmdStr)
				if err != nil {
					return "", err
				}
				printResult(result)
				return "", nil
			},
		}, "exec.aws", fmt.Sprintf("%s@%s", instance, region), nil

	case "exec github":
		fs := newIntentFlagSet("exec github")
		var token, repo, workflow, ref, preset string
		var timeout int
		fs.StringVar(&token, "token", "", "")
		fs.StringVar(&repo, "repo", "", "")
		fs.StringVar(&workflow, "workflow", "", "")
		fs.StringVar(&ref, "ref", "main", "")
		fs.StringVar(&preset, "browser-preset", "chrome", "")
		fs.IntVar(&timeout, "timeout", 60, "")
		if err := fs.Parse(fields[2:]); err != nil {
			return nil, "", "", err
		}
		if token == "" || repo == "" || workflow == "" {
			return nil, "", "", fmt.Errorf("exec github requires --token, --repo, --workflow")
		}
		hc, err := transport.NewClient(preset, time.Duration(timeout)*time.Second)
		if err != nil {
			return nil, "", "", err
		}
		e := exec.NewGitHubExecutor(token, hc)
		return &cliMutation{
			kindV:   "exec.github",
			targetV: fmt.Sprintf("%s @%s (%s)", repo, ref, workflow),
			undoV:   irreversibleShell(),
			riskV:   60,
			execFn: func(ctx context.Context) (string, error) {
				result, err := e.ExecuteOnRunner(ctx, repo, workflow, ref, nil)
				if err != nil {
					return "", err
				}
				printResult(result)
				return "", nil
			},
		}, "exec.github", fmt.Sprintf("%s @%s (%s)", repo, ref, workflow), nil

	case "exec gcp":
		fs := newIntentFlagSet("exec gcp")
		var project, token, target, cmdStr string
		fs.StringVar(&project, "project", "", "")
		fs.StringVar(&token, "token", "", "")
		fs.StringVar(&target, "target", "", "")
		fs.StringVar(&cmdStr, "cmd", "", "")
		if err := fs.Parse(fields[2:]); err != nil {
			return nil, "", "", err
		}
		if project == "" || token == "" || target == "" || cmdStr == "" {
			return nil, "", "", fmt.Errorf("exec gcp requires --project, --token, --target, --cmd")
		}
		prov := gcp.New(project, token)
		provider, ok := any(prov).(sdk.Provider)
		if !ok {
			return nil, "", "", fmt.Errorf("gcp plugin does not implement sdk.Provider")
		}
		return &cliMutation{
			kindV:   "exec.gcp",
			targetV: project + "/" + target,
			undoV:   irreversibleShell(),
			riskV:   60,
			execFn: func(ctx context.Context) (string, error) {
				res, err := provider.Execute(ctx, target, cmdStr)
				if err != nil {
					return "", err
				}
				fmt.Printf("Status:  %s\nExit:    %d\nOutput:  %s\n", res.Status, res.ExitCode, res.Output)
				return "", nil
			},
		}, "exec.gcp", project+"/"+target, nil

	default:
		return nil, "", "", fmt.Errorf(
			"unsupported action kind %q — the spine whitelist covers: exec azure, exec aws, exec github, exec gcp",
			strings.Join(fields, " "))
	}
}
