#!/usr/bin/env bash
set -euo pipefail

# These are the artifacts necessary to get the embedded cluster builder running. We
# need access to an s3 compatible object store so we use minio. This script depends
# on the `deploy-minio.sh` script to be run first. The embedded cluster image is
# passed in as the first argument.
builder_manifests="
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: embedded-cluster-builder
  namespace: default
spec:
  serviceName: embedded-cluster-builder
  replicas: 1
  selector:
    matchLabels:
      app: embedded-cluster-builder
  template:
    metadata:
      labels:
        app: embedded-cluster-builder
    spec:
      volumes:
      - name: minio-credentials
        secret:
          secretName: minio-tenant-env-configuration
      containers:
      - name: embedded-cluster-builder
        image: BUILDER_IMAGE
        env:
        - name: EMBEDDED_CLUSTER_RELEASE_BUCKET_NAME
          value: embedded-cluster-releases
        - name: SSL_CERT_FILE
          value: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: releases
          mountPath: /releases
        - name: minio-credentials
          mountPath: /minio-credentials
          readOnly: true
        command: ['/bin/sh', '-c']
        args:
        - >
          . /minio-credentials/config.env &&
          export AWS_ACCESS_KEY_ID=\$MINIO_ROOT_USER &&
          export AWS_SECRET_ACCESS_KEY=\$MINIO_ROOT_PASSWORD &&
          export AWS_REGION=us-east-1 &&
          export AWS_ENDPOINT_URL=https://minio.default.svc.cluster.local &&
          ./builder
  volumeClaimTemplates:
  - metadata:
      name: releases
    spec:
      accessModes: ['ReadWriteOnce']
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: embedded-cluster-builder
  labels:
    app: embedded-cluster-builder
spec:
  type: NodePort
  selector:
    app: embedded-cluster-builder
  ports:
    - port: 8080
      targetPort: 8080
      nodePort: 30001
"

# Deploys the embedded cluster builder using the BUILDER_IMAGE image. The builder will be exposed
# as a NodePort service on port 30001. This scripts depends on the `deploy-minio.sh` script to be
# run first as it deployes a minio instance in the default namespace.
main() {
    # shellcheck disable=SC2001
    echo "$builder_manifests" | sed  "s|BUILDER_IMAGE|$BUILDER_IMAGE|" > builder-manifests.yaml
    if ! kubectl apply -f builder-manifests.yaml ; then
        echo "Failed to deploy embedded cluster builder assets"
        exit 1
    fi
    if ! kubectl rollout status statefulset/embedded-cluster-builder --timeout=3m ; then
        echo "Failed to deploy embedded cluster builder assets"
        exit 1
    fi
}

export BUILDER_IMAGE=$1
export EMBEDDED_CLUSTER_METRICS_BASEURL="https://staging.replicated.app"
export KUBECONFIG=/root/.config/embedded-cluster/etc/kubeconfig
export PATH=$PATH:/root/.config/embedded-cluster/bin
main
