package provisioner

import (
	"context"
	"log/slog"
	"os"

	"github.com/heathcliff26/predictable-path-provisioner/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v13/controller"
)

func Execute() {
	cmd := NewRootCommand()
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	cobra.AddTemplateFunc(
		"ProgramName", func() string {
			return version.Name
		},
	)

	rootCmd := &cobra.Command{
		Use:   version.Name,
		Short: version.Name + " k8s hostpath provisioner with human readable paths",
		Run: func(cmd *cobra.Command, _ []string) {
			run()
		},
	}

	rootCmd.AddCommand(
		version.NewCommand(),
	)

	return rootCmd
}

func run() {
	node := os.Getenv("NODE_NAME")
	if node == "" {
		slog.Error("NODE_NAME environment variable is not set")
		os.Exit(1)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		slog.Error("Failed to get kubeconfig, the provisioner should be run inside a kubernetes cluster", "err", err)
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "err", err)
		os.Exit(1)
	}

	slog.Info("Starting predictable-path-provisioner", "version", version.Version())

	p := NewProvisioner(defaultProvisionerName, node)

	ctx := context.Background()

	ctrl := controller.NewProvisionController(
		ctx,
		client,
		defaultProvisionerName,
		p,
		controller.LeaderElection(false),
	)

	ctrl.Run(ctx)
}
