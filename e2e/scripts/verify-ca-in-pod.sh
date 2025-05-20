#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh || echo "common.sh not found, continuing..."

main() {
    local ca_cert="$1"
    local server_cert="$2"
    local pod_namespace="kotsadm"
    local pod_label="app=kotsadm"

    # Validate parameters
    if [ -z "$ca_cert" ] || [ -z "$server_cert" ]; then
        echo "Error: Missing required parameters"
        echo "Usage: $0 CA_CERTIFICATE SERVER_CERTIFICATE"
        return 1
    fi

    echo "Checking if CA certificate is properly mounted in kotsadm pod"
    
    # Extract CA hash for searching - we need this even though it's host-side
    CA_HASH=$(openssl x509 -in "$ca_cert" -noout -hash)
    echo "CA Hash: $CA_HASH"
    
    # Find the kotsadm pod
    local pod_name
    pod_name=$(kubectl get pods -n "$pod_namespace" -l "$pod_label" -o jsonpath='{.items[0].metadata.name}')
    if [ -z "$pod_name" ]; then
        echo "Error: kotsadm pod not found"
        return 1
    fi
    echo "Found kotsadm pod: $pod_name"
    
    # Extract CA bundle from pod
    echo "Extracting CA bundle from pod"
    kubectl exec -n "$pod_namespace" "$pod_name" -- cat /etc/ssl/certs/ca-certificates.crt > /tmp/pod-ca-bundle.crt
    
    # Verify our CA is in the pod's bundle
    echo "Checking if our CA is in the pod's bundle"
    if grep -q "$CA_HASH" /tmp/pod-ca-bundle.crt; then
        echo "CA found in pod's CA bundle (matched by hash)"
    else
        echo "Error: Our CA is not found in the pod's CA bundle"
        # Additional diagnostics if needed
        return 1
    fi
    
    # Verify server certificate is trusted using the pod's CA bundle
    echo "Verifying that the server certificate is trusted via pod's CA bundle"
    if openssl verify -CAfile /tmp/pod-ca-bundle.crt "$server_cert" 2>/dev/null; then
        echo "Server certificate is trusted using pod's CA bundle"
    else
        echo "Error: Server certificate is not trusted by the pod's CA bundle"
        echo "This indicates the CA was not properly mounted or is not functioning correctly"
        return 1
    fi
    
    # Cleanup
    rm -f /tmp/pod-ca-bundle.crt 2>/dev/null || true
    
    echo "Success: CA is properly mounted and trusted in the kotsadm pod"
    return 0
}

main "$@" 