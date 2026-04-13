package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heathcliff26/predictable-path-provisioner/pkg/provisioner"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/utils"
)

const (
	customProvisionerNamespace = "p3-custom"
	customProvisionerRelease   = "custom-p3"
	customProvisionerName      = "custom.example.com/p3"
	customStorageClassName     = "custom-p3"
	customTestNamespace        = "test-custom-provisioner"
	customPVCName              = "custom-provisioner-pvc"
)

func TestCustomProvisionerName(t *testing.T) {
	installFeat := features.New("install custom provisioner via helm").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			err := utils.RunCommandWithSeperatedOutput(
				fmt.Sprintf(
					"helm install %s manifests/helm"+
						" --namespace %s"+
						" --create-namespace"+
						" --set image.repository=localhost/p3"+
						" --set image.tag=%s"+
						" --set provisionerName=%s"+
						" --set fullnameOverride=%s"+
						" --set storageClass.name=%s"+
						" --set storageClass.default=false",
					customProvisionerRelease,
					customProvisionerNamespace,
					imageTag,
					customProvisionerName,
					customProvisionerRelease,
					customStorageClassName,
				),
				os.Stdout,
				os.Stderr,
			)
			require.NoError(t, err, "Should install custom provisioner via helm")
			return ctx
		}).
		Assess("available", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var dep appsv1.DaemonSet
			err := c.Client().Resources().Get(ctx, customProvisionerRelease, customProvisionerNamespace, &dep)
			require.NoError(err, "Should get custom provisioner daemonset")

			err = wait.For(conditions.New(c.Client().Resources()).DaemonSetReady(&dep),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "Custom provisioner daemonset should be ready")
			return ctx
		}).
		Feature()

	testenv.Test(t, installFeat)
	if t.Failed() {
		t.Fatal("Failed to install custom provisioner via helm, can't run tests")
	}

	customPVCFeat := newCustomProvisionerPVCFeature()
	testenv.Test(t, customPVCFeat)

	cleanupFeat := features.New("uninstall custom provisioner via helm").
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			err := utils.RunCommandWithSeperatedOutput(
				fmt.Sprintf("helm uninstall %s --namespace %s", customProvisionerRelease, customProvisionerNamespace),
				os.Stdout,
				os.Stderr,
			)
			require.NoError(t, err, "Should uninstall custom provisioner via helm")

			var ns corev1.Namespace
			err = c.Client().Resources().Get(ctx, customProvisionerNamespace, "", &ns)
			if err == nil {
				err = c.Client().Resources().Delete(ctx, &ns)
				require.NoError(t, err, "Should delete custom provisioner namespace")
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, cleanupFeat)
}

func newCustomProvisionerPVCFeature() features.Feature {
	var pvName, nodeName string

	return features.New("custom provisioner PVC").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			err := decoder.ApplyWithManifestDir(ctx, c.Client().Resources(), "tests/testdata/", "custom-provisioner.yaml", []resources.CreateOption{})
			require.NoError(t, err, "Should apply custom provisioner test manifest")
			return ctx
		}).
		Assess("pvc bound", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      customPVCName,
					Namespace: customTestNamespace,
				},
			}
			err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(pvc, func(obj k8s.Object) bool {
				return obj.(*corev1.PersistentVolumeClaim).Status.Phase == corev1.ClaimBound
			}),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "Custom provisioner PVC should be bound")

			pvs := &corev1.PersistentVolumeList{}
			err = c.Client().Resources().List(ctx, pvs)
			require.NoError(err, "Should list PVs")

			for _, pv := range pvs.Items {
				if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name == customPVCName && pv.Spec.ClaimRef.Namespace == customTestNamespace {
					pvName = pv.Name
					break
				}
			}
			require.NotEmpty(pvName, "PV should exist for custom provisioner PVC")

			return ctx
		}).
		Assess("pv writable", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var pod corev1.Pod
			err := c.Client().Resources().Get(ctx, "test-pod", customTestNamespace, &pod)
			require.NoError(err, "Should get test pod")

			err = wait.For(conditions.New(c.Client().Resources()).PodPhaseMatch(&pod, corev1.PodSucceeded),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "Test pod should complete successfully")

			nodeName = pod.Spec.NodeName

			output := listNodePath(t, c, nodeName, customTestNamespace, filepath.Join(provisioner.DefaultBasePath, customTestNamespace, customPVCName))
			require.Contains(output, "test-file", "Should have test file on disk")

			return ctx
		}).
		Assess("delete", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var pod corev1.Pod
			err := c.Client().Resources().Get(ctx, "test-pod", customTestNamespace, &pod)
			require.NoError(err, "Should get test pod")

			err = c.Client().Resources().Delete(ctx, &pod)
			require.NoError(err, "Should delete test pod")

			var pvc corev1.PersistentVolumeClaim
			err = c.Client().Resources().Get(ctx, customPVCName, customTestNamespace, &pvc)
			require.NoError(err, "Should get PVC")

			err = c.Client().Resources().Delete(ctx, &pvc)
			require.NoError(err, "Should delete PVC")

			err = wait.For(conditions.New(c.Client().Resources()).ResourceDeleted(&pvc),
				wait.WithTimeout(time.Minute*2),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "PVC should be deleted")

			var pv corev1.PersistentVolume
			err = c.Client().Resources().Get(ctx, pvName, "", &pv)
			require.Error(err, "PV should be deleted")

			output := listNodePath(t, c, nodeName, customTestNamespace, filepath.Join(provisioner.DefaultBasePath, customTestNamespace))
			require.NotContains(output, customPVCName, "Should delete the hostpath directory from the node")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var ns corev1.Namespace
			err := c.Client().Resources().Get(ctx, customTestNamespace, "", &ns)
			require.NoError(err, "Should get test namespace")

			err = c.Client().Resources().Delete(ctx, &ns)
			require.NoError(err, "Should delete test namespace")

			return ctx
		}).
		Feature()
}
