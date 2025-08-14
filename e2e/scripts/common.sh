#!/bin/bash

export EMBEDDED_CLUSTER_BIN="${EMBEDDED_CLUSTER_BIN:-embedded-cluster-smoke-test-staging-app}"
export EMBEDDED_CLUSTER_BASE_DIR="${EMBEDDED_CLUSTER_BASE_DIR:-/var/lib/embedded-cluster}"
export PATH="$PATH:${EMBEDDED_CLUSTER_BASE_DIR}/bin"
export K0SCONFIG=/etc/k0s/k0s.yaml
export APP_NAMESPACE="${APP_NAMESPACE:-kotsadm}"

KUBECONFIG="${KUBECONFIG:-${EMBEDDED_CLUSTER_BASE_DIR}/k0s/pki/admin.conf}"
export KUBECONFIG

function retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** count))
        count=$((count + 1))
        if [ $count -lt "$retries" ]; then
            echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}

wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes || true
    done
    return 0
}

ensure_installation_is_installed() {
    echo "ensure that installation is installed"
    if ! kubectl get installations --no-headers | grep -q "Installed"; then
        echo "installation is not installed"
        kubectl get installations 2>&1 || true
        kubectl describe installations 2>&1 || true
        kubectl get charts -A
        kubectl get secrets -A
        kubectl describe clusterconfig -A
        kubectl get pods -A
        echo "node charts"
        kubectl get charts -A
        echo "node secrets"
        kubectl get secrets -A
        echo "node pods"
        kubectl get pods -A
        exit 1
    fi
}

wait_for_installation() {
    ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 84 ]; then
            echo "installation did not become ready"
            kubectl get installations 2>&1 || true
            kubectl describe installations 2>&1 || true
            kubectl get charts -A
            kubectl get secrets -A
            kubectl describe clusterconfig -A
            kubectl get pods -A
            echo "operator logs:"
            kubectl logs -n embedded-cluster -l app.kubernetes.io/name=embedded-cluster-operator --tail=100
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for installation"
        ready=$(kubectl get installations --no-headers | grep -c "Installed" || true)
        kubectl get installations 2>&1 || true
    done
}

wait_for_nginx_pods() {
    ready=$(kubectl get pods -n "$APP_NAMESPACE" | grep "nginx" | grep -c Running || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            echo "nginx pods did not appear"
            if [ "$APP_NAMESPACE" != "kotsadm" ]; then
                kubectl get pods -n kotsadm
            fi
            kubectl get pods -n "$APP_NAMESPACE"
            kubectl logs -n kotsadm -l app=kotsadm
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for nginx pods"
        ready=$(kubectl get pods -n "$APP_NAMESPACE" | grep "nginx" | grep -c Running || true)
        kubectl get pods -n nginx 2>&1 || true
        echo "ready: $ready"
    done
}

wait_for_pods_running() {
    local timeout="$1"
    local start_time
    local current_time
    local elapsed_time
    start_time=$(date +%s)
    while true; do
        current_time=$(date +%s)
        elapsed_time=$((current_time - start_time))
        if [ "$elapsed_time" -ge "$timeout" ]; then
            kubectl get pods -A || true
            kubectl describe nodes || true
            echo "Timed out waiting for all pods to be running."
            return 1
        fi
        local non_running_pods
        non_running_pods=$(kubectl get pods --all-namespaces --no-headers 2>/dev/null | awk '$4 != "Running" && $4 != "Completed" { print $0 }' | wc -l || echo 1)
        if [ "$non_running_pods" -ne 0 ]; then
            echo "Not all pods are running. Waiting."
            sleep 5
            continue
        fi
        echo "All pods are running."
        return 0
    done
}

wait_for_ingress_pods() {
     ready=$(kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}' | grep -c Running || true)
     counter=0
     while [ "$ready" -lt "1" ]; do
         if [ "$counter" -gt 36 ]; then
             echo "ingress pods did not appear"
             kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}'
             kubectl get pods -n ingress-nginx 2>&1 || true
             kubectl get secrets -n ingress-nginx 2>&1 || true
             kubectl get charts -A
             return 1
         fi
         sleep 5
         counter=$((counter+1))
         echo "Waiting for ingress pods"
         ready=$(kubectl get pods -n ingress-nginx -o jsonpath='{.items[*].status.phase}' | grep -c Running || true)
         kubectl get pods -n ingress-nginx 2>&1 || true
         echo "ready: $ready"
     done
 }

ensure_app_deployed() {
    local version="$1"

    kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app
    if ! kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app | grep -q "${version}\W*[01]\W*deployed"; then
        echo "application version ${version} not deployed"
        kubectl kots get versions -n kotsadm embedded-cluster-smoke-test-staging-app
        return 1
    fi
}

ensure_app_deployed_airgap() {
    local version="$1"

    echo "exporting authstring"
    # export the authstring secret
    local kotsadm_auth_string=
    kotsadm_auth_string=$(kubectl get secret -n kotsadm kotsadm-authstring -o jsonpath='{.data.kotsadm-authstring}' | base64 -d)
    echo "kotsadm_auth_string: $kotsadm_auth_string"

    echo "getting kotsadm service IP"
    # get kotsadm service IP address
    local kotsadm_ip=
    kotsadm_ip=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.clusterIP}')
    echo "kotsadm_ip: $kotsadm_ip"

    echo "getting kotsadm service port"
    # get kotsadm service port
    local kotsadm_port=
    kotsadm_port=$(kubectl get svc -n kotsadm kotsadm -o jsonpath='{.spec.ports[?(@.name=="http")].port}')
    echo "kotsadm_port: $kotsadm_port"

    echo "ensuring app version ${version} is deployed"
    local versions=
    versions=$(curl -k -X GET "http://${kotsadm_ip}:${kotsadm_port}/api/v1/app/embedded-cluster-smoke-test-staging-app/versions?currentPage=0&pageSize=1" -H "Authorization: $kotsadm_auth_string")
    # search for the version and that it is deployed
    # there should not be a '}' between the version and the status, as that would indicate a different version
    if ! echo "$versions" | grep -e "\"versionLabel\":\"${version}\"[^}]*\"status\":\"deployed\""; then
        echo "application version ${version} not deployed, current versions ${versions}"
        return 1
    fi
}

ensure_app_not_upgraded() {
    if kubectl get ns | grep -q gitea ; then
        echo "found gitea ns"
        return 1
    fi
    if kubectl get pods -n "$APP_NAMESPACE" -l app=second | grep -q second ; then
        echo "found pods from app update"
        return 1
    fi
}

ensure_installation_label() {
    # ensure that the installation has the kots backup label
    if ! kubectl get installations -l "replicated.com/disaster-recovery=ec-install" --no-headers; then
        echo "installation does not have the replicated.com/disaster-recovery=ec-install label"
        kubectl describe installations --no-headers
        return 1
    fi
}

# ensure_release_builtin_overrides verifies if the built in overrides we provide as part
# of the release have been applied to the helm charts.
ensure_release_builtin_overrides() {
    if ! kubectl get deployment -n kotsadm kotsadm -ojsonpath='{.metadata.labels}' | grep -q "release-custom-label"; then
        echo "release-custom-label not found in admin-console"
        kubectl get deployment -n kotsadm kotsadm -ojsonpath='{.metadata.labels}'
        kubectl get deployment -n kotsadm kotsadm -o yaml
        return 1
    fi
    if ! kubectl get deployment -n embedded-cluster embedded-cluster-operator -ojsonpath='{.metadata.labels}' | grep -q "release-custom-label"; then
        echo "release-custom-label not found in embedded-cluster-operator"
        kubectl get deployment -n embedded-cluster embedded-cluster-operator -ojsonpath='{.metadata.labels}'
        kubectl get deployment -n embedded-cluster embedded-cluster-operator -o yaml
        return 1
    fi
}

# ensure_release_builtin_overrides_postupgrade verifies if the built in overrides we provide as part
# of the upgrade release have been applied to the helm charts.
ensure_release_builtin_overrides_postupgrade() {
    # postugrade includes the same overrides as install (and also an extra one)
    if ! ensure_release_builtin_overrides; then
        return 1
    fi

    if ! kubectl get deployment -n kotsadm kotsadm -ojsonpath='{.metadata.labels}' | grep -q "second-custom-label"; then
        echo "second-custom-label not found in admin-console"
        kubectl get deployment -n kotsadm kotsadm -ojsonpath='{.metadata.labels}'
        kubectl get deployment -n kotsadm kotsadm -o yaml
        return 1
    fi

    if ! kubectl get deployment -n embedded-cluster embedded-cluster-operator -ojsonpath='{.metadata.labels}' | grep -q "second-custom-label"; then
        echo "second-custom-label not found in embedded-cluster-operator"
        kubectl get deployment -n embedded-cluster embedded-cluster-operator -ojsonpath='{.metadata.labels}'
        kubectl get deployment -n embedded-cluster embedded-cluster-operator -o yaml
        return 1
    fi
}

# ensure_version_metadata_present verifies if a configmap containing the embedded cluster version
# metadata is present in the embedded-cluster namespace. this configmap should always exist.
ensure_version_metadata_present() {
    echo "ensure that versions configmap is present"
    if ! kubectl get cm -n embedded-cluster | grep -q version-metadata-; then
        echo "version metadata configmap not found"
        kubectl get cm -n embedded-cluster
        return 1
    fi
    local name
    name=$(kubectl get cm -n embedded-cluster | grep version-metadata- | awk '{print $1}')
    if ! kubectl get cm -n embedded-cluster "$name" -o yaml | grep -q Versions ; then
        echo "version metadata configmap does not contain Versions entry"
        kubectl get cm -n embedded-cluster "$name" -o yaml
        return 1
    fi
}

# ensure_binary_copy verifies that the installer is copying itself to the default location of
# binaries on the node.
ensure_binary_copy() {
    if ! ls "${EMBEDDED_CLUSTER_BASE_DIR}/bin/${EMBEDDED_CLUSTER_BIN}" ; then
        echo "embedded-cluster binary not found at default location"
        ls -la "${EMBEDDED_CLUSTER_BASE_DIR}/bin"
        return 1
    fi
    if ! "${EMBEDDED_CLUSTER_BASE_DIR}/bin/${EMBEDDED_CLUSTER_BIN}" version ; then
        echo "embedded-cluster binary is not executable"
        return 1
    fi
}

ensure_license_in_data_dir() {
    local expected_license_path="$EMBEDDED_CLUSTER_BASE_DIR/license.yaml"
    if [ -e "$expected_license_path" ]; then
        echo "license file exists in $expected_license_path"
    else
        echo "license file does not exist in $expected_license_path"
        return 1
    fi
}

ensure_copy_host_preflight_results_configmap() {
    if ! kubectl get configmap -n embedded-cluster -l embedded-cluster/host-preflight-result --no-headers 2>/dev/null | grep -q host-preflight-results ; then
        echo "ConfigMap with label embedded-cluster/host-preflight-result not found in embedded-cluster namespace"
        kubectl get configmap -n embedded-cluster
        return 1
    fi
    return 0
}

ensure_node_config() {
    if ! kubectl describe node | grep "controller-label" ; then
        echo "Failed to find controller-label"
        return 1
    fi

    if ! kubectl describe node | grep "controller-test" ; then
        echo "Failed to find controller-test"
        return 1
    fi
}

ensure_nodes_match_kube_version() {
    local version="$1"
    if kubectl get nodes -o jsonpath='{.items[*].status.nodeInfo.kubeletVersion}' | grep -v "$version"; then
        echo "Node kubelet version does not match expected version $version"
        kubectl get nodes -o jsonpath='{.items[*].status.nodeInfo.kubeletVersion}'
        kubectl get nodes
        return 1
    fi
}

check_pod_install_order() {
    local ingress_install_time=
    ingress_install_time=$(kubectl get pods --no-headers=true -n ingress-nginx -o jsonpath='{.items[*].metadata.creationTimestamp}' | sort | head -n 1)

    local openebs_install_time=
    openebs_install_time=$(kubectl get pods --no-headers=true -n openebs -o jsonpath='{.items[*].metadata.creationTimestamp}' | sort | head -n 1)

    echo "ingress_install_time: $ingress_install_time"
    echo "openebs_install_time: $openebs_install_time"

    if [[ "$ingress_install_time" < "$openebs_install_time" ]]; then
        echo "Ingress pods were installed before openebs pods"
        return 1
    fi
}

has_stored_host_preflight_results() {
    if [ ! -f "${EMBEDDED_CLUSTER_BASE_DIR}/support/host-preflight-results.json" ]; then
        return 1
    fi
}

install_kots_cli() {
    if command -v kubectl-kots; then
        return
    fi

    maybe_install_curl

    # install kots CLI
    echo "installing kots cli"
    local ec_version=
    ec_version=$(embedded-cluster version | grep AdminConsole | awk '{print substr($4,2)}' | cut -d'-' -f1)
    curl --retry 5 -fL -o /tmp/kotsinstall.sh "https://kots.io/install/$ec_version"
    chmod +x /tmp/kotsinstall.sh
    /tmp/kotsinstall.sh
}

maybe_install_curl() {
    if ! command -v curl; then
        apt-get update
        apt-get install -y curl
    fi
}

validate_data_dirs() {
    local expected_datadir="$EMBEDDED_CLUSTER_BASE_DIR"
    local expected_k0sdatadir="$EMBEDDED_CLUSTER_BASE_DIR/k0s"
    local expected_openebsdatadir="$EMBEDDED_CLUSTER_BASE_DIR/openebs-local"
    if [ "$KUBECONFIG" = "/var/lib/k0s/pki/admin.conf" ]; then
        expected_k0sdatadir=/var/lib/k0s
        expected_openebsdatadir=/var/openebs
    fi

    local fail=0

    if kubectl -n kube-system get charts k0s-addon-chart-openebs -oyaml >/dev/null 2>&1 ; then
        echo "found openebs chart"

        openebsdatadir=$(kubectl -n kube-system get charts k0s-addon-chart-openebs -oyaml | grep -v apiVersion | grep "basePath:" | awk '{print $2}')
        echo "found openebsdatadir $openebsdatadir, want $expected_openebsdatadir"
        if [ "$openebsdatadir" != "$expected_openebsdatadir" ]; then
            echo "got unexpected openebsdatadir $openebsdatadir, want $expected_openebsdatadir"
            kubectl -n kube-system get charts k0s-addon-chart-openebs -oyaml | grep -v apiVersion | grep "basePath:" -A5 -B5
            fail=1
        else
            echo "validated openebsdatadir $openebsdatadir"
        fi
    else
        echo "did not find openebs chart"
    fi

    if kubectl -n kube-system get charts k0s-addon-chart-seaweedfs -oyaml >/dev/null 2>&1 ; then
        echo "found seaweedfs chart"

        seaweefdatadir=$(kubectl -n kube-system get charts k0s-addon-chart-seaweedfs -oyaml| grep -v apiVersion | grep -m 1 "hostPathPrefix:" | awk '{print $2}')
        echo "found seaweefdatadir $seaweefdatadir, want $expected_datadir/seaweedfs/(ssd|storage)"
        if ! echo "$seaweefdatadir" | grep -qE "^$expected_datadir/seaweedfs/(ssd|storage)$" ; then
            echo "got unexpected seaweefdatadir $seaweefdatadir, want $expected_datadir/seaweedfs/(ssd|storage)"
            kubectl -n kube-system get charts k0s-addon-chart-seaweedfs -oyaml| grep -v apiVersion | grep -m 1 "hostPathPrefix:" -A5 -B5
            fail=1
        else
            echo "validated seaweefdatadir $seaweefdatadir"
        fi
    else
        echo "did not find seaweedfs chart"
    fi

    if kubectl -n kube-system get charts k0s-addon-chart-velero -oyaml >/dev/null 2>&1 ; then
        echo "found velero chart"

        velerodatadir=$(kubectl -n kube-system get charts k0s-addon-chart-velero -oyaml | grep -v apiVersion | grep "podVolumePath:" | awk '{print $2}')
        echo "found velerodatadir $velerodatadir, want $expected_k0sdatadir/kubelet/pods"
        if [ "$velerodatadir" != "$expected_k0sdatadir/kubelet/pods" ]; then
            echo "got unexpected velerodatadir $velerodatadir, want $expected_openebsdatadir/kubelet/pods"
            kubectl -n kube-system get charts k0s-addon-chart-velero -oyaml | grep -v apiVersion | grep "podVolumePath:" -A5 -B5
            fail=1
        else
            echo "validated velerodatadir $velerodatadir"
        fi
    else
        echo "did not find velero chart"
    fi

    if [ "$fail" -eq 1 ]; then
        echo "data dir validation failed"
        exit 1
    else
        echo "data dir validation succeeded"
    fi
}

validate_non_job_pods_healthy() {
    local unhealthy_pods
    local unready_pods
    
    # Check for environment variable override (used by specific tests)
    if [ "${ALLOW_PENDING_PODS:-}" = "true" ]; then
        # Allow Running, Completed, Succeeded, Pending
        unhealthy_pods=$(kubectl get pods -A --no-headers -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,OWNER:.metadata.ownerReferences[0].kind" | \
            awk '$4 != "Job" && ($3 != "Running" && $3 != "Completed" && $3 != "Succeeded" && $3 != "Pending") { print $1 "/" $2 " (" $3 ")" }')
        echo "All non-Job pods are healthy (allowing Pending pods)"
    else
        # Default: only allow Running, Completed, Succeeded  
        unhealthy_pods=$(kubectl get pods -A --no-headers -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,OWNER:.metadata.ownerReferences[0].kind" | \
            awk '$4 != "Job" && ($3 != "Running" && $3 != "Completed" && $3 != "Succeeded") { print $1 "/" $2 " (" $3 ")" }')
        echo "All non-Job pods are healthy"
    fi
    
    # Check container readiness for Running pods (skip Completed/Succeeded pods as they don't need to be ready)
    unready_pods=$(kubectl get pods -A --no-headers -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase,READY:.status.containerStatuses[*].ready,OWNER:.metadata.ownerReferences[0].kind" | \
        awk '$5 != "Job" && $3 == "Running" && ($4 == "" || $4 !~ /^(true[[:space:]]*)*$/) { print $1 "/" $2 " (not ready)" }')
    
    local has_issues=0
    
    if [ -n "$unhealthy_pods" ]; then
        echo "found non-Job pods in unhealthy state:"
        echo "$unhealthy_pods"
        has_issues=1
    fi
    
    if [ -n "$unready_pods" ]; then
        echo "found non-Job pods that are Running but not ready:"
        echo "$unready_pods"
        has_issues=1
    fi
    
    if [ $has_issues -eq 1 ]; then
        return 1
    fi
    
    return 0
}

validate_jobs_completed() {
    local incomplete_jobs
    # Check that all Jobs have succeeded (status.succeeded should equal spec.completions)
    # Flag any job that hasn't fully succeeded
    incomplete_jobs=$(kubectl get jobs -A --no-headers -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,COMPLETIONS:.spec.completions,SUCCESSFUL:.status.succeeded" | \
        awk '$4 != $3 { print $1 "/" $2 " (succeeded: " $4 "/" $3 ")" }')
    
    if [ -n "$incomplete_jobs" ]; then
        echo "found Jobs that have not completed successfully:"
        echo "$incomplete_jobs"
        echo ""
        echo "Job details:"
        kubectl get jobs -A
        return 1
    fi
    echo "All Jobs have completed successfully"
    return 0
}

validate_all_pods_healthy() {
    local timeout=300  # 5 minutes
    local start_time
    local current_time
    local elapsed_time
    start_time=$(date +%s)
    
    # Show what mode we're in
    if [ "${ALLOW_PENDING_PODS:-}" = "true" ]; then
        echo "Validating pod and job health (allowing Pending pods)..."
    else
        echo "Validating pod and job health (default: Running, Completed, Succeeded)..."
    fi
    
    while true; do
        current_time=$(date +%s)
        elapsed_time=$((current_time - start_time))
        
        if [ "$elapsed_time" -ge "$timeout" ]; then
            echo "Timed out waiting for pods and jobs to be healthy after 5 minutes"
            
            # Show detailed failure info
            validate_non_job_pods_healthy || true
            echo ""
            validate_jobs_completed || true
            
            return 1
        fi
        
        # Check if both validations pass
        local pods_healthy=0
        local jobs_healthy=0
        
        if validate_non_job_pods_healthy >/dev/null 2>&1; then
            pods_healthy=1
        fi
        
        if validate_jobs_completed >/dev/null 2>&1; then
            jobs_healthy=1
        fi
        
        if [ $pods_healthy -eq 1 ] && [ $jobs_healthy -eq 1 ]; then
            echo "All pods and jobs are healthy"
            return 0
        fi
        
        echo "Waiting for pods and jobs to be healthy... (${elapsed_time}s elapsed)"
        sleep 10
    done
}

validate_worker_profile() {
    # if /etc/systemd/system/k0scontroller.service exists, check it - otherwise check /etc/systemd/system/k0sworker.service
    if [ -f /etc/systemd/system/k0scontroller.service ]; then
        if ! grep -- "--profile=ip-forward" /etc/systemd/system/k0scontroller.service >/dev/null; then
            echo "expected worker profile 'ip-forward' not found in k0scontroller.service"
            exit 1
        fi
    elif [ -f /etc/systemd/system/k0sworker.service ]; then
        if ! grep -- "--profile=ip-forward" /etc/systemd/system/k0sworker.service >/dev/null; then
            echo "expected worker profile 'ip-forward' not found in k0sworker.service"
            exit 1
        fi
    else
        echo "expected k0scontroller.service or k0sworker.service not found"
        exit 1
    fi
}

# check that the 'join print-command returns a command that contains 'sudo' and 'join'
check_join_command() {
    output=$(embedded-cluster join print-command)
    if ! echo "$output" | grep "sudo" | grep -q "join"; then
        echo "join command does not contain 'sudo' or 'join'"
        echo "$output"
        exit 1
    fi
}
