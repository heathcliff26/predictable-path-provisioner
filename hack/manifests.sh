#!/bin/bash

set -e

base_dir="$(dirname "${BASH_SOURCE[0]}" | xargs realpath)/.."

export REPOSITORY="${REPOSITORY:-ghcr.io/heathcliff26}"
export TAG="${TAG:-latest}"
export P3_NAMESPACE="${P3_NAMESPACE:-p3}"

output_dir="${base_dir}/manifests/release"
deployment_file="${output_dir}/p3.yaml"

if [[ "${RELEASE_VERSION}" != "" ]] && [[ "${TAG}" == "latest" ]]; then
    TAG="${RELEASE_VERSION}"
fi

[ ! -d "${output_dir}" ] && mkdir "${output_dir}"

echo "Creating manifest from helm chart"
cat > "${deployment_file}" <<EOF
---
apiVersion: v1
kind: Namespace
metadata:
EOF
echo "  name: ${P3_NAMESPACE}" >> "${deployment_file}"

helm template "${base_dir}/manifests/helm" \
    --debug \
    --set fullnameOverride=p3 \
    --set image.repository="${REPOSITORY}/p3" \
    --set image.tag="${TAG}" \
    --name-template p3 \
    --namespace "${P3_NAMESPACE}" \
    | grep -v '# Source: predictable-path-provisioner/templates/' \
    | grep -v 'helm.sh/chart: predictable-path-provisioner' \
    | grep -v 'app.kubernetes.io/managed-by: Helm' \
    | sed "s/v0.0.0/${TAG}/g" >> "${deployment_file}"

echo "Wrote manifests to ${output_dir}"

if [ "${TAG}" == "latest" ]; then
    echo "Tag is latest, syncing manifests with examples"
    cp "${output_dir}"/*.yaml "${base_dir}/examples/"
fi
