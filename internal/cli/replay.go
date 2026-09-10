package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
)

// replayCmd — replay past operations (dry-run by default).
var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay past operations from a runbook (dry-run by default)",
	Long: `Replay a saved operation file — a graph runbook, a run report, or a
hand-written list of aether commands. Dry-run mode prints what would
execute and performs NO network or destructive actions. Pass --confirm
to actually execute each step.`,
	RunE: runReplay,
}

var (
	replayFile    string
	replayConfirm bool
	replayFilter  string
)

func init() {
	rootCmd.AddCommand(replayCmd)
	replayCmd.Flags().StringVar(&replayFile, "file", "", "Operation file: runbook JSON, or one command per line (required)")
	replayCmd.Flags().BoolVar(&replayConfirm, "confirm", false, "Actually execute the steps (default: dry-run)")
	replayCmd.Flags().StringVar(&replayFilter, "filter", "", "Only replay steps containing this substring")
	_ = replayCmd.MarkFlagRequired("file")
}

func runReplay(cmd *cobra.Command, args []string) error {
	steps, err := loadRunbook(replayFile)
	if err != nil {
		return err
	}

	if replayFilter != "" {
		var filtered []string
		for _, s := range steps {
			if strings.Contains(s, replayFilter) {
				filtered = append(filtered, s)
			}
		}
		steps = filtered
	}
	if len(steps) == 0 {
		return fmt.Errorf("no steps to replay")
	}

	mode := "DRY-RUN (no execution)"
	if replayConfirm {
		mode = "EXECUTE"
	}
	fmt.Printf("=== Replay: %s — %d step(s) [%s] ===\n\n", replayFile, len(steps), mode)

	for i, step := range steps {
		fmt.Printf("%d. %s\n", i+1, step)
	}

	if !replayConfirm {
		fmt.Println("\nDry-run complete. Re-run with --confirm to execute.")
		return nil
	}

	// Confirmed execution: each step must be an aether subcommand.
	fmt.Println()
	for i, step := range steps {
		fmt.Printf(">>> Executing step %d: %s\n", i+1, step)
		if err := executeAetherLine(step); err != nil {
			fmt.Fprintf(os.Stderr, "    step %d failed: %v\n", i+1, err)
			if !askContinue() {
				return fmt.Errorf("replay aborted at step %d", i+1)
			}
		}
	}
	fmt.Println("\nReplay finished.")
	return nil
}

// loadRunbook accepts three formats: a graph runbook export
// ({"runbook": [...]}), a correlated-path export, or a plain text file
// with one command per line.
func loadRunbook(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read runbook: %w", err)
	}

	// Try runbook-shaped JSON first.
	var doc struct {
		Runbook []string `json:"runbook"`
	}
	if json.Unmarshal(data, &doc) == nil && len(doc.Runbook) > 0 {
		return doc.Runbook, nil
	}

	// Plain text: one command per line, '#' comments skipped.
	var steps []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		steps = append(steps, line)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps found in %s", path)
	}
	return steps, nil
}

// executeAetherLine dispatches one "aether <args>" line. Stage 2: any
// mutating intent executes through the Action spine; read-only
// analysis commands dispatch through the CLI root.
func executeAetherLine(line string) error {
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "aether ")))
	if len(fields) == 0 {
		return fmt.Errorf("empty command")
	}
	if isMutatingIntent(fields) {
		ws, err := openGovernedWorkspace(execWorkspace)
		if err != nil {
			return err
		}
		_, err = runIntent(context.Background(), ws, line, "replay")
		return err
	}

	root := NewRootCommand()
	root.SetArgs(fields)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return root.Execute()
}

// askContinue prompts the operator after a failed step.
func askContinue() bool {
	fmt.Print("Continue? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// saveRunbookCmd writes a graph runbook to a replayable file.
var saveRunbookCmd = &cobra.Command{
	Use:   "save",
	Short: "Save a graph path's runbook as a replayable operation file",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(saveRunbookGraph)
		if err != nil {
			return err
		}
		engine := graph.NewGraphEngine(g, saveRunbookRisk)

		var pathIDs []string
		for _, id := range splitComma(saveRunbookPath) {
			pathIDs = append(pathIDs, id)
		}
		run, err := engine.QualifyPath(pathIDs)
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(run, "", "  ")
		if err != nil {
			return err
		}
		if saveRunbookOut == "" {
			saveRunbookOut = "operation.json"
		}
		if err := os.WriteFile(saveRunbookOut, data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Runbook saved to %s (%d steps)\n", saveRunbookOut, len(run.Runbook))
		return nil
	},
}

var (
	saveRunbookGraph string
	saveRunbookPath  string
	saveRunbookOut   string
	saveRunbookRisk  int
)

func init() {
	replayCmd.AddCommand(saveRunbookCmd)
	saveRunbookCmd.Flags().StringVar(&saveRunbookGraph, "graph", "", "Graph JSON file (required)")
	saveRunbookCmd.Flags().StringVar(&saveRunbookPath, "path", "", "Comma-separated node IDs (required)")
	saveRunbookCmd.Flags().StringVar(&saveRunbookOut, "output", "operation.json", "Output operation file")
	saveRunbookCmd.Flags().IntVar(&saveRunbookRisk, "risk-threshold", 60, "Risk ceiling")
	_ = saveRunbookCmd.MarkFlagRequired("graph")
	_ = saveRunbookCmd.MarkFlagRequired("path")
}
