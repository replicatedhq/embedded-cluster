#!/usr/bin/env bash
set -euox pipefail

# This script generates and installs a CA certificate into the system trust store
# Usage: install-ca-cert.sh [OUTPUT_DIR]

# Set output directory
OUTPUT_DIR=${1:-/tmp/certs}
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"

echo "Generating and installing certificates in $OUTPUT_DIR"

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate - using a recognizable subject for easier verification
openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -out ca.crt \
  -subj "/C=US/ST=Washington/L=Seattle/O=EmbeddedClusterTest/OU=Testing/CN=Test CA for Embedded Cluster"

# Generate server private key
openssl genrsa -out server.key 2048

# Create server CSR
openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=Washington/L=Seattle/O=EmbeddedClusterTest/OU=Testing/CN=testserver.example.com"

# Create a config file for SAN extension
cat > san.cnf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = testserver.example.com

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = testserver.example.com
DNS.2 = testserver
EOF

# Sign the server certificate with our CA
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days 365 -sha256 -extfile san.cnf -extensions v3_req

# Verify the certificate chains correctly
echo "Verifying certificate chain..."
openssl verify -CAfile ca.crt server.crt

# Display certificate information
echo "Certificate information:"
echo "CA certificate hash: $(openssl x509 -in ca.crt -noout -hash)"
echo "CA certificate subject: $(openssl x509 -in ca.crt -noout -subject)"
echo "Server certificate subject: $(openssl x509 -in server.crt -noout -subject)"

# Output certificate paths
echo "Certificates generated successfully:"
echo "CA certificate:       $OUTPUT_DIR/ca.crt"
echo "Server certificate:   $OUTPUT_DIR/server.crt" 
echo "Server private key:   $OUTPUT_DIR/server.key"

# Cleanup unnecessary files
rm -f san.cnf server.csr ca.key.srl

# Now install the CA certificate
CA_CERT="$OUTPUT_DIR/ca.crt"

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
        # Get certificate hash before installation
        CERT_HASH=$(openssl x509 -in "$CA_CERT" -noout -hash)
        CERT_SUBJECT=$(openssl x509 -in "$CA_CERT" -noout -subject)
        
        # Copy the certificate to the appropriate location
        cp "$CA_CERT" /usr/local/share/ca-certificates/custom-ca.crt
        
        # Update the CA certificate store
        update-ca-certificates
        
        # Check for the individual hash-named certificate or the custom-ca.pem
        if [ -L "/etc/ssl/certs/${CERT_HASH}.0" ] || [ -f "/etc/ssl/certs/custom-ca.pem" ]; then
            echo "Certificate successfully installed (verified by symlink or direct file)"
        # As a backup verification, check if subject is in any of the certs
        elif grep -q "$CERT_SUBJECT" /etc/ssl/certs/* 2>/dev/null; then
            echo "Certificate found in system CA store (verified by subject)"
        else
            echo "Error: CA certificate not found in system CA bundle after update"
            echo "Expected certificate hash: $CERT_HASH"
            echo "Expected subject: $CERT_SUBJECT"
            exit 1
        fi
        ;;
    centos|rhel|almalinux|rocky)
        # Get certificate hash and subject for verification
        CERT_HASH=$(openssl x509 -in "$CA_CERT" -noout -hash)
        CERT_SUBJECT=$(openssl x509 -in "$CA_CERT" -noout -subject)
        
        # Copy the certificate to the appropriate location
        cp "$CA_CERT" /etc/pki/ca-trust/source/anchors/custom-ca.crt
        
        # Update the CA certificate store
        update-ca-trust extract

        # Try multiple verification methods
        if grep -q "$CERT_SUBJECT" /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem; then
            echo "Certificate found in system CA store (verified by subject)"
        else
            echo "Error: CA certificate not found in system CA bundle after update"
            echo "Expected certificate hash: $CERT_HASH"
            echo "Expected subject: $CERT_SUBJECT"
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