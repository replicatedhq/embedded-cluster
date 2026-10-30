# External Dependency Versions
# This file contains version definitions for external dependencies used in the build process
# The versions are kept up to date by the .github/workflows/dependencies.yaml github actions workflow

# K0S Kubernetes Distribution Versions
K0S_MINOR_VERSION ?= 36
# Release the current minor and the previous two. CI also needs the previous
# minor outside this range to exercise upgrades into the supported range.
SUPPORTED_K0S_MINOR_VERSIONS = $(shell seq $(K0S_MINOR_VERSION) -1 $$(($(K0S_MINOR_VERSION) - 2)))

# K0S Versions
K0S_VERSION_1_36 = v1.36.5+k0s.0
K0S_VERSION_1_35 = v1.35.9+k0s.0
K0S_VERSION_1_34 = v1.34.12+k0s.0
K0S_VERSION_1_33 = v1.33.13+k0s.1

# Dynamic version selection
K0S_VERSION = $(K0S_VERSION_1_$(K0S_MINOR_VERSION))
K0S_GO_VERSION = $(K0S_VERSION_1_$(K0S_MINOR_VERSION))

# Troubleshoot Version
TROUBLESHOOT_VERSION = v0.134.3

# Helm Version
HELM_VERSION = v4.3.1

# FIO Version (for performance testing)
FIO_VERSION = 3.43

# Kubernetes Development Tool Versions
CONTROLLER_TOOLS_VERSION = v0.23.0
KUSTOMIZE_VERSION = v5.8.3

### Overrides ###

# KOTS Version Overrides
# If KOTS_BINARY_URL_OVERRIDE is set to a ttl.sh artifact, KOTS_VERSION will be dynamically generated
KOTS_BINARY_URL_OVERRIDE =
# If KOTS_BINARY_FILE_OVERRIDE is set, KOTS_VERSION will be dynamically generated
# For dev env, build the kots binary in the kots repo with "make kots-linux-arm64" and set this to "../kots/bin/kots"
KOTS_BINARY_FILE_OVERRIDE =

# Optional source override for locally patched k0s builds.
K0S_BINARY_SOURCE_OVERRIDE =

# The k0s airgap.GetImageURIs signature changed in k0s 1.36 (it takes a TargetEnv).
# Code compiled against a k0s module < 1.36 uses the old signature and must select
# the legacy ListK0sImages via the k0s_legacy_airgap build tag (see pkg/config/images.go).
# Defined here, not just the root Makefile, so the operator and local-artifact-mirror
# sub-builds (which include this file) pick it up. K0S_MINOR_VERSION is the source of
# truth: the build re-pins go.mod to K0S_GO_VERSION for the selected minor.
# Assumes common.mk (which defines GO_BUILD_TAGS) is included before this file.
# TODO(k0s-1.37-oldest): drop this gate when the oldest supported minor is >= 1.37.
GO_INSTALLER_BUILD_TAGS := osusergo,netgo
ifeq ($(shell [ "$(K0S_MINOR_VERSION)" -lt 36 ] && echo legacy),legacy)
GO_BUILD_TAGS := $(GO_BUILD_TAGS),k0s_legacy_airgap
GO_INSTALLER_BUILD_TAGS := $(GO_INSTALLER_BUILD_TAGS),k0s_legacy_airgap
endif
