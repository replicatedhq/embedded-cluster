#!/usr/bin/env bash
set -euox pipefail

# This script installs a CA certificate into the system trust store and updates the CA certificates
# Usage: install-ca-cert.sh CA_CERTIFICATE_PATH

if [ $# -lt 1 ]; then
    echo "Usage: $0 CA_CERTIFICATE_PATH"
    exit 1
fi

CA_CERT="$1"

if [ ! -f "$CA_CERT" ]; then
    echo "Error: CA certificate file not found: $CA_CERT"
    exit 1
fi

# Detect the OS distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_ID="$ID"
else
    echo "Error: Unable to detect OS distribution"
    exit 1
fi

echo "Installing CA certificate on $OS_ID system..."

# Install the certificate based on the distribution
case "$OS_ID" in
    ubuntu|debian)
        # Copy the certificate to the appropriate location
        cp "$CA_CERT" /usr/local/share/ca-certificates/custom-ca.crt
        
        # Update the CA certificate store
        update-ca-certificates

        # Verify the certificate is in the CA bundle
        if ! grep -q "$(openssl x509 -in "$CA_CERT" -noout -hash)" /etc/ssl/certs/ca-certificates.crt; then
            echo "Error: CA certificate not found in system CA bundle after update"
            exit 1
        fi
        ;;
    centos|rhel|almalinux|rocky)
        # Copy the certificate to the appropriate location
        cp "$CA_CERT" /etc/pki/ca-trust/source/anchors/custom-ca.crt
        
        # Update the CA certificate store
        update-ca-trust extract

        # Verify the certificate is in the CA bundle
        if ! grep -q "$(openssl x509 -in "$CA_CERT" -noout -hash)" /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem; then
            echo "Error: CA certificate not found in system CA bundle after update"
            exit 1
        fi
        ;;
    *)
        echo "Error: Unsupported OS distribution: $OS_ID"
        exit 1
        ;;
esac

echo "CA certificate successfully installed and verified in system trust store"
exit 0 