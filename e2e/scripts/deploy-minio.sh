#!/usr/bin/env bash
set -euo pipefail

# This YAML configures a job that mounts the minio secret to obtain the access key and secret key and then
# runs the minio client to create a bucket called 'embedded-cluster-releases'. Once the bucket is created
# the job then downloads the embedded cluster version 1.28.2+ec.0 and uploads it to the bucket. Most of the
# things here are hardcoded as this is meant to be used against the minio instance deployed by this script
# itself.
create_embedded_cluster_releases_job="
apiVersion: batch/v1
kind: Job
metadata:
  name: minio-create-bucket-job
  namespace: default
spec:
  backoffLimit: 100
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
        - name: minio-credentials
          mountPath: /minio-credentials
          readOnly: true
        - name: node-bin
          mountPath: /node-bin
        command: ['/bin/sh', '-c']
        args:
        - >
          . /minio-credentials/config.env &&
          apt-get update -y &&
          apt-get install -y wget &&
          wget https://dl.min.io/client/mc/release/linux-amd64/mc &&
          chmod 755 mc &&
          mv mc /usr/local/bin &&
          mc alias set minio https://minio.default.svc.cluster.local \$MINIO_ROOT_USER \$MINIO_ROOT_PASSWORD &&
          mc mb minio/embedded-cluster-releases &&
          tar -czf v99.99.99+ec.99.tgz -C /node-bin embedded-cluster &&
          mc cp ./v99.99.99+ec.99.tgz minio/embedded-cluster-releases
      volumes:
      - name: minio-credentials
        secret:
          secretName: minio-tenant-env-configuration
      - name: node-bin
        hostPath:
          path: /usr/local/bin
          type: Directory
"

# Installs helm as we are going to need it to deploy minio operator. Uses the latest version of helm and
# installs it in the PATH.
install_helm() {
    if ! curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 ; then
        return 1
    fi
    chmod 700 get_helm.sh
    if ! ./get_helm.sh ; then
        return 1
    fi
    return 0
}

# Installs the minio operator using helm. The operator is installed in the minio-operator namespace.
deploy_minio_operator() {
    if ! helm repo add minio-operator https://operator.min.io; then
        return 1
    fi
    if ! helm install --namespace minio-operator --wait --create-namespace minio-operator minio-operator/operator ; then
        return 1
    fi
    return 0
}

# To deploy a tenant (instance of minio) we use the krew kubectl plugin. This function installs krew and
# then uses it to install the kubectl-minio plugin. Once the plugin is installed, we can use it to deploy
# a tenant. Tenant is deployed in the default namespace.
# For reference please see: https://krew.sigs.k8s.io/docs/user-guide/setup/install/
deploy_minio_tenant() {
    (
        KREW="krew-linux_amd64" &&
        curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" &&
        tar zxvf "${KREW}.tar.gz" &&
        ./"${KREW}" install krew
    )
    export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$PATH"
    if ! kubectl krew install minio ; then
        return 1
    fi
    if ! kubectl minio tenant create minio-tenant --servers 1 --volumes 1 --capacity 10Gi --namespace default ; then
        return 1
    fi
    return 0
}

# Here we create a bucket that is expected by the embedded cluster builder service. This may be not needed
# for other kinds of tests but we always create it as it is harmless.
create_minio_bucket() {
    echo "$create_embedded_cluster_releases_job" > create_embedded_cluster_releases_job.yaml
    if ! kubectl apply -f create_embedded_cluster_releases_job.yaml ; then
        echo "Failed to create embedded-cluster-releases bucket: "
        cat create_embedded_cluster_releases_job.yaml
        return 1
    fi
    if ! kubectl wait --for=condition=complete --timeout=3m job/minio-create-bucket-job ; then
        echo "Job did not complete successfully in time"
        return 1
    fi
    return 0
}

main() {
    # Sleeps a little bit as the cluster may not be ready yet. The last step of the single node cluster
    # installation script is a restart in the embedded cluster service so if this script is called
    # right after the installation, the restart may not be finished yet.
    sleep 30

    if ! install_helm; then
        echo "Failed to install helm"
        exit 1
    fi
    if ! deploy_minio_operator; then
        echo "Failed to deploy minio operator"
        exit 1
    fi
    if ! deploy_minio_tenant; then
        echo "Failed to deploy minio tenant"
        exit 1
    fi
    if ! create_minio_bucket; then
        echo "Failed to create minio bucket"
        exit 1
    fi
}

export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/.embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/.embedded-cluster/bin
main
