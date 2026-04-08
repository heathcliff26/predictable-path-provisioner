[![CI](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/ci.yaml/badge.svg?event=push)](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/ci.yaml)
[![Coverage Status](https://coveralls.io/repos/github/heathcliff26/predictable-path-provisioner/badge.svg)](https://coveralls.io/github/heathcliff26/predictable-path-provisioner)
[![Editorconfig Check](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/editorconfig-check.yaml/badge.svg?event=push)](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/editorconfig-check.yaml)
[![Generate go test cover report](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/go-testcover-report.yaml/badge.svg)](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/go-testcover-report.yaml)
[![Renovate](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/renovate.yaml/badge.svg)](https://github.com/heathcliff26/predictable-path-provisioner/actions/workflows/renovate.yaml)

# P3 (Predictable Path Provisioner)

A Kubernetes external dynamic provisioner that creates local PersistentVolumes under human-readable folder paths.

## Table of Contents

- [P3 (Predictable Path Provisioner)](#p3-predictable-path-provisioner)
  - [Table of Contents](#table-of-contents)
  - [Important considerations](#important-considerations)
  - [Installation](#installation)
    - [Using helm](#using-helm)
    - [Using kubectl](#using-kubectl)
  - [Usage](#usage)
    - [Example storage classes](#example-storage-classes)
      - [With defaults:](#with-defaults)
      - [With custom basePath and pathTemplate:](#with-custom-basepath-and-pathtemplate)
  - [Container Images](#container-images)
    - [Image location](#image-location)
    - [Tags](#tags)

## Important considerations

- There is mechanism to prevent path collisions. Use custom name templates at your own risk.
- This is mostly a hobby project, intended for my own use. Use at your own risk.
- Only AccessModes `ReadWriteOnce` and `ReadWriteOncePod` is supported.

## Installation

### Using helm

P3 helm charts are released via oci repos and can be installed with:
```bash
helm install p3 oci://ghcr.io/heathcliff26/manifests/predictable-path-provisioner --version <version>
```
Please use the latest version from the [releases page](https://github.com/heathcliff26/predictable-path-provisioner/releases).

### Using kubectl
An example deployment can be found [here](examples/p3.yaml).

To deploy it to your cluster, run:
```bash
kubectl apply -f https://raw.githubusercontent.com/heathcliff26/predictable-path-provisioner/main/examples/p3.yaml
```

This will deploy the app to your cluster into the namespace `p3`.

## Usage

By default P3 uses the provisioner name `heathcliff.eu/predictable-path-provisioner`.

It's behaviour can be configured via the StorageClass parameters. The following parameters are supported:

| Parameter  | Default                                 | Description                                                                                           |
| ---------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `basePath` | `/var/lib/predictable-path-provisioner` | The base directory on the host under which the volumes will be created. Needs to be an absolute path. |
| TODO: `pathTemplate` | `{{pvc.Namespace}}/{{pvc.Name}}` | The template for the volume folder. It accepts the following templates (case sensitive): `pvc.name`, `pvc.namespace`, `pvc.uid` |

### Example storage classes

#### With defaults:
```
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: "p3"
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: heathcliff.eu/predictable-path-provisioner
reclaimPolicy: Delete
volumeBindingMode: Immediate
```
This would result in the Persistent Volume for the Claim `default/test` being created under `/var/lib/predictable-path-provisioner/default/test`.

#### With custom basePath and pathTemplate:
```
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: "p3"
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: heathcliff.eu/predictable-path-provisioner
parameters:
    basePath: /data/p3
    pathTemplate: pvc-{{pvc.uid}}
reclaimPolicy: Delete
volumeBindingMode: Immediate
```
This would result in the Persistent Volume being created under `/data/p3/pvc-<some-uid>`

## Container Images

### Image location

| Container Registry                                                                      | Image                       |
| --------------------------------------------------------------------------------------- | --------------------------- |
| [Github Container](https://github.com/users/heathcliff26/packages/container/package/p3) | `ghcr.io/heathcliff26/p3`   |
| [Docker Hub](https://hub.docker.com/r/heathcliff26/p3)                                  | `docker.io/heathcliff26/p3` |
| [Quay.io](https://quay.io/heathcliff26/p3)                                              | `quay.io/heathcliff26/p3`   |

### Tags

There are different flavors of the image:

| Tag(s)      | Description                                                 |
| ----------- | ----------------------------------------------------------- |
| **latest**  | Last released version of the image                          |
| **rolling** | Rolling update of the image, always build from main branch. |
| **vX.Y.Z**  | Released version of the image                               |
