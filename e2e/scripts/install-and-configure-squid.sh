#!/bin/bash
set -x

squid_config="
http_port 3129 ssl-bump cert=/etc/squid/ssl_cert/ca.pem generate-host-certificates=on dynamic_cert_mem_cache_size=4MB
https_port 3130 cert=/etc/squid/ssl_cert/proxy.crt key=/etc/squid/ssl_cert/proxy.key
sslcrtd_program /usr/lib/squid/security_file_certgen -s /opt/ssl.db -M 4MB
acl step1 at_step SslBump1
ssl_bump peek step1
ssl_bump bump all
http_access allow localnet
"

create_ca() {
        openssl req -new -newkey rsa:2048 -sha256 \
                -days 7 -nodes -x509 -extensions v3_ca \
                -keyout /etc/squid/ssl_cert/ca.pem \
                -out /etc/squid/ssl_cert/ca.pem \
                -subj "/C=US/ST=State/L=City/O=Replicated/OU=IT"
        openssl x509 -inform PEM -in /etc/squid/ssl_cert/ca.pem \
                -out /tmp/ca.crt
}

create_squid_ssl() {
        openssl genrsa -out /etc/squid/ssl_cert/proxy.key 2048
        openssl req -new -key /etc/squid/ssl_cert/proxy.key \
                -out /etc/squid/ssl_cert/proxy.csr \
                -subj "/C=US/ST=State/L=City/O=Replicated/OU=IT/CN=10.0.0.254"
        openssl x509 -req -in /etc/squid/ssl_cert/proxy.csr \
                -CA /etc/squid/ssl_cert/ca.pem \
                -CAkey /etc/squid/ssl_cert/ca.pem -CAcreateserial \
                -out /etc/squid/ssl_cert/proxy.crt -days 7 -sha256
}


main() {
        apt install -y squid-openssl
        /usr/lib/squid/security_file_certgen -c -s /opt/ssl.db -M 4MB
        mkdir -p /etc/squid/ssl_cert
        create_ca
        create_squid_ssl
        echo "$squid_config" > /etc/squid/conf.d/ec.conf
        systemctl restart squid
}

main
