package provisioner

import (
	"fmt"
	"path/filepath"

	storagev1 "k8s.io/api/storage/v1"
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
	return storageConfig{
		BasePath: basePath,
	}, nil
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
