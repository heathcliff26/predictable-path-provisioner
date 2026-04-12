package provisioner

import (
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newStorageConfig(sc *storagev1.StorageClass) (storageConfig, error) {
	basePath, ok := sc.Parameters[parameterBasePath]
	if !ok {
		basePath = defaultBasePath
	}
	err := validateBasePath(basePath)
	if err != nil {
		return storageConfig{}, fmt.Errorf("invalid storage class parameters: %w", err)
	}
	pathTemplate, ok := sc.Parameters[parameterPathTemplate]
	if !ok {
		pathTemplate = defaultPathTemplate
	}
	err = validatePathTemplate(pathTemplate)
	if err != nil {
		return storageConfig{}, fmt.Errorf("invalid storage class parameters: %w", err)
	}

	return storageConfig{
		BasePath:     basePath,
		PathTemplate: pathTemplate,
	}, nil
}

func createFilePath(cfg storageConfig, pvc *corev1.PersistentVolumeClaim) string {
	volume := evalPathTemplate(cfg.PathTemplate, pvc)
	return filepath.Join(cfg.BasePath, volume)
}

func evalPathTemplate(template string, pvc *corev1.PersistentVolumeClaim) string {
	values := map[string]string{
		"{{pvc.namespace}}": pvc.Namespace,
		"{{pvc.name}}":      pvc.Name,
		"{{pvc.uid}}":       string(pvc.UID),
	}
	volume := template
	for k, v := range values {
		volume = strings.ReplaceAll(volume, k, v)
	}
	return volume
}

func validateBasePath(basePath string) error {
	if basePath == "" {
		return fmt.Errorf("basePath is required")
	}
	if !filepath.IsAbs(basePath) {
		return fmt.Errorf("basePath must be an absolute path")
	}
	return nil
}

func validatePathTemplate(pathTemplate string) error {
	if pathTemplate == "" {
		return fmt.Errorf("pathTemplate is required")
	}
	volume := evalPathTemplate(pathTemplate, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "test-namespace",
			UID:       types.UID("12345"),
		},
	})

	if filepath.Join(defaultBasePath, volume) != fmt.Sprintf("%s/%s", defaultBasePath, volume) {
		return fmt.Errorf("pathTemplate must evaluate to a relative path located within basePath")
	}
	return nil
}

func isForCurrentNode(nodeName string, affinity *corev1.VolumeNodeAffinity) bool {
	if affinity == nil || affinity.Required == nil {
		return false
	}
	if len(affinity.Required.NodeSelectorTerms) != 1 {
		return false
	}
	terms := affinity.Required.NodeSelectorTerms[0]
	if len(terms.MatchExpressions) != 1 {
		return false
	}
	expr := terms.MatchExpressions[0]
	if len(expr.Values) != 1 {
		return false
	}
	return expr.Key == "kubernetes.io/hostname" && expr.Operator == corev1.NodeSelectorOpIn && expr.Values[0] == nodeName
}
