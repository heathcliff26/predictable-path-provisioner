# predictable-path-provisioner Helm Chart

This Helm chart for P3 (Predictable Path Provisioner) - K8s hostpath provisioner with human readable paths

## Prerequisites

- Kubernetes 1.35+
- Helm 3.19+
- FluxCD installed in the cluster (recommended)

## Installation

### Installing from OCI Registry (GitHub Packages)

```bash
# Install the chart
helm install predictable-path-provisioner oci://ghcr.io/heathcliff26/manifests/predictable-path-provisioner --version <version>
```

## Configuration

## Values Reference

See [values.yaml](./values.yaml) for all available configuration options.

### Key Parameters

| Parameter                | Description                                              | Default                                  |
| ------------------------ | -------------------------------------------------------- | ---------------------------------------- |
| `image.repository`       | Container image repository                               | `ghcr.io/heathcliff26/p3` |
| `image.tag`              | Container image tag                                      | Same as chart version                    |

## Support

For more information, visit: https://github.com/heathcliff26/predictable-path-provisioner
