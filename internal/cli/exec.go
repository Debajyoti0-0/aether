package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Cloud command execution",
	Long:  "Execute commands on cloud VMs and CI runners with valid tokens (authorized testing only).",
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
)

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.AddCommand(execAzureCmd, execAWSCmd, execGitHubCmd)

	execAzureCmd.Flags().StringVar(&execToken, "token", "", "Access token for management.azure.com (required)")
	execAzureCmd.Flags().StringVar(&execSubID, "subscription-id", "", "Azure subscription id (required)")
	execAzureCmd.Flags().StringVar(&execGroup, "resource-group", "", "Resource group (required)")
	execAzureCmd.Flags().StringVar(&execVMID, "vm-id", "", "VM name (required)")
	execAzureCmd.Flags().StringVar(&execCmdStr, "cmd", "", "Shell command to run (required)")
	execAzureCmd.Flags().IntVar(&execTimeout, "timeout", 300, "Timeout seconds")
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
	_ = execAWSCmd.MarkFlagRequired("access-key")
	_ = execAWSCmd.MarkFlagRequired("secret-key")
	_ = execAWSCmd.MarkFlagRequired("instance-id")
	_ = execAWSCmd.MarkFlagRequired("cmd")

	execGitHubCmd.Flags().StringVar(&execToken, "token", "", "GitHub token (required)")
	execGitHubCmd.Flags().StringVar(&execRepo, "repo", "", "owner/repo (required)")
	execGitHubCmd.Flags().StringVar(&execWorkflow, "workflow", "", "Workflow file name (required)")
	execGitHubCmd.Flags().StringVar(&execRef, "ref", "main", "Git ref to dispatch")
	execGitHubCmd.Flags().IntVar(&execTimeout, "timeout", 60, "Timeout seconds")
	_ = execGitHubCmd.MarkFlagRequired("token")
	_ = execGitHubCmd.MarkFlagRequired("repo")
	_ = execGitHubCmd.MarkFlagRequired("workflow")
}

func runExecAzure(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	hc, err := transport.NewClient(execPreset, time.Duration(execTimeout)*time.Second)
	if err != nil {
		return err
	}

	e := exec.NewAzureExecutor(execSubID, execToken, hc)
	result, err := e.ExecuteOnAzureVM(ctx, execGroup, execVMID, execCmdStr)
	if err != nil {
		return err
	}
	return printResult(result)
}

func runExecAWS(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	e := exec.NewAWSExecutor(execRegion, execAK, execSK, execSessTok, nil)
	result, err := e.ExecuteOnEC2(ctx, execInstance, execCmdStr)
	if err != nil {
		return err
	}
	return printResult(result)
}

func runExecGitHub(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(execTimeout)*time.Second)
	defer cancel()

	hc, err := transport.NewClient(execPreset, time.Duration(execTimeout)*time.Second)
	if err != nil {
		return err
	}

	e := exec.NewGitHubExecutor(execToken, hc)
	result, err := e.ExecuteOnRunner(ctx, execRepo, execWorkflow, execRef, nil)
	if err != nil {
		return err
	}
	return printResult(result)
}

func printResult(r *types.CommandResult) error {
	fmt.Printf("Status:   %s\nExitCode: %d\nOutput:\n%s\n", r.Status, r.ExitCode, r.Output)
	return nil
}
