package provisioner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v13/controller"
)

func TestNewProvisioner(t *testing.T) {
	assert := assert.New(t)

	p := NewProvisioner("test-name", "test-node")

	assert.Equal("test-name", p.name, "Provisioner name should be set correctly")
	assert.Equal("test-node", p.node, "Provisioner node should be set correctly")
}

func TestProvision(t *testing.T) {
	tMatrix := []struct {
		Name  string
		Opts  controller.ProvisionOptions
		Error string
	}{
		{
			Name: "NilStorageClass",
			Opts: controller.ProvisionOptions{
				StorageClass: nil,
			},
			Error: "storage class is required",
		},
		{
			Name: "NilPersistentVolumeClaim",
			Opts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{},
				PVC:          nil,
			},
			Error: "persistent volume claim is required",
		},
		{
			Name: "VolumeNotForCurrentNode",
			Opts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					VolumeBindingMode: pointer(storagev1.VolumeBindingWaitForFirstConsumer),
				},
				PVC:              &corev1.PersistentVolumeClaim{},
				SelectedNodeName: "other-node",
			},
			Error: "Wrong node",
		},
		{
			Name: "PVCAccessModeRWX",
			Opts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{},
				PVC: &corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteMany,
						},
					},
				},
			},
			Error: "unsupported access modes: volumes can only be mounted on a single host",
		},
		{
			Name: "PVCAccessModeROX",
			Opts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{},
				PVC: &corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadOnlyMany,
						},
					},
				},
			},
			Error: "unsupported access modes: volumes can only be mounted on a single host",
		},
		{
			Name: "NewStorageConfigError",
			Opts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{
						"basePath": "",
					},
				},
				PVC: &corev1.PersistentVolumeClaim{},
			},
			Error: "basePath is required",
		},
	}

	for _, tCase := range tMatrix {
		t.Run(tCase.Name, func(t *testing.T) {
			assert := assert.New(t)

			p := NewProvisioner("test-name", "test-node")

			pv, state, err := p.Provision(t.Context(), tCase.Opts)
			assert.Nil(pv, "PV should be nil")
			assert.Equal(controller.ProvisioningFinished, state, "Provisioning state should be finished")
			assert.ErrorContains(err, tCase.Error, "Error message should match expected")
		})
	}

	t.Run("FailedToCreateBasePath", func(t *testing.T) {
		require := require.New(t)

		tmpDir := filepath.Join(t.TempDir(), "tmp")
		oldPrefix := hostPrefix
		hostPrefix = tmpDir
		t.Cleanup(func() {
			hostPrefix = oldPrefix
		})

		p := NewProvisioner("test-name", "test-node")

		require.NoError(os.MkdirAll(tmpDir, 0755), "Should create base path successfully")
		require.NoError(os.Chmod(tmpDir, 0500), "Should set base path permissions to read and execute only")

		pvc, state, err := p.Provision(t.Context(), controller.ProvisionOptions{
			StorageClass: &storagev1.StorageClass{
				Parameters: map[string]string{},
			},
			PVC: &corev1.PersistentVolumeClaim{},
		})
		require.Nil(pvc, "PV should be nil")
		require.Equal(controller.ProvisioningFinished, state, "Provisioning state should be finished")
		require.ErrorContains(err, "failed to create base path", "Error message should indicate failure to create base path")
	})
	t.Run("FailedToCreateVolume", func(t *testing.T) {
		require := require.New(t)

		tmpDir := t.TempDir()
		oldPrefix := hostPrefix
		hostPrefix = tmpDir
		t.Cleanup(func() {
			hostPrefix = oldPrefix
		})

		p := NewProvisioner("test-name", "test-node")

		tmpDir = filepath.Join(tmpDir, DefaultBasePath)
		require.NoError(os.MkdirAll(tmpDir, 0755), "Should create base path successfully")
		require.NoError(os.Chmod(tmpDir, 0500), "Should set base path permissions to read and execute only")

		pvc, state, err := p.Provision(t.Context(), controller.ProvisionOptions{
			StorageClass: &storagev1.StorageClass{
				Parameters: map[string]string{},
			},
			PVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "default",
				},
			},
		})
		require.Nil(pvc, "PV should be nil")
		require.Equal(controller.ProvisioningFinished, state, "Provisioning state should be finished")
		require.ErrorContains(err, "failed to create volume", "Error message should indicate failure to create base path")
	})

	t.Run("SuccessGeneratedPVAndHostPath", func(t *testing.T) {
		require := require.New(t)

		tmpDir := t.TempDir()
		oldPrefix := hostPrefix
		hostPrefix = tmpDir
		t.Cleanup(func() {
			hostPrefix = oldPrefix
		})

		reclaimPolicy := corev1.PersistentVolumeReclaimDelete
		volumeMode := corev1.PersistentVolumeFilesystem
		pvc := &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-pvc",
				Namespace:       "test-ns",
				UID:             types.UID("12345"),
				ResourceVersion: "7",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				VolumeMode: &volumeMode,
			},
		}

		opts := controller.ProvisionOptions{
			PVName: "test-pv",
			StorageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{Name: "local-sc"},
				Parameters: map[string]string{
					parameterBasePath:     "/test-volumes",
					parameterPathTemplate: "{{pvc.namespace}}/{{pvc.name}}",
				},
				ReclaimPolicy: &reclaimPolicy,
			},
			PVC: pvc,
		}

		p := NewProvisioner("test-name", "test-node")
		pv, state, err := p.Provision(t.Context(), opts)
		require.NoError(err)
		require.Equal(controller.ProvisioningFinished, state)
		require.NotNil(pv)

		require.Equal("test-pv", pv.Name)
		require.Equal("test-name", pv.Annotations[pvProvisionedByAnnotation])
		require.Equal("/test-volumes/test-ns/test-pvc", pv.Spec.Local.Path)
		require.Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pv.Spec.AccessModes)
		require.Equal(opts.PVC.Spec.Resources.Requests, pv.Spec.Capacity)
		require.Equal("local-sc", pv.Spec.StorageClassName)
		require.NotNil(pv.Spec.ClaimRef)
		require.Equal("test-ns", pv.Spec.ClaimRef.Namespace)
		require.Equal("test-pvc", pv.Spec.ClaimRef.Name)
		require.Equal(types.UID("12345"), pv.Spec.ClaimRef.UID)
		require.NotNil(pv.Spec.NodeAffinity)

		require.DirExists(filepath.Join(tmpDir, pv.Spec.Local.Path), "Volume directory should be created on disk")
	})
}

func TestDelete(t *testing.T) {
	p := NewProvisioner("test-name", "test-node")

	newPV := func(name, node string, local *corev1.LocalVolumeSource) *corev1.PersistentVolume {
		return &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{Local: local},
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{node},
							}},
						}},
					},
				},
			},
		}
	}

	t.Run("NilPersistentVolume", func(t *testing.T) {
		err := p.Delete(t.Context(), nil)
		assert.ErrorContains(t, err, "persistent volume can't be nil")
	})

	t.Run("VolumeNotForCurrentNode", func(t *testing.T) {
		pv := newPV("pv-not-current-node", "other-node", &corev1.LocalVolumeSource{Path: "/tmp/not-used"})
		err := p.Delete(t.Context(), pv)

		assert.Equal(t, err, &controller.IgnoredError{}, "Should return an IgnoredError")
	})

	t.Run("CurrentNodeWithoutLocalVolume", func(t *testing.T) {
		pv := newPV("pv-no-local", "test-node", nil)
		err := p.Delete(t.Context(), pv)
		assert.ErrorContains(t, err, "provided persistent volume is not a local volume")
	})

	t.Run("CurrentNodeLocalVolumeDeleteSuccess", func(t *testing.T) {
		require := require.New(t)

		oldPrefix := hostPrefix
		hostPrefix = t.TempDir()
		t.Cleanup(func() {
			hostPrefix = oldPrefix
		})

		path := filepath.Join(hostPrefix, "test-pv")

		require.NoError(os.MkdirAll(path, 0755), "Should create test volume directory successfully")

		pv := newPV("pv-local-success", "test-node", &corev1.LocalVolumeSource{Path: "test-pv"})
		err := p.Delete(t.Context(), pv)

		require.NoError(err)
		require.NoDirExists(path, "Volume directory should be deleted from disk")
	})
}

func TestSupportsBlock(t *testing.T) {
	p := NewProvisioner("test-name", "test-node")

	supportsBlock := p.SupportsBlock(t.Context())
	assert.False(t, supportsBlock, "SupportsBlock should return false")
}
