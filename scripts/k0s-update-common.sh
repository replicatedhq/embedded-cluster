#!/bin/bash

# Common functions for k0s update scripts
# Source this file: source "$(dirname "$0")/k0s-update-common.sh"

# Detect OS and use appropriate sed syntax
if [[ "$OSTYPE" == "darwin"* ]]; then
    SED_ARGS=(-i '')
else
    SED_ARGS=(-i)
fi

function get_k0s_version() {
    local minor_version=$1
    grep "^K0S_VERSION_1_${minor_version} = " versions.mk | sed 's/.*= //'
}

function sync_k8s_replace_directives() {
    local k0s_version=$1
    if [[ -z "$k0s_version" ]]; then
        echo "Warning: no k0s version found, skipping k8s replace directive sync"
        return
    fi

    local k8s_version
    local gomod_info
    local gomod_path

    # Get k8s version from k0s go.mod
    gomod_info=$(go mod download -json "github.com/k0sproject/k0s@${k0s_version}" 2>/dev/null || true)
    if [[ -z "$gomod_info" ]]; then
        echo "Warning: failed to download k0s go.mod for ${k0s_version}, skipping k8s replace directive sync"
        return
    fi

    gomod_path=$(echo "$gomod_info" | jq -r '.GoMod')
    if [[ -z "$gomod_path" || "$gomod_path" == "null" ]]; then
        echo "Warning: could not determine k0s go.mod path for ${k0s_version}, skipping k8s replace directive sync"
        return
    fi

    k8s_version=$(grep 'k8s.io/api => k8s.io/api v' "$gomod_path" | awk '{print $NF}' || true)
    if [[ -z "$k8s_version" ]]; then
        echo "Warning: could not find k8s version in k0s go.mod, skipping k8s replace directive sync"
        return
    fi

    # Update replace directives in both go.mod files
    for modfile in go.mod kinds/go.mod; do
        if [[ -f "$modfile" ]]; then
            local current_version
            current_version=$(grep 'k8s.io/api => k8s.io/api v' "$modfile" | awk '{print $NF}')
            if [[ -n "$current_version" && "$current_version" != "$k8s_version" ]]; then
                echo "Syncing k8s replace directives in ${modfile} from ${current_version} to ${k8s_version}"
                sed "${SED_ARGS[@]}" "/k8s\\.io\\//s|${current_version}|${k8s_version}|g" "$modfile"
            fi
        fi
    done
}
