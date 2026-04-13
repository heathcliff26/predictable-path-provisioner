package e2e

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/heathcliff26/predictable-path-provisioner/pkg/provisioner"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestE2E(t *testing.T) {
	provisionerDeploymentFeat := features.New("deploy p3").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			err := decoder.ApplyWithManifestDir(ctx, c.Client().Resources(), "manifests/release", "p3.yaml", []resources.CreateOption{})
			require.NoError(t, err, "Should apply p3 manifest")

			return ctx
		}).
		Assess("available", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var dep appsv1.DaemonSet
			err := c.Client().Resources().Get(ctx, "p3", namespace, &dep)
			require.NoError(err, "Should get p3 daemonset")

			err = wait.For(conditions.New(c.Client().Resources()).DaemonSetReady(&dep),
				wait.WithTimeout(time.Minute*1),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "p3 daemonset should be ready")

			pods := &corev1.PodList{}
			err = c.Client().Resources(namespace).List(ctx, pods)
			require.NoError(err, "Should list pods in p3 namespace")

			nodes := &corev1.NodeList{}
			err = c.Client().Resources().List(ctx, nodes)
			require.NoError(err, "Should list nodes in cluster")

			// Subtract the control-plane node
			require.Equal(len(pods.Items), len(nodes.Items)-1, "Number of provisioner pods should match number of worker-nodes")

			for _, node := range nodes.Items {
				if _, ok := node.GetLabels()["node-role.kubernetes.io/control-plane"]; ok {
					continue
				}

				found := false
				var pod corev1.Pod
				for _, pod = range pods.Items {
					if pod.Spec.NodeName == node.Name {
						found = true
						break
					}
				}
				require.Truef(found, "Should find a pod running on node %s", node.Name)

				err = wait.For(conditions.New(c.Client().Resources()).PodConditionMatch(&pod, corev1.PodReady, corev1.ConditionTrue),
					wait.WithTimeout(time.Minute*1),
					wait.WithInterval(time.Second*5),
				)
				require.NoErrorf(err, "Pod %s on node %s should be ready", pod.Name, node.Name)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, provisionerDeploymentFeat)
	if t.Failed() {
		t.Fatal("Failed to deploy p3, can't run tests")
	}

	simplePVCFeat := newSimplePVCFeature("DefaultSC", "test-simple-pvc", "simple-pvc.yaml", "default-sc")
	waitForFirstConsumerFeat := newSimplePVCFeature("WaitForFirstConsumer", "test-wait-for-consumer", "wait-for-consumer.yaml", "test-pvc")

	testenv.TestInParallel(t, simplePVCFeat, waitForFirstConsumerFeat)
}

func newSimplePVCFeature(featName, nsName, manifest, pvcName string) features.Feature {
	var pvName, nodeName string

	return features.New(featName).
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			err := decoder.ApplyWithManifestDir(ctx, c.Client().Resources(), "tests/testdata/", manifest, []resources.CreateOption{})
			require.NoError(t, err, "Should apply manifest")

			return ctx
		}).
		Assess("pvc bound", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: nsName,
				},
			}
			err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(pvc, func(obj k8s.Object) bool {
				return obj.(*corev1.PersistentVolumeClaim).Status.Phase == corev1.ClaimBound
			}),
				wait.WithTimeout(time.Minute*1),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "PVC should be bound")

			pvs := &corev1.PersistentVolumeList{}
			err = c.Client().Resources().List(ctx, pvs)
			require.NoError(err, "Should list PV")

			for _, pv := range pvs.Items {
				if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name == pvcName && pv.Spec.ClaimRef.Namespace == nsName {
					pvName = pv.Name
					break
				}
			}
			require.NotEmpty(pvName, "PV should exist")

			return ctx
		}).
		Assess("pv writable", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var pod corev1.Pod
			err := c.Client().Resources().Get(ctx, "test-pod", nsName, &pod)
			require.NoError(err, "Should get test pod")

			err = wait.For(conditions.New(c.Client().Resources()).PodPhaseMatch(&pod, corev1.PodSucceeded),
				wait.WithTimeout(time.Minute*1),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "Test pod should complete successfully")

			nodeName = pod.Spec.NodeName

			output := listNodePath(t, c, nodeName, nsName, filepath.Join(provisioner.DefaultBasePath, nsName, pvcName))
			require.Contains(output, "test-file", "Should have test file on disk")

			return ctx
		}).
		Assess("delete", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var pod corev1.Pod
			err := c.Client().Resources().Get(ctx, "test-pod", nsName, &pod)
			require.NoError(err, "Should get test pod")

			err = c.Client().Resources().Delete(ctx, &pod)
			require.NoError(err, "Should delete test pod")

			var pvc corev1.PersistentVolumeClaim
			err = c.Client().Resources().Get(ctx, pvcName, nsName, &pvc)
			require.NoError(err, "Should get PVC")

			err = c.Client().Resources().Delete(ctx, &pvc)
			require.NoError(err, "Should delete PVC")

			err = wait.For(conditions.New(c.Client().Resources()).ResourceDeleted(&pvc),
				wait.WithTimeout(time.Minute*1),
				wait.WithInterval(time.Second*5),
			)
			require.NoError(err, "PVC should be deleted")

			var pv corev1.PersistentVolume
			err = c.Client().Resources().Get(ctx, pvName, "", &pv)
			require.Error(err, "PV should be deleted")

			output := listNodePath(t, c, nodeName, nsName, filepath.Join(provisioner.DefaultBasePath, nsName))
			require.NotContains(output, pvcName, "Should delete the hostpath directory from the node")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			require := require.New(t)

			var ns corev1.Namespace
			err := c.Client().Resources().Get(ctx, nsName, "", &ns)
			require.NoError(err, "Should get namespace")

			err = c.Client().Resources().Delete(ctx, &ns)
			require.NoError(err, "Should delete namespace")

			return ctx
		}).
		Feature()
}

func listNodePath(t *testing.T, c *envconf.Config, nodeName, namespace, path string) string {
	require := require.New(t)

	client, err := kubernetes.NewForConfig(c.Client().RESTConfig())
	require.NoError(err, "Should create kubernetes client")

	podName := fmt.Sprintf("debug-pod-%s-%d", nodeName, rand.IntN(999999999))
	debugPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "fedora",
					Image: "registry.fedoraproject.org/fedora:latest",
					Args:  []string{"ls", fmt.Sprintf("/host/%s", path)},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "rootfs",
							MountPath: "/host",
						},
					},
				},
			},
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: "rootfs",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
		},
	}

	debugPod, err = client.CoreV1().Pods(namespace).Create(t.Context(), debugPod, metav1.CreateOptions{})
	require.NoError(err, "Should create debug pod")

	err = wait.For(
		conditions.New(c.Client().Resources()).PodPhaseMatch(debugPod, corev1.PodSucceeded),
		wait.WithTimeout(time.Minute*1),
		wait.WithInterval(time.Second*5),
	)
	require.NoError(err, "Should wait for debug pod to succeed")

	req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(t.Context())
	require.NoError(err, "Should get logs from debug pod")
	defer stream.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(stream)
	require.NoError(err, "Should read logs from debug pod")

	return buf.String()
}
