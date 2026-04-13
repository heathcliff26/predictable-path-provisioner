package provisioner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestEvalPathTemplate(t *testing.T) {
	tMatrix := []struct {
		Name                          string
		PVCName, PVCNamespace, PVCUID string
		Template                      string
		Expected                      string
	}{
		{
			Name:         "DefaultTemplate",
			PVCName:      "my-pvc",
			PVCNamespace: "my-namespace",
			PVCUID:       "12345",
			Template:     defaultPathTemplate,
			Expected:     "my-namespace/my-pvc",
		},
		{
			Name:         "WithUID",
			PVCName:      "my-pvc",
			PVCNamespace: "my-namespace",
			PVCUID:       "12345",
			Template:     "pvc-{{pvc.uid}}",
			Expected:     "pvc-12345",
		},
		{
			Name:         "WithoutVariables",
			PVCName:      "my-pvc",
			PVCNamespace: "my-namespace",
			PVCUID:       "12345",
			Template:     "pvc-without-variables",
			Expected:     "pvc-without-variables",
		},
		{
			Name:         "VariablesAndConstants",
			PVCName:      "my-pvc",
			PVCNamespace: "my-namespace",
			PVCUID:       "12345",
			Template:     "ns-{{pvc.namespace}}-pvc-{{pvc.name}}-uid-{{pvc.uid}}",
			Expected:     "ns-my-namespace-pvc-my-pvc-uid-12345",
		},
	}
	for _, tCase := range tMatrix {
		t.Run(tCase.Name, func(t *testing.T) {
			assert := assert.New(t)

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tCase.PVCName,
					Namespace: tCase.PVCNamespace,
					UID:       types.UID(tCase.PVCUID),
				},
			}
			result := evalPathTemplate(tCase.Template, pvc)
			assert.Equal(tCase.Expected, result, "Should match the expected path")
		})
	}
}

func TestCreateFilePath(t *testing.T) {
	cfg := storageConfig{
		BasePath:     "/base/path",
		PathTemplate: "{{pvc.namespace}}/{{pvc.name}}",
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pvc",
			Namespace: "my-namespace",
			UID:       types.UID("12345"),
		},
	}
	expected := "/base/path/my-namespace/my-pvc"
	result := createFilePath(cfg, pvc)
	assert.Equal(t, expected, result, "Should create the correct file path")
}

func TestNewStorageConfig(t *testing.T) {
	tMatrix := []struct {
		Name           string
		Parameters     map[string]string
		ExpectedConfig storageConfig
		ExpectError    string
	}{
		{
			Name:       "NoParameters",
			Parameters: map[string]string{},
			ExpectedConfig: storageConfig{
				BasePath:     DefaultBasePath,
				PathTemplate: defaultPathTemplate,
			},
		},
		{
			Name: "ValidParameters",
			Parameters: map[string]string{
				parameterBasePath:     "/custom/base/path",
				parameterPathTemplate: "custom/{{pvc.uid}}",
			},
			ExpectedConfig: storageConfig{
				BasePath:     "/custom/base/path",
				PathTemplate: "custom/{{pvc.uid}}",
			},
		},
		{
			Name: "EmptyBasePath",
			Parameters: map[string]string{
				parameterBasePath: "",
			},
			ExpectError: "basePath is required",
		},
		{
			Name: "EmptyPathTemplate",
			Parameters: map[string]string{
				parameterPathTemplate: "",
			},
			ExpectError: "pathTemplate is required",
		},
		{
			Name: "RelativeBasePath",
			Parameters: map[string]string{
				parameterBasePath: "relative/path",
			},
			ExpectError: "basePath must be an absolute path",
		},
		{
			Name: "InvalidPathTemplate",
			Parameters: map[string]string{
				parameterPathTemplate: "../pvc-{{pvc.uid}}",
			},
			ExpectError: "pathTemplate must evaluate to a relative path located within basePath",
		},
	}
	for _, tCase := range tMatrix {
		t.Run(tCase.Name, func(t *testing.T) {
			assert := assert.New(t)

			sc := &storagev1.StorageClass{
				Parameters: tCase.Parameters,
			}
			cfg, err := newStorageConfig(sc)
			if tCase.ExpectError != "" {
				assert.ErrorContains(err, tCase.ExpectError, "Error message should contain expected text")
			} else {
				assert.NoError(err, "Should not return an error")
				assert.Equal(tCase.ExpectedConfig, cfg, "Should match the expected storage config")
			}
		})
	}
}

func TestIsForCurrentNode(t *testing.T) {
	tMatrix := []struct {
		Name     string
		Affinity *corev1.VolumeNodeAffinity
		Expected bool
	}{
		{
			Name:     "NilAffinity",
			Affinity: nil,
			Expected: false,
		},
		{
			Name: "NilRequired",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: nil,
			},
			Expected: false,
		},
		{
			Name: "EmptyNodeSelectorTerms",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{},
				},
			},
			Expected: false,
		},
		{
			Name: "MultipleNodeSelectorTerms",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{},
						{},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "EmptyMatchExpressions",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "MultipleMatchExpressions",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{},
								{},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "EmptyValues",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "MultipleValues",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"node1", "node2"},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "WrongKey",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "wrong/key",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"node1"},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "WrongOperator",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpNotIn,
									Values:   []string{"node1"},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "NonMatchingNodeName",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"node2"},
								},
							},
						},
					},
				},
			},
			Expected: false,
		},
		{
			Name: "MatchingNodeName",
			Affinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"node1"},
								},
							},
						},
					},
				},
			},
			Expected: true,
		},
	}
	for _, tCase := range tMatrix {
		t.Run(tCase.Name, func(t *testing.T) {
			assert := assert.New(t)

			result := isForCurrentNode("node1", tCase.Affinity)
			assert.Equal(tCase.Expected, result, "Should match the expected result")
		})
	}
}

func TestGetProvisionerName(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		assert.Equal(t, defaultProvisionerName, getProvisionerName(), "Should return default provisioner name when env variable not set")
	})
	t.Run("Custom", func(t *testing.T) {
		t.Setenv(envProvisionerName, "test-provisioner")
		assert.Equal(t, "test-provisioner", getProvisionerName(), "Should return custom provisioner name from env variable")
	})
}

func pointer[T any](v T) *T {
	return &v
}
