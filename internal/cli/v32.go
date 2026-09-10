package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/rl"
	"github.com/Debajyoti0-0/aether/internal/paths"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// planTrainCmd â€” offline Q-learning from exported episodes.
var planTrainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train the RL planner from exported episodes",
	Long: `Train the tabular Q-learning agent on episode JSONL files
(exported via 'aether plan export' or hand-authored). The resulting
policy JSON is consumed by 'aether plan generate --rl --policy'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := rl.NewEpisodeStore(trainEpisodes)
		episodes, err := s.Load()
		if err != nil {
			return err
		}
		if len(episodes) == 0 {
			return fmt.Errorf("no episodes in %s", trainEpisodes)
		}

		h := rl.DefaultHyperParams()
		if trainLR > 0 {
			h.LearningRate = trainLR
		}
		if trainEpsilon > 0 {
			h.Epsilon = trainEpsilon
		}

		agent, report, err := rl.TrainAgent(episodes, h, trainEpochs)
		if err != nil {
			return err
		}

		if trainOut == "" {
			trainOut = "policy.json"
		}
		if err := agent.SavePolicy(trainOut); err != nil {
			return err
		}
		fmt.Print(report)
		fmt.Printf("Policy saved to %s\n", trainOut)
		fmt.Print(agent.RenderPolicy())
		return nil
	},
}

var (
	trainEpisodes string
	trainEpochs   int
	trainLR       float64
	trainEpsilon  float64
	trainOut      string
)

// planExportCmd â€” export workspace operations as RL episodes. Stage 3
// (T5): the default destination is the workspace vault; --output keeps
// the JSONL interchange format for cross-machine training sets.
// A legacy <workspace>-episodes.jsonl in the working directory is
// imported into the vault and preserved (idempotent migration).
var planExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export workspace operations as RL training episodes (vault; --output for JSONL)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceOpen(expWorkspace)
		if err != nil {
			return err
		}

		// Legacy migration: import a sibling JSONL if present.
		legacyPath := ws.Name + "-episodes.jsonl"
		if data, err := os.ReadFile(legacyPath); err == nil {
			legacy := rl.NewEpisodeStoreBytes(data)
			episodes, err := legacy.Load()
			if err != nil {
				return fmt.Errorf("parse legacy %s: %w", legacyPath, err)
			}
			vs := rl.NewVaultEpisodeStore(ws.Vault())
			for _, ep := range episodes {
				if err := vs.Append(ep); err != nil {
					return err
				}
			}
			if err := os.Rename(legacyPath, legacyPath+".pre-vault-imported"); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Migrated legacy %s into the workspace vault (%d episodes)\n", legacyPath, len(episodes))
		}

		events, err := ws.Events()
		if err != nil {
			return err
		}

		var journal []rl.JournalEvent
		for _, ev := range events {
			journal = append(journal, rl.JournalEvent{Kind: ev.Kind, Detail: ev.Detail})
		}
		if len(journal) == 0 {
			return fmt.Errorf("workspace journal is empty â€” nothing to export")
		}

		ep := rl.BuildEpisode(ws.Name, journal)
		if len(ep.Steps) == 0 {
			return fmt.Errorf("journal events mapped to no known actions")
		}

		if expOut != "" {
			s := rl.NewEpisodeStore(expOut)
			if err := s.Append(ep); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Exported 1 episode (%d steps) to %s\n", len(ep.Steps), expOut)
			return nil
		}

		vs := rl.NewVaultEpisodeStore(ws.Vault())
		if err := vs.Append(ep); err != nil {
			return err
		}
		n, _ := vs.Count()
		fmt.Fprintf(os.Stderr, "Exported 1 episode (%d steps) into workspace %q vault (%d episodes total)\n", len(ep.Steps), ws.Name, n)
		return nil
	},
}

var (
	expWorkspace string
	expOut       string
)

// planGenerateRLCmd â€” policy-driven plan generation.
var planGenerateRLCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a plan using the trained RL policy",
	Long: `Walk the learned policy from a start state to produce a DAG plan.
The start state is built from the workspace's current posture (stored
tokens) and an optional graph file (density). Output is a plan JSON
consumable by 'aether run plan'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, err := rl.LoadPolicy(rlPolicy)
		if err != nil {
			return err
		}

		// Discretize the start state from workspace + graph inputs.
		start := rl.State{
			TokenBucket:   0,
			GraphDensity:  "low",
			CAPStrictness: "medium",
			Phase:         "recon",
		}
		if rlGraphFile != "" {
			if g, err := graph.LoadGraph(rlGraphFile); err == nil {
				start.GraphDensity = graphDensityFromCounts(len(g.Nodes), len(g.Edges))
			}
		}
		if rlWorkspace != "" && workspaceExists(rlWorkspace) {
			if w, err := workspaceOpen(rlWorkspace); err == nil {
				var tokens []map[string]any
				if err := w.LoadRecord(workspace.BucketTokens, "oauth", &tokens); err == nil && len(tokens) > 0 {
					start.TokenBucket = rl.BucketTokens(len(tokens))
				}
			}
		}

		nodes, err := rl.GenerateRLPlan(agent, start, rlMaxSteps)
		if err != nil {
			return err
		}

		plan := struct {
			MaxParallel int                `json:"max_parallel"`
			Nodes       []rl.PlanNode `json:"nodes"`
		}{MaxParallel: rlParallel, Nodes: nodes}

		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		if rlOut != "" {
			if err := os.WriteFile(rlOut, data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "RL plan written to %s\n", rlOut)
		}
		fmt.Println(string(data))
		return nil
	},
}

var (
	rlPolicy    string
	rlGraphFile string
	rlWorkspace string
	rlOut       string
	rlMaxSteps  int
	rlParallel  int
)

// workspaceExists is a local helper (workspace.Exists without import
// churn in this file).
func workspaceExists(name string) bool {
	if name == "" {
		return false
	}
	dir := filepath.Join(paths.WorkspacesDir(), name)
	_, err := os.Stat(dir)
	return err == nil
}

// graphDensityFromCounts maps counts to a density label (mirrors
// rl.DensityFromCounts without a planner import cycle).
func graphDensityFromCounts(nodes, edges int) string {
	if nodes == 0 {
		return "low"
	}
	ratio := float64(edges) / float64(nodes)
	switch {
	case ratio < 0.5:
		return "low"
	case ratio < 2:
		return "medium"
	default:
		return "high"
	}
}

// planCmd â€” RL planner operations.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "RL planner: train, export episodes, generate adaptive plans",
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planTrainCmd, planExportCmd, planGenerateRLCmd)

	planTrainCmd.Flags().StringVar(&trainEpisodes, "episodes", "", "Episodes JSONL file (required)")
	planTrainCmd.Flags().IntVar(&trainEpochs, "epochs", 50, "Training epochs")
	planTrainCmd.Flags().Float64Var(&trainLR, "learning-rate", 0.1, "Q-learning alpha")
	planTrainCmd.Flags().Float64Var(&trainEpsilon, "epsilon", 0.3, "Initial exploration rate")
	planTrainCmd.Flags().StringVar(&trainOut, "output", "policy.json", "Output policy JSON")
	_ = planTrainCmd.MarkFlagRequired("episodes")

	planExportCmd.Flags().StringVar(&expWorkspace, "workspace", "", "Workspace name (required)")
	planExportCmd.Flags().StringVar(&expOut, "output", "", "Output episodes JSONL file")
	_ = planExportCmd.MarkFlagRequired("workspace")

	planGenerateRLCmd.Flags().StringVar(&rlPolicy, "policy", "", "Trained policy JSON (required)")
	planGenerateRLCmd.Flags().StringVar(&rlGraphFile, "graph", "", "Graph JSON (for density discretization)")
	planGenerateRLCmd.Flags().StringVar(&rlWorkspace, "workspace", "", "Workspace (for token-count discretization)")
	planGenerateRLCmd.Flags().StringVar(&rlOut, "output", "", "Write plan JSON to file")
	planGenerateRLCmd.Flags().IntVar(&rlMaxSteps, "max-steps", 8, "Max plan steps")
	planGenerateRLCmd.Flags().IntVar(&rlParallel, "parallel", 2, "Plan max_parallel")
	_ = planGenerateRLCmd.MarkFlagRequired("policy")
}
