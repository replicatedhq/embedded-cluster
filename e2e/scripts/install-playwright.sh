#!/usr/bin/env bash
set -euox pipefail

install_apt() {
    curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" -o nodesource_setup.sh
    bash nodesource_setup.sh
    apt-get install -y nodejs
}

install_yum() {
    curl -fsSL "https://rpm.nodesource.com/setup_${NODE_MAJOR}.x" -o nodesource_setup.sh
    bash nodesource_setup.sh
    yum install -y nodejs
}

main() {
    if command -v apt-get &> /dev/null; then
        install_apt
    elif command -v yum &> /dev/null; then
        install_yum
    else
        echo "Unsupported package manager"
        exit 1
    fi

    cd /automation/playwright
    npm ci
    npx playwright install --with-deps
}

export NODE_MAJOR=20
main "$@"
