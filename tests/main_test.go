package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/utils"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const namespace = "p3"

var testenv env.Environment
var imageTag string

func TestMain(m *testing.M) {
	testenv = env.New()
	clusterName := envconf.RandomName("p3-e2e", 24)
	imageTag = clusterName

	tmpDir := filepath.Join(os.TempDir(), clusterName)
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Using temporary directory %s\n", tmpDir)

	os.Setenv("KUBECONFIG", filepath.Join(tmpDir, "kubeconfig"))

	err = os.Chdir("..")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = utils.RunCommandWithSeperatedOutput("make REPOSITORY=localhost TAG="+clusterName+" image", os.Stdout, os.Stderr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = utils.RunCommandWithSeperatedOutput("make REPOSITORY=localhost TAG="+clusterName+" manifests", os.Stdout, os.Stderr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	imageArchive := filepath.Join(tmpDir, "p3-image.tar")

	err = utils.RunCommandWithSeperatedOutput(fmt.Sprintf("podman save -o %s localhost/p3:%s", imageArchive, clusterName), os.Stdout, os.Stderr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testenv.Setup(
		envfuncs.CreateClusterWithConfig(kind.NewProvider(), clusterName, "tests/kind-config.yaml"),
		envfuncs.LoadImageArchiveToCluster(clusterName, imageArchive),
	)
	testenv.Finish(
		envfuncs.ExportClusterLogs(clusterName, "./logs"),
		envfuncs.DestroyCluster(clusterName),
	)

	exitCode := testenv.Run(m)
	if exitCode != 0 {
		fmt.Printf("Failed e2e testsuite with exit code %d\n", exitCode)
	}

	fmt.Print("\nRunning cleanup\n\n")

	fmt.Printf("Deleting kind cluster %s\n", clusterName)
	err = utils.RunCommandWithSeperatedOutput("kind delete cluster --name "+clusterName, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Println(err)
		exitCode = 1
	}

	fmt.Printf("Removing temporary directory '%s'\n", tmpDir)
	err = os.RemoveAll(tmpDir)
	if err != nil {
		fmt.Printf("Failed to remove temporary directory '%s': %v\n", tmpDir, err)
		exitCode = 1
	}

	fmt.Println("")

	os.Exit(exitCode)
}
