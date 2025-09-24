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
K0S_GO_VERSION = $(K0S_VERSION_1_$(K0S_MINOR_VERSION))

# Troubleshoot Version
TROUBLESHOOT_VERSION = v0.122.0

# FIO Version (for performance testing)
FIO_VERSION = 3.41

# Kubernetes Development Tool Versions
CONTROLLER_TOOLS_VERSION = v0.19.0
KUSTOMIZE_VERSION = v5.7.1

### Overrides ###

# KOTS Version Overrides
# If KOTS_BINARY_URL_OVERRIDE is set to a ttl.sh artifact, KOTS_VERSION will be dynamically generated
KOTS_BINARY_URL_OVERRIDE =
# If KOTS_BINARY_FILE_OVERRIDE is set, KOTS_VERSION will be dynamically generated
# For dev env, build the kots binary in the kots repo with "make kots-linux-arm64" and set this to "../kots/bin/kots"
KOTS_BINARY_FILE_OVERRIDE =

# k0s version overrides go here

ifeq ($(K0S_VERSION),v1.31.12+k0s.0)
K0S_VERSION = v1.31.12+k0s.0-ec.0
endif

# K0S go version overrides go here

# K0S binary source overrides go here
K0S_BINARY_SOURCE_OVERRIDE =
ifeq ($(K0S_VERSION),v1.31.12+k0s.0-ec.0)
K0S_BINARY_SOURCE_OVERRIDE = https://tf-staging-embedded-cluster-bin.s3.amazonaws.com/custom-k0s-binaries/k0s-v1.31.12%2Bk0s.0-ec.0-$(ARCH)
endif

# Require a new build be released if the patched k0s version changes
.PHONY: check-k0s-version
check-k0s-version:
	@if echo "$(K0S_VERSION)" | grep -q "^v1\.31"; then \
		if [ "$(K0S_VERSION)" != "v1.31.12+k0s.0-ec.0" ]; then \
			echo "Error: K0S_VERSION starts with v1.31 but does not equal v1.31.12+k0s.0-ec.0"; \
			echo "Current K0S_VERSION: $(K0S_VERSION)"; \
			echo "Expected: v1.31.12+k0s.0-ec.0"; \
			exit 1; \
		fi; \
	fi
