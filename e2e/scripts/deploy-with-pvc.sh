#!/usr/bin/env bash
set -euo pipefail

deploy="
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      volumes:
      - name: nginx-persistent-storage
        persistentVolumeClaim:
          claimName: nginx-pvc
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
        volumeMounts:
        - name: nginx-persistent-storage
          mountPath: /usr/share/nginx/html
"

pvc="
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nginx-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
"

create_deployment() {
    echo "$deploy" > deploy.yaml
    echo "$pvc" > pvc.yaml
    if ! kubectl apply -f pvc.yaml 2>&1; then
        echo "Failed to create PVC"
        return 1
    fi
    if ! kubectl apply -f deploy.yaml 2>&1; then
        echo "Failed to create PVC"
        return 1
    fi
}

wait_for_nginx_pods() {
    running=$(kubectl get pods | grep nginx | grep -c Running || true)
    counter=0
    while [ "$running" -lt "1" ]; do
        if [ "$counter" -gt 30 ]; then
            kubectl describe pvc nginx-pvc 2>&1 || true
            kubectl describe deploy nginx-deployment 2>&1 || true
            return 1
        fi
        sleep 10
        counter=$((counter+1))
        echo "Waiting for nginx pods to run"
        running=$(kubectl get pods | grep nginx | grep -c Running || true)
        kubectl get pods || true
    done
    return 0
}

main() {
    echo "Creating deployment with mounted PVC"
    if ! create_deployment; then
        echo "Failed to create deployment"
        return 1
    fi
    echo "Waiting for deployment to rollout"
    if ! wait_for_nginx_pods; then
        kubectl get pods -A 2>&1 || true
        echo "Failed to wait for nginx pods"
        exit 1
    fi
    echo "Deployment created successfully"
}

export KUBECONFIG=/root/.helmvm/etc/kubeconfig
export PATH=$PATH:/root/.helmvm/bin
main
