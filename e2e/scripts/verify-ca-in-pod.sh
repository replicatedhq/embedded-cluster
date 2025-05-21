#!/usr/bin/env bash
set -euox pipefail

DIR=/usr/local/bin
. $DIR/common.sh || echo "common.sh not found, continuing..."

# Hardcode paths to match what install-ca-cert.sh generates
CA_CERT="/tmp/certs/ca.crt"
SERVER_CERT="/tmp/certs/server.crt"
POD_NAMESPACE="kotsadm"
POD_LABEL="app=kotsadm"

# Validate files exist
if [ ! -f "$CA_CERT" ] || [ ! -f "$SERVER_CERT" ]; then
    echo "Error: Certificate files not found"
    echo "CA certificate: $CA_CERT"
    echo "Server certificate: $SERVER_CERT"
    exit 1
fi

echo "Checking if CA certificate is properly mounted in kotsadm pod"

# Extract CA info for searching
CA_HASH=$(openssl x509 -in "$CA_CERT" -noout -hash)
CA_SUBJECT=$(openssl x509 -in "$CA_CERT" -noout -subject)
echo "CA Hash: $CA_HASH"
echo "CA Subject: $CA_SUBJECT"

# Find the kotsadm pod
pod_name=$(kubectl get pods -n "$POD_NAMESPACE" -l "$POD_LABEL" -o jsonpath='{.items[0].metadata.name}')
if [ -z "$pod_name" ]; then
    echo "Error: kotsadm pod not found"
    exit 1
fi
echo "Found kotsadm pod: $pod_name"

# First verify the SSL_CERT_DIR environment variable is set correctly
echo "Checking for SSL_CERT_DIR environment variable in pod"
if ! kubectl exec -n "$POD_NAMESPACE" "$pod_name" -- env | grep -q "SSL_CERT_DIR=/certs"; then
    echo "Error: SSL_CERT_DIR environment variable not set correctly in the pod"
    kubectl exec -n "$POD_NAMESPACE" "$pod_name" -- env | grep SSL
    exit 1
fi
echo "SSL_CERT_DIR environment variable is set correctly"

# Check for the mounted CA certificate file
echo "Checking for mounted CA certificate file"
if ! kubectl exec -n "$POD_NAMESPACE" "$pod_name" -- ls -la /certs/ca-certificates.crt >/dev/null 2>&1; then
    echo "Error: CA certificate file not mounted at /certs/ca-certificates.crt"
    kubectl exec -n "$POD_NAMESPACE" "$pod_name" -- ls -la /certs/ || true
    exit 1
fi
echo "CA certificate file is mounted correctly"

# Extract CA bundle from pod
echo "Extracting CA bundle from pod"
kubectl exec -n "$POD_NAMESPACE" "$pod_name" -- cat /certs/ca-certificates.crt > /tmp/pod-ca-bundle.crt

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
if openssl verify -CAfile /tmp/pod-ca-bundle.crt "$SERVER_CERT" 2>/dev/null; then
    echo "Server certificate is trusted using pod's CA bundle"
else
    echo "Error: Server certificate is not trusted by the pod's CA bundle"
    echo "This indicates the CA was not properly mounted or is not functioning correctly"
    
    # Additional diagnostics
    echo "Diagnostic information:"
    echo "- Server certificate subject: $(openssl x509 -in "$SERVER_CERT" -noout -subject)"
    echo "- Pod CA bundle size: $(wc -c < /tmp/pod-ca-bundle.crt) bytes"
    exit 1
fi

# Cleanup
rm -f /tmp/pod-ca-bundle.crt 2>/dev/null || true

echo "Success: CA is properly mounted and trusted in the kotsadm pod"
exit 0 