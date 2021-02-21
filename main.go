package main

import (
	"fmt"
	_ "github.com/aws/aws-sdk-go/service/eks"
	"github.com/mumoshu/argocd-clusterset/pkg/manager"
	"github.com/mumoshu/argocd-clusterset/pkg/run"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ApplicationName = "argocd-eks"
)

func main() {
	var (
		dryRun   bool
		ns       string
		roleARN  string
		name     string
		endpoint string
		caData   string
		eksTags  []string
		labelKVs []string
		awsAuthConfigRoleARN string
	)

	cmd := &cobra.Command{
		Use: ApplicationName,
	}

	flag := cmd.PersistentFlags()

	flag.BoolVar(&dryRun, "dry-run", false, "")
	flag.StringVar(&ns, "namespace", "", "")
	flag.StringVar(&roleARN, "role-arn", "", "")
	flag.StringVar(&name, "name", "", "")
	flag.StringVar(&endpoint, "endpoint", "", "")
	flag.StringVar(&caData, "ca-data", "", "")
	flag.StringSliceVar(&eksTags, "eks-tags", nil, "Comma-separated KEY=VALUE pairs of EKS control-plane tags")
	flag.StringSliceVar(&labelKVs, "labels", nil, "Comma-separated KEY=VALUE pairs of cluster secret labels")
	flag.StringVar(&awsAuthConfigRoleARN, "aws-auth-config-role-arn", "", "")

	newLabels := func() map[string]string {
		labels := map[string]string{}

		return labels
	}

	newConfig := func() run.Config {
		return run.Config{
			DryRun:   dryRun,
			NS:       ns,
			RoleARN: roleARN,
			Name:     name,
			Endpoint: endpoint,
			CAData:   caData,
			Labels:   newLabels(),
			AwsAuthConfigRoleARN: awsAuthConfigRoleARN,
		}
	}

	newSetConfig := func() run.ClusterSetConfig {
		tags := map[string]string{}
		for _, kv := range eksTags {
			split := strings.Split(kv, "=")
			tags[split[0]] = split[1]
		}

		setConfig := run.ClusterSetConfig{
			DryRun:               dryRun,
			NS:                   ns,
			RoleARN:              roleARN,
			EKSTags:              tags,
			Labels:               newLabels(),
			AWSAuthConfigRoleARN: awsAuthConfigRoleARN,
		}

		return setConfig
	}

	create := &cobra.Command{
		Use: "create",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run.Create(newConfig())
		},
	}
	cmd.AddCommand(create)

	delete := &cobra.Command{
		Use: "delete",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run.Delete(newConfig())
		},
	}
	cmd.AddCommand(delete)

	createMissing := &cobra.Command{
		Use: "create-missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run.CreateMissing(newSetConfig())
		},
	}
	cmd.AddCommand(createMissing)

	deleteMissing := &cobra.Command{
		Use: "delete-missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run.DeleteMissing(newSetConfig())
		},
	}
	cmd.AddCommand(deleteMissing)

	sync := &cobra.Command{
		Use: "sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run.Sync(newSetConfig())
		},
	}
	cmd.AddCommand(sync)

	m := &manager.Manager{}

	controllerManager := &cobra.Command{
		Use: "controller-manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			return m.Run()
		},
	}
	m.AddPFlags(controllerManager.Flags())
	cmd.AddCommand(controllerManager)

	err := cmd.Execute()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}
