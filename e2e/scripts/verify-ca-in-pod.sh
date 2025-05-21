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
    
    # Extract CA info for searching
    CA_HASH=$(openssl x509 -in "$ca_cert" -noout -hash)
    CA_SUBJECT=$(openssl x509 -in "$ca_cert" -noout -subject)
    echo "CA Hash: $CA_HASH"
    echo "CA Subject: $CA_SUBJECT"
    
    # Find the kotsadm pod
    local pod_name
    pod_name=$(kubectl get pods -n "$pod_namespace" -l "$pod_label" -o jsonpath='{.items[0].metadata.name}')
    if [ -z "$pod_name" ]; then
        echo "Error: kotsadm pod not found"
        return 1
    fi
    echo "Found kotsadm pod: $pod_name"
    
    # First verify the SSL_CERT_DIR environment variable is set correctly
    echo "Checking for SSL_CERT_DIR environment variable in pod"
    if ! kubectl exec -n "$pod_namespace" "$pod_name" -- env | grep -q "SSL_CERT_DIR=/certs"; then
        echo "Error: SSL_CERT_DIR environment variable not set correctly in the pod"
        kubectl exec -n "$pod_namespace" "$pod_name" -- env | grep SSL
        return 1
    fi
    echo "SSL_CERT_DIR environment variable is set correctly"
    
    # Check for the mounted CA certificate file
    echo "Checking for mounted CA certificate file"
    if ! kubectl exec -n "$pod_namespace" "$pod_name" -- ls -la /certs/ca-certificates.crt >/dev/null 2>&1; then
        echo "Error: CA certificate file not mounted at /certs/ca-certificates.crt"
        kubectl exec -n "$pod_namespace" "$pod_name" -- ls -la /certs/ || true
        return 1
    fi
    echo "CA certificate file is mounted correctly"
    
    # Extract CA bundle from pod
    echo "Extracting CA bundle from pod"
    kubectl exec -n "$pod_namespace" "$pod_name" -- cat /certs/ca-certificates.crt > /tmp/pod-ca-bundle.crt
    
    # Verify our CA is in the pod's bundle using multiple methods
    echo "Checking if our CA is in the pod's bundle"
    if grep -q "$CA_SUBJECT" /tmp/pod-ca-bundle.crt; then
        echo "CA found in pod's CA bundle (matched by subject)"
    else
        echo "Warning: CA subject not found in pod's CA bundle"
        echo "Trying to verify server certificate directly..."
    fi
    
    # Verify server certificate is trusted using the pod's CA bundle
    echo "Verifying that the server certificate is trusted via pod's CA bundle"
    if openssl verify -CAfile /tmp/pod-ca-bundle.crt "$server_cert" 2>/dev/null; then
        echo "Server certificate is trusted using pod's CA bundle"
    else
        echo "Error: Server certificate is not trusted by the pod's CA bundle"
        echo "This indicates the CA was not properly mounted or is not functioning correctly"
        
        # Additional diagnostics
        echo "Diagnostic information:"
        echo "- Server certificate subject: $(openssl x509 -in "$server_cert" -noout -subject)"
        echo "- Pod CA bundle size: $(wc -c < /tmp/pod-ca-bundle.crt) bytes"
        return 1
    fi
    
    # Cleanup
    rm -f /tmp/pod-ca-bundle.crt 2>/dev/null || true
    
    echo "Success: CA is properly mounted and trusted in the kotsadm pod"
    return 0
}

main "$@" 