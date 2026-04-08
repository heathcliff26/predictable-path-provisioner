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
	cmd := newCMD()
	err := cmd.Execute()
	if err != nil {
		slog.Error("Command exited with error", "err", err)
		os.Exit(1)
	}
}

func newCMD() *cobra.Command {
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
	node := os.Getenv("NODE_NAME")
	if node == "" {
		slog.Warn("NODE_NAME environment variable is not set, defaulting to 'localhost'")
		node = "localhost"
	}
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
