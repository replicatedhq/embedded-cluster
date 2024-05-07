#!/usr/bin/env bash
set -euox pipefail

main() {
    apt-get update -y
    apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        socat

    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
    NODE_MAJOR=20
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list
    apt-get update && apt-get install nodejs -y

    cd /tmp/playwright
    npm ci
    npx playwright install --with-deps
}

main "$@"
