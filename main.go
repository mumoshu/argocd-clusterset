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
		name     string
		endpoint string
		caData   string
		eksTags  []string
	)

	cmd := &cobra.Command{
		Use: ApplicationName,
	}

	flag := cmd.PersistentFlags()

	flag.BoolVar(&dryRun, "dry-run", false, "")
	flag.StringVar(&ns, "namespace", "", "")
	flag.StringVar(&name, "name", "", "")
	flag.StringVar(&endpoint, "endpoint", "", "")
	flag.StringVar(&caData, "ca-data", "", "")
	flag.StringSliceVar(&eksTags, "eks-tags", nil, "Comma-separated KEY=VALUE pairs of EKS control-plane tags")

	cmd.MarkPersistentFlagRequired("name")

	create := &cobra.Command{
		Use: "create",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := run.Config{
				DryRun:   dryRun,
				NS:       ns,
				Name:     name,
				Endpoint: endpoint,
				CAData:   caData,
			}
			return run.Create(config)
		},
	}
	cmd.AddCommand(create)

	delete := &cobra.Command{
		Use: "delete",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := run.Config{
				DryRun:   dryRun,
				NS:       ns,
				Name:     name,
				Endpoint: endpoint,
				CAData:   caData,
			}
			return run.Delete(config)
		},
	}
	cmd.AddCommand(delete)

	createMissing := &cobra.Command{
		Use: "create-missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := map[string]string{}
			for _, kv := range eksTags {
				split := strings.Split(kv, "=")
				tags[split[0]] = split[1]
			}

			setConfig := run.ClusterSetConfig{
				DryRun:  dryRun,
				NS:      ns,
				EKSTags: tags,
			}
			return run.CreateMissing(setConfig)
		},
	}
	cmd.AddCommand(createMissing)

	deleteMissing := &cobra.Command{
		Use: "delete-missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := map[string]string{}
			for _, kv := range eksTags {
				split := strings.Split(kv, "=")
				tags[split[0]] = split[1]
			}

			setConfig := run.ClusterSetConfig{
				DryRun:  dryRun,
				NS:      ns,
				EKSTags: tags,
			}
			return run.DeleteMissing(setConfig)
		},
	}
	cmd.AddCommand(deleteMissing)

	sync := &cobra.Command{
		Use: "sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := map[string]string{}
			for _, kv := range eksTags {
				split := strings.Split(kv, "=")
				tags[split[0]] = split[1]
			}

			setConfig := run.ClusterSetConfig{
				DryRun:  dryRun,
				NS:      ns,
				EKSTags: tags,
			}
			return run.Sync(setConfig)
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
