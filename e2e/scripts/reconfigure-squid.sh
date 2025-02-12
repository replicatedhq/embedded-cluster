#!/bin/bash
set -euxo pipefail

function main() {
	sed -i 's/http_access allow localnet/http_access allow whitelist/' /etc/squid/conf.d/ec.conf

	systemctl restart squid
}

main "$@"
