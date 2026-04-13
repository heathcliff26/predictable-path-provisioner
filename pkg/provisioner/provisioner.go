package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v13/controller"
)

const (
	defaultProvisionerName = "heathcliff.eu/predictable-path-provisioner"
	DefaultBasePath        = "/var/lib/predictable-path-provisioner"
	defaultPathTemplate    = "{{pvc.namespace}}/{{pvc.name}}"

	pvProvisionedByAnnotation = "pv.kubernetes.io/provisioned-by"

	parameterBasePath     = "basePath"
	parameterPathTemplate = "pathTemplate"
)

var hostPrefix = "/host"

type provisioner struct {
	name string
	node string
}

type storageConfig struct {
	BasePath     string
	PathTemplate string
}

// Ensure external provisioner interfaces are implemented
var _ controller.Provisioner = (*provisioner)(nil)
var _ controller.BlockProvisioner = (*provisioner)(nil)

func NewProvisioner(name, node string) *provisioner {
	return &provisioner{
		name: name,
		node: node,
	}
}

// Implements Provision from controller.Provisioner interface
//
// Provision creates a volume i.e. the storage asset and returns a PV object
// for the volume. The provisioner can return an error (e.g. timeout) and state
// ProvisioningInBackground to tell the controller that provisioning may be in
// progress after Provision() finishes. The controller will call Provision()
// again with the same parameters, assuming that the provisioner continues
// provisioning the volume. The provisioner must return either final error (with
// ProvisioningFinished) or success eventually, otherwise the controller will try
// forever (unless FailedProvisionThreshold is set).
func (p *provisioner) Provision(_ context.Context, opts controller.ProvisionOptions) (*corev1.PersistentVolume, controller.ProvisioningState, error) {
	if opts.StorageClass == nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("storage class is required")
	}
	if opts.PVC == nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("persistent volume claim is required")
	}
	if slices.Contains(opts.PVC.Spec.AccessModes, corev1.ReadWriteMany) || slices.Contains(opts.PVC.Spec.AccessModes, corev1.ReadOnlyMany) {
		return nil, controller.ProvisioningFinished, fmt.Errorf("unsupported access modes: volumes can only be mounted on a single host")
	}
	slog.Info("Provision called", "pvc.Name", opts.PVC.Name, "pvc.Namespace", opts.PVC.Namespace, "storageClass", opts.StorageClass.Name)

	cfg, err := newStorageConfig(opts.StorageClass)
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	name := opts.PVName
	path := createFilePath(cfg, opts.PVC)
	pathWithHostPrefix := filepath.Join(hostPrefix, path)

	// #nosec G301 -- Directory permissions are decided by umask
	err = os.MkdirAll(filepath.Join(hostPrefix, cfg.BasePath), 0755)
	if err != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("failed to create base path: %w", err)
	}
	// #nosec G301 -- Directory permissions are decided by umask
	err = os.MkdirAll(pathWithHostPrefix, 0755)
	if err != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("failed to create volume: %w", err)
	}
	// #nosec G302 -- Directory needs to be writable by anyone as the user inside the consuming container is not known and likely not root.
	err = os.Chmod(pathWithHostPrefix, 0777)
	if err != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("failed to set permissions on volume: %w", err)
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				pvProvisionedByAnnotation: p.name,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: opts.PVC.Spec.Resources.Requests,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path: path,
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			ClaimRef: &corev1.ObjectReference{
				APIVersion:      opts.PVC.APIVersion,
				Kind:            opts.PVC.Kind,
				Namespace:       opts.PVC.Namespace,
				Name:            opts.PVC.Name,
				UID:             opts.PVC.UID,
				ResourceVersion: opts.PVC.ResourceVersion,
			},
			PersistentVolumeReclaimPolicy: *opts.StorageClass.ReclaimPolicy,
			StorageClassName:              opts.StorageClass.Name,
			VolumeMode:                    opts.PVC.Spec.VolumeMode,
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{p.node},
								},
							},
						},
					},
				},
			},
		},
	}
	return pv, controller.ProvisioningFinished, nil
}

// Implements Delete from controller.Provisioner interface
//
// Delete removes the storage asset that was created by Provision backing the
// given PV. Does not delete the PV object itself.
//
// May return IgnoredError to indicate that the call has been ignored and no
// action taken.
func (p *provisioner) Delete(_ context.Context, pv *corev1.PersistentVolume) error {
	if pv == nil {
		return fmt.Errorf("persistent volume can't be nil")
	}
	if !isForCurrentNode(p.node, pv.Spec.NodeAffinity) {
		slog.Debug("Delete ignored: volume is not on this node", "pv", pv.Name)
		return &controller.IgnoredError{}
	}
	if pv.Spec.Local == nil {
		return fmt.Errorf("provided persistent volume is not a local volume")
	}
	slog.Info("Delete called", "pv", pv.Name, "path", pv.Spec.Local.Path)
	return os.RemoveAll(filepath.Join(hostPrefix, pv.Spec.Local.Path))
}

// Implements SupportsBlock from controller.BlockProvisioner interface
//
// SupportsBlock returns whether provisioner supports block volume.
func (p *provisioner) SupportsBlock(_ context.Context) bool {
	return false
}
