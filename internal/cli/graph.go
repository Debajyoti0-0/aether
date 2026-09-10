package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/transport"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Cross-provider identity graph",
	Long:  "Build and execute a live identity graph across Entra ID, AWS, and GCP.",
}

var graphBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the identity graph from exports or live Graph",
	RunE:  runGraphBuild,
}

var graphQualifyCmd = &cobra.Command{
	Use:   "qualify",
	Short: "Qualify a node path into an executable runbook",
	RunE:  runGraphQualify,
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show graph statistics",
	RunE:  runGraphStats,
}

var (
	graphToken    string
	graphAPIBase  string
	graphOut      string
	graphInput    string
	graphInputs   []string
	graphKinds    []string
	graphPath     string
	graphMaxRisk  int
	graphPreset   string
	graphTimeout  int
)

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphBuildCmd, graphQualifyCmd, graphStatsCmd)

	graphBuildCmd.Flags().StringVar(&graphToken, "token", "", "Graph token for live fetch")
	graphBuildCmd.Flags().StringVar(&graphAPIBase, "graph-base", "https://graph.microsoft.com/v1.0", "Graph base URL")
	graphBuildCmd.Flags().StringSliceVar(&graphInputs, "input", nil, "Input JSON export file(s)")
	graphBuildCmd.Flags().StringSliceVar(&graphKinds, "kind", nil, "Kind per input: entra:users, entra:servicePrincipals, entra:roleAssignments, aws, gcp")
	graphBuildCmd.Flags().StringVar(&graphOut, "output", "aether-graph.json", "Output graph file")
	graphBuildCmd.Flags().StringVar(&graphPreset, "browser-preset", "chrome", "TLS preset")
	graphBuildCmd.Flags().IntVar(&graphTimeout, "timeout", 30, "Timeout seconds")

	graphQualifyCmd.Flags().StringVar(&graphInput, "graph", "", "Graph JSON file (required)")
	graphQualifyCmd.Flags().StringVar(&graphPath, "path", "", "Comma-separated node IDs (required)")
	graphQualifyCmd.Flags().IntVar(&graphMaxRisk, "risk-threshold", 60, "Risk ceiling")
	_ = graphQualifyCmd.MarkFlagRequired("graph")
	_ = graphQualifyCmd.MarkFlagRequired("path")

	graphStatsCmd.Flags().StringVar(&graphInput, "graph", "", "Graph JSON file (required)")
	_ = graphStatsCmd.MarkFlagRequired("graph")
}

func runGraphBuild(cmd *cobra.Command, args []string) error {
	g := &graph.IdentityGraph{}

	// Live fetch takes precedence when a token is provided.
	if graphToken != "" {
		hc, err := transport.NewClient(graphPreset, time.Duration(graphTimeout)*time.Second)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(graphTimeout)*time.Second)
		defer cancel()
		live, err := graph.FetchEntra(ctx, hc, graphToken, graphAPIBase)
		if err != nil {
			return err
		}
		g.Merge(live)
	}

	// Merge file exports.
	for i, file := range graphInputs {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		kind := "aws"
		if i < len(graphKinds) {
			kind = graphKinds[i]
		}

		switch {
		case kind == "aws":
			if err := g.IngestAWSJSON(data); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
		case kind == "gcp":
			if err := g.IngestGCPJSON(data); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
		case len(kind) > 6 && kind[:6] == "entra:":
			if err := g.IngestEntraJSON(kind[6:], data); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
		default:
			return fmt.Errorf("unknown kind %q for %s", kind, file)
		}
	}

	if len(g.Nodes) == 0 {
		return fmt.Errorf("graph is empty: provide --token or --input files")
	}

	if err := g.Save(graphOut); err != nil {
		return err
	}
	fmt.Printf("Graph saved to %s\n%s", graphOut, g.Stats())
	return nil
}

func runGraphQualify(cmd *cobra.Command, args []string) error {
	g, err := graph.LoadGraph(graphInput)
	if err != nil {
		return err
	}

	engine := graph.NewGraphEngine(g, graphMaxRisk)

	var pathIDs []string
	for _, p := range splitComma(graphPath) {
		pathIDs = append(pathIDs, p)
	}

	run, err := engine.QualifyPath(pathIDs)
	if err != nil {
		return err
	}
	fmt.Print(graph.RenderRunbook(run))
	return nil
}

func runGraphStats(cmd *cobra.Command, args []string) error {
	g, err := graph.LoadGraph(graphInput)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]int{"nodes": len(g.Nodes), "edges": len(g.Edges)}); err != nil {
		return err
	}
	fmt.Print(g.Stats())
	return nil
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' || r == '>' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
