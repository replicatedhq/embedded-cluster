#!/usr/bin/env bash

SCHEME="{SCHEME}"
HOST="{HOST}"

# get_os returns the operating system we are running on.
function get_os() {
    uname | awk '{print tolower($0)}'
}

# get_arch returns the architecture we are running on. if arch is x86_64, we
# return amd64 instead.
function get_arch() {
    local arch
    arch=$(uname -m | awk '{print tolower($0)}')
    if [ "$arch" = "x86_64" ]; then
        arch="amd64"
    fi
    echo "$arch"
}

# download_and_install issues a request to the remote host to retrieve a compiled
# version of helmvm. it sends the provided yaml file path as body to the request.
# installs helmvm on /usr/local/bin directory.
function download_and_install() {
    local os=$1
    local arch=$2
    local download_url
    echo "Downloading HelmVM..."
    download_url="$SCHEME://$HOST/build?os=$os&arch=$arch"
    if ! curl -o helmvm -f --progress-bar "$download_url" ; then
        echo "Failed to download helmvm"
        exit 1
    fi
    chmod 755 ./helmvm
    echo "Moving helmvm to /usr/local/bin/helmvm..."
    sudo mv ./helmvm /usr/local/bin/helmvm
    echo "HelmVM installed."
}

function main() {
    local os
    local arch
    os=$(get_os)
    arch=$(get_arch)
    echo "Detected architecture $os/$arch."
    download_and_install "$os" "$arch"
    if [ $# -eq 0 ]; then
        exit 0
    fi
    echo "Running helmvm with arguments: $*"
    /usr/local/bin/helmvm "$@" < /dev/tty
}

main "$@"
