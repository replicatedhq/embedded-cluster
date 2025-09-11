# External Dependency Versions
# This file contains version definitions for external dependencies used in the build process
# The versions are kept up to date by the .github/workflows/dependencies.yaml github actions workflow

# K0S Kubernetes Distribution Versions
K0S_MINOR_VERSION ?= 33

# K0S Versions
K0S_VERSION_1_33 = v1.33.4+k0s.0
K0S_VERSION_1_32 = v1.32.8+k0s.0
K0S_VERSION_1_31 = v1.31.12+k0s.0
K0S_VERSION_1_30 = v1.30.14+k0s.0
K0S_VERSION_1_29 = v1.29.15+k0s.0

# Dynamic version selection
K0S_VERSION = $(K0S_VERSION_1_$(K0S_MINOR_VERSION))

# K0S Go Version Overrides (optional)
# These allow overriding the Go version used for specific K0S minor versions
# Format: K0S_GO_VERSION_OVERRIDE_<MINOR_VERSION>
# Example: K0S_GO_VERSION_OVERRIDE_32 = v1.32.7+k0s.0.ec.1

# K0S Binary Source Overrides (optional)
# These allow overriding the binary source used for specific K0S minor versions
# Format: K0S_BINARY_SOURCE_OVERRIDE_<MINOR_VERSION>
# Example: K0S_BINARY_SOURCE_OVERRIDE_32 = https://github.com/k0sproject/k0s/releases/download/v1.32.7+k0s.0/k0s-v1.32.7+k0s.0-amd64

# Troubleshoot Version
TROUBLESHOOT_VERSION = v0.122.0

# FIO Version (for performance testing)
FIO_VERSION = 3.41

# Kubernetes Development Tool Versions
CONTROLLER_TOOLS_VERSION = v0.19.0
KUSTOMIZE_VERSION = v5.7.1

# KOTS Version Overrides
# If KOTS_BINARY_URL_OVERRIDE is set to a ttl.sh artifact, KOTS_VERSION will be dynamically generated
KOTS_BINARY_URL_OVERRIDE =
# If KOTS_BINARY_FILE_OVERRIDE is set, KOTS_VERSION will be dynamically generated
# For dev env, build the kots binary in the kots repo with "make kots-linux-arm64" and set this to "../kots/bin/kots"
KOTS_BINARY_FILE_OVERRIDE =
