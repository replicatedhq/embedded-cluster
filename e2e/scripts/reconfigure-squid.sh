#!/bin/bash
set -euxo pipefail

function main() {
	sed -i 's/http_access allow localnet/http_access allow whitelist/' /etc/squid/conf.d/ec.conf

	systemctl restart squid

    # validate that the whitelist is working
    # validate that we can access ec-e2e-replicated-app.testcluster.net
    status_code=$(curl -s -o /dev/null -w "%{http_code}" -x http://10.0.0.254:3128 http://ec-e2e-replicated-app.testcluster.net/market/v1/echo/ip)
    if [ "$status_code" -ne 200 ]; then
        echo "Error: Expected status code 200, got $status_code"
        exit 1
    fi

    # validate that we cannot access google.com (should be blocked)
    status_code=$(curl -s -o /dev/null -w "%{http_code}" -x http://10.0.0.254:3128 http://google.com)
    if [ "$status_code" -ne 403 ] && [ "$status_code" -ne 407 ]; then
        echo "Error: Expected status code 403 or 407 (blocked), got $status_code"
        exit 1
    fi
}

main "$@"
