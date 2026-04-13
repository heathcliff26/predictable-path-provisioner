package provisioner

import (
	"os"
	"os/exec"
	"testing"

	"github.com/heathcliff26/predictable-path-provisioner/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()

	assert.Equal(t, version.Name, cmd.Use)
}

func TestGetProvisionerName(t *testing.T) {
	t.Run("DefaultName", func(t *testing.T) {
		t.Setenv(EnvProvisionerName, "")

		name := getProvisionerName()
		assert.Equal(t, defaultProvisionerName, name, "Should return default provisioner name when env var is not set")
	})

	t.Run("CustomName", func(t *testing.T) {
		customName := "custom.example.com/my-provisioner"
		t.Setenv(EnvProvisionerName, customName)

		name := getProvisionerName()
		assert.Equal(t, customName, name, "Should return custom provisioner name from env var")
	})
}

func TestRunMissingNodeName(t *testing.T) {
	if os.Getenv("RUN_CRASH_TEST") == "1" {
		os.Setenv("NODE_NAME", "")
		Execute()
	}
	execExitTest(t, "TestRunMissingNodeName", "NODE_NAME environment variable is not set")
}

func TestRunMissingKubernetesConfig(t *testing.T) {
	if os.Getenv("RUN_CRASH_TEST") == "1" {
		os.Setenv("NODE_NAME", "test")
		Execute()
	}
	execExitTest(t, "TestRunMissingKubernetesConfig", "Failed to get kubeconfig, the provisioner should be run inside a kubernetes cluster")
}

func execExitTest(t *testing.T, test string, contains string) {
	require := require.New(t)

	cmd := exec.Command(os.Args[0], "-test.run="+test)
	cmd.Env = append(os.Environ(), "RUN_CRASH_TEST=1")

	out, err := cmd.CombinedOutput()
	require.Contains(string(out), contains, "Output should contain expected string")
	require.Error(err, "Process should exit with error")
}
