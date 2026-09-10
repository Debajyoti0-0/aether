package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Cloud command execution (governed: audit + rollback recorded)",
	Long:  "Execute commands on cloud VMs and CI runners with valid tokens (authorized testing only). Every execution is written to the signed audit chain and rollback stack of the --workspace it names.",
}

var execAzureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Azure VM RunCommand",
	RunE:  runExecAzure,
}

var execAWSCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS EC2 SSM RunCommand",
	RunE:  runExecAWS,
}

var execIMDSCmd = &cobra.Command{
	Use:   "imds",
	Short: "Azure IMDS managed identity exploitation (from a compromised VM)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Shares the runner with `aether pivot imds`.
		return runIMDS(context.Background(), execIMDSResource, execIMDSClientID, execIMDSObjectID, execIMDSOut, execIMDSTimeout)
	},
}

var execGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub Actions workflow dispatch",
	RunE:  runExecGitHub,
}

var (
	execToken     string
	execVMID      string
	execCmdStr    string
	execGroup     string
	execSubID     string
	execInstance  string
	execRegion    string
	execAK        string
	execSK        string
	execSessTok   string
	execRepo      string
	execWorkflow  string
	execRef       string
	execTimeout   int
	execPreset    string
	execWorkspace string
)

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.AddCommand(execAzureCmd, execAWSCmd, execGitHubCmd, execIMDSCmd)

	// exec imds reuses pivot imds flags (registered there), but the
	// flags live on pivotIMDSCmd; register on this command too via alias.
	execIMDSCmd.Flags().StringVar(&execIMDSResource, "resource", "https://management.azure.com/", "Resource to request")
	execIMDSCmd.Flags().StringVar(&execIMDSClientID, "client-id", "", "Pin user-assigned identity by client id")
	execIMDSCmd.Flags().StringVar(&execIMDSObjectID, "object-id", "", "Pin user-assigned identity by object id")
	execIMDSCmd.Flags().StringVar(&execIMDSOut, "out", "", "Save token JSON to file")
	execIMDSCmd.Flags().IntVar(&execIMDSTimeout, "timeout", 30, "Timeout seconds")

	execAzureCmd.Flags().StringVar(&execToken, "token", "", "Access token for management.azure.com (required)")
	execAzureCmd.Flags().StringVar(&execSubID, "subscription-id", "", "Azure subscription id (required)")
	execAzureCmd.Flags().StringVar(&execGroup, "resource-group", "", "Resource group (required)")
	execAzureCmd.Flags().StringVar(&execVMID, "vm-id", "", "VM name (required)")
	execAzureCmd.Flags().StringVar(&execCmdStr, "cmd", "", "Shell command to run (required)")
	execAzureCmd.Flags().IntVar(&execTimeout, "timeout", 300, "Timeout seconds")
	execAzureCmd.Flags().StringVar(&execPreset, "browser-preset", "chrome", "TLS fingerprint preset (chrome | edge | firefox)")
	execAzureCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = execAzureCmd.MarkFlagRequired("token")
	_ = execAzureCmd.MarkFlagRequired("subscription-id")
	_ = execAzureCmd.MarkFlagRequired("resource-group")
	_ = execAzureCmd.MarkFlagRequired("vm-id")
	_ = execAzureCmd.MarkFlagRequired("cmd")

	execAWSCmd.Flags().StringVar(&execAK, "access-key", "", "AWS access key (required)")
	execAWSCmd.Flags().StringVar(&execSK, "secret-key", "", "AWS secret key (required)")
	execAWSCmd.Flags().StringVar(&execSessTok, "session-token", "", "AWS session token")
	execAWSCmd.Flags().StringVar(&execRegion, "region", "us-east-1", "AWS region")
	execAWSCmd.Flags().StringVar(&execInstance, "instance-id", "", "EC2 instance id (required)")
	execAWSCmd.Flags().StringVar(&execCmdStr, "cmd", "", "Shell command to run (required)")
	execAWSCmd.Flags().IntVar(&execTimeout, "timeout", 300, "Timeout seconds")
	execAWSCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = execAWSCmd.MarkFlagRequired("access-key")
	_ = execAWSCmd.MarkFlagRequired("secret-key")
	_ = execAWSCmd.MarkFlagRequired("instance-id")
	_ = execAWSCmd.MarkFlagRequired("cmd")

	execGitHubCmd.Flags().StringVar(&execToken, "token", "", "GitHub token (required)")
	execGitHubCmd.Flags().StringVar(&execRepo, "repo", "", "owner/repo (required)")
	execGitHubCmd.Flags().StringVar(&execWorkflow, "workflow", "", "Workflow file name (required)")
	execGitHubCmd.Flags().StringVar(&execRef, "ref", "main", "Git ref to dispatch")
	execGitHubCmd.Flags().IntVar(&execTimeout, "timeout", 60, "Timeout seconds")
	execGitHubCmd.Flags().StringVar(&execPreset, "browser-preset", "chrome", "TLS fingerprint preset (chrome | edge | firefox)")
	execGitHubCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = execGitHubCmd.MarkFlagRequired("token")
	_ = execGitHubCmd.MarkFlagRequired("repo")
	_ = execGitHubCmd.MarkFlagRequired("workflow")
}

func runExecAzure(cmd *cobra.Command, args []string) error {
	ws, err := openGovernedWorkspace(execWorkspace)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	hc, err := transport.NewClient(execPreset, time.Duration(execTimeout)*time.Second)
	if err != nil {
		return err
	}

	e := exec.NewAzureExecutor(execSubID, execToken, hc)
	res, err := mutation.Run(ctx, ws, &cliMutation{
		kindV:   "exec.azure",
		targetV: fmt.Sprintf("%s/%s (sub %s)", execGroup, execVMID, execSubID),
		undoV:   irreversibleShell(),
		execFn: func(ctx context.Context) (string, error) {
			result, err := e.ExecuteOnAzureVM(ctx, execGroup, execVMID, execCmdStr)
			if err != nil {
				return "", err
			}
			printResult(result)
			return "", nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Action:   %s\nStatus:   %s\n", res.ActionID, res.Status)
	return nil
}

func runExecAWS(cmd *cobra.Command, args []string) error {
	ws, err := openGovernedWorkspace(execWorkspace)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	e := exec.NewAWSExecutor(execRegion, execAK, execSK, execSessTok, nil)
	res, err := mutation.Run(ctx, ws, &cliMutation{
		kindV:   "exec.aws",
		targetV: fmt.Sprintf("%s@%s", execInstance, execRegion),
		undoV:   irreversibleShell(),
		execFn: func(ctx context.Context) (string, error) {
			result, err := e.ExecuteOnEC2(ctx, execInstance, execCmdStr)
			if err != nil {
				return "", err
			}
			printResult(result)
			return "", nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Action:   %s\nStatus:   %s\n", res.ActionID, res.Status)
	return nil
}

func runExecGitHub(cmd *cobra.Command, args []string) error {
	ws, err := openGovernedWorkspace(execWorkspace)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	hc, err := transport.NewClient(execPreset, time.Duration(execTimeout)*time.Second)
	if err != nil {
		return err
	}

	e := exec.NewGitHubExecutor(execToken, hc)
	res, err := mutation.Run(ctx, ws, &cliMutation{
		kindV:   "exec.github",
		targetV: fmt.Sprintf("%s @%s (%s)", execRepo, execRef, execWorkflow),
		undoV:   irreversibleShell(),
		execFn: func(ctx context.Context) (string, error) {
			result, err := e.ExecuteOnRunner(ctx, execRepo, execWorkflow, execRef, nil)
			if err != nil {
				return "", err
			}
			printResult(result)
			return "", nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Action:   %s\nStatus:   %s\n", res.ActionID, res.Status)
	return nil
}

func printResult(r *types.CommandResult) error {
	fmt.Printf("Status:   %s\nExitCode: %d\nOutput:\n%s\n", r.Status, r.ExitCode, r.Output)
	return nil
}
