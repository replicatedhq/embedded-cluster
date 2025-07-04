#!/bin/bash
set -x

squid_config="
http_port 3129 ssl-bump cert=/etc/squid/ssl_cert/ca.pem generate-host-certificates=on dynamic_cert_mem_cache_size=4MB
https_port 3130 cert=/etc/squid/ssl_cert/proxy.crt key=/etc/squid/ssl_cert/proxy.key
sslcrtd_program /usr/lib/squid/security_file_certgen -s /opt/ssl.db -M 4MB
acl step1 at_step SslBump1
ssl_bump peek step1
ssl_bump bump all

acl whitelist dstdomain \"/etc/squid/sites.whitelist.txt\"

# this will allow all access to the internet from local IPs
http_access allow localnet

# to restrict access so only local IPs can access the internet and only sites on the whitelist, instead use
# http_access allow localnet whitelist
"

whitelist_txt="
ec-e2e-proxy.testcluster.net
ec-e2e-replicated-app.testcluster.net

# dr
.amazonaws.com
"

COUNTRY=US
STATE=State
LOCALITY=City
ORGANIZATION=Replicated
ORGANIZATIONAL_UNIT=IT
COMMON_NAME=10.0.0.254
IP_SAN=10.0.0.254

create_config() {
        cat > /etc/squid/ssl_cert/san.cnf <<EOL
[req]
distinguished_name = req_distinguished_name
req_extensions = req_ext
prompt = no
[req_distinguished_name]
C = $COUNTRY
ST = $STATE
L = $LOCALITY
O = $ORGANIZATION
OU = $ORGANIZATIONAL_UNIT
CN = $COMMON_NAME
[req_ext]
subjectAltName = @alt_names
[v3_ca]
subjectAltName = @alt_names
basicConstraints = CA:true
[alt_names]
IP.1 = $IP_SAN
EOL
}

create_ca() {
        openssl req -new -newkey rsa:2048 -sha256 \
                -days 7 -nodes -x509 -extensions v3_ca \
                -keyout /etc/squid/ssl_cert/ca.pem \
                -out /etc/squid/ssl_cert/ca.pem \
                -config /etc/squid/ssl_cert/san.cnf \
                -subj "/C=US/ST=State/L=City/O=Replicated/OU=IT"
        openssl x509 -inform PEM -in /etc/squid/ssl_cert/ca.pem \
                -out /tmp/ca.crt
}

create_squid_ssl() {
        openssl genrsa -out /etc/squid/ssl_cert/proxy.key 2048
        openssl req \
                -new \
                -key /etc/squid/ssl_cert/proxy.key \
                -out /etc/squid/ssl_cert/proxy.csr \
                -config /etc/squid/ssl_cert/san.cnf \
                -extensions req_ext \
                -subj "/C=US/ST=State/L=City/O=Replicated/OU=IT/CN=10.128.0.4"
        openssl x509 \
                -req \
                -in /etc/squid/ssl_cert/proxy.csr \
                -CA /etc/squid/ssl_cert/ca.pem \
                -CAkey /etc/squid/ssl_cert/ca.pem \
                -CAcreateserial \
                -extfile /etc/squid/ssl_cert/san.cnf \
                -extensions req_ext \
                -out /etc/squid/ssl_cert/proxy.crt \
                -days 7 \
                -sha256
}


main() {
        apt-get update -y
        apt install -y squid-openssl
        /usr/lib/squid/security_file_certgen -c -s /opt/ssl.db -M 4MB
        mkdir -p /etc/squid/ssl_cert
        create_config
        create_ca
        create_squid_ssl
        echo "$squid_config" > /etc/squid/conf.d/ec.conf
        echo "$whitelist_txt" > /etc/squid/sites.whitelist.txt
        systemctl restart squid
}

main
