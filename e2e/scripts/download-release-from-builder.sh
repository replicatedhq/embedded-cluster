#!/usr/bin/env bash
set -euo pipefail

create_download_release_from_builder_job="
apiVersion: batch/v1
kind: Job
metadata:
  name: download-release-from-builder
  namespace: default
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: minio-client
        image: ubuntu:latest
        env:
        - name: SSL_CERT_FILE
          value: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        volumeMounts:
        - name: outside-world
          mountPath: /outside-world
        command: ['/bin/sh', '-c']
        args:
        - >
          apt-get update -y &&
          apt-get install -y curl jq &&
          curl -s -X POST -o /outside-world/response.json --data /outside-world/request.json http://10.0.0.2:8080/build &&
          export URL=\$(jq -r .url /outside-world/response.json) &&
          curl -s -o /outside-world/release.tgz \$URL
      volumes:
      - name: outside-world
        hostPath:
          path: /root/results
          type: Directory
"

download_release_from_builder() {
    mkdir -p /root/results
    echo "$create_download_release_from_builder_job" > create_download_release_from_builder_job.yaml
    if ! kubectl apply -f create_download_release_from_builder_job ; then
        echo "Failed to create download release from builder bucket: "
        return 1
    fi
    if ! kubectl wait --for=condition=complete --timeout=5m job/download-release-from-builder; then
        echo "Job did not complete successfully in time"
        return 1
    fi
}

create_kots_release() {
    mkdir -p kots-release
    echo "$create_download_release_from_builder_job" > kots-release/job.yaml
    if ! tar -czvf release.tar.gz -C kots-release ./* ; then
        echo "Failed to create kots release"
        return 1
    fi
}

create_request_json() {
    release_content=$(base64 -w 0 < release.tar.gz)
    cat <<EOF > /root/results/request.json
{
        "binaryName": "my-release",
        "embeddedClusterVersion": "99.99.99+ec.99",
        "licenseID": "my-license",
        "kotsRelease": "$release_content"
        "kotsReleaseVersion": "1.0.0"
}
EOF
}

main() {
    if ! create_kots_release ; then
        echo "Failed to create kots release"
        return 1
    fi
    if ! create_request_json ; then
        echo "Failed to create request json"
        return 1
    fi
    if ! download_release_from_builder; then
        echo "Failed to download release from builder bucket"
        return 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
