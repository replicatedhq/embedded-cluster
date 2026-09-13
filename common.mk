SHELL := /bin/bash
MAKEFLAGS += --no-print-directory

ARCH ?= $(shell go env GOARCH)
CURRENT_USER := $(if $(GITHUB_USER),$(GITHUB_USER),$(if $(GITHUB_RUN_ID),$(shell id -u -n)-$(GITHUB_RUN_ID)-$(GITHUB_JOB)-$(GITHUB_RUN_ATTEMPT),$(shell id -u -n)))

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
MELANGE ?= melange
APKO ?= apko
MELANGE_RUN ?= sudo $(MELANGE)

# melange mounts this host directory read-write at /var/cache/melange when
# using bubblewrap. Keep it in the repository so GitHub Actions can persist it.
PROJECT_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
MELANGE_CACHE_DIR ?= $(PROJECT_ROOT)/.cache/melange
MELANGE_APK_CACHE_DIR ?= $(PROJECT_ROOT)/.cache/melange-apk
APKO_CACHE_DIR ?= $(PROJECT_ROOT)/.cache/apko

## Version to use for building
VERSION ?= $(shell git describe --tags --match='[0-9]*.[0-9]*.[0-9]*' --abbrev=4)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_BUILD_TAGS ?= containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,exclude_graphdriver_overlay

ifdef GH_TOKEN
GH_AUTH_HEADER ?= -H 'Authorization: token $(GH_TOKEN)'
endif

image-tag = $(shell echo "$1" | sed 's/+/-/')

.PHONY: print-%
print-%:
	@echo -n $($*)

.PHONY: check-env-%
check-env-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi

melange:
	@command -v $(MELANGE) >/dev/null || { echo "melange is required" >&2; exit 1; }

apko:
	@command -v $(APKO) >/dev/null || { echo "apko is required" >&2; exit 1; }

$(MELANGE_CACHE_DIR) $(MELANGE_APK_CACHE_DIR) $(APKO_CACHE_DIR):
	mkdir -p $@

.PHONY: apko-build
apko-build: ARCHS ?= $(ARCH)
apko-build: $(APKO_CACHE_DIR) check-env-IMAGE apko apko-template
	cd build && ${APKO} \
		build apko.yaml ${IMAGE} apko.tar \
		--arch ${ARCHS} \
		--cache-dir $(APKO_CACHE_DIR)

.PHONY: apko-build-and-publish
apko-build-and-publish: ARCHS ?= $(ARCH)
apko-build-and-publish: $(APKO_CACHE_DIR) check-env-IMAGE apko apko-template
	@bash -c 'set -o pipefail && cd build && ${APKO} publish apko.yaml ${IMAGE} --arch ${ARCHS} --cache-dir $(APKO_CACHE_DIR) | tee digest'
	$(MAKE) apko-output-image

.PHONY: apko-login
apko-login:
	rm -f build/.docker/config.json
	@ { [ "${PASSWORD}" = "" ] || [ "${USERNAME}" = "" ] ; } || \
	${APKO} \
		login -u "${USERNAME}" \
		--password "${PASSWORD}" "${REGISTRY}"

.PHONY: apko-print-pkg-version
apko-print-pkg-version: ARCHS ?= $(ARCH)
apko-print-pkg-version: apko-template check-env-PACKAGE_NAME
		cd build && \
		${APKO_CMD} show-packages apko.yaml --arch=${ARCHS} | \
		grep ${PACKAGE_NAME} | \
		cut -s -d" " -f2 | \
		head -n1

.PHONY: apko-output-image
apko-output-image: check-env-IMAGE
	@digest=$$(cut -s -d'@' -f2 build/digest); \
	if [ -z "$$digest" ]; then \
		echo "error: no image digest found" >&2; \
		exit 1; \
	fi ; \
	echo "$(IMAGE)@$$digest" > build/image

.PHONY: melange-build
melange-build: ARCHS ?= $(ARCH)
melange-build: MELANGE_SOURCE_DIR ?= .
melange-build: $(MELANGE_CACHE_DIR) $(MELANGE_APK_CACHE_DIR) melange melange-template
	mkdir -p build
	${MELANGE_RUN} \
		keygen build/melange.rsa
	${MELANGE_RUN} \
		build build/melange.yaml \
		--arch ${ARCHS} \
		--runner bubblewrap \
		--signing-key build/melange.rsa \
		--cache-dir=$(MELANGE_CACHE_DIR) \
		--apk-cache-dir=$(MELANGE_APK_CACHE_DIR) \
		--source-dir $(MELANGE_SOURCE_DIR) \
		--out-dir build/packages/
	sudo chown -R $$(id -u):$$(id -g) build $(MELANGE_CACHE_DIR) $(MELANGE_APK_CACHE_DIR)

.PHONY: melange-template
melange-template: check-env-MELANGE_CONFIG check-env-PACKAGE_VERSION check-env-K0S_MINOR_VERSION
	mkdir -p build
	PACKAGE_VERSION='$(PACKAGE_VERSION)' K0S_MINOR_VERSION='$(K0S_MINOR_VERSION)' \
		envsubst '$${PACKAGE_VERSION} $${K0S_MINOR_VERSION}' < ${MELANGE_CONFIG} > build/melange.yaml

.PHONY: apko-template
apko-template: check-env-APKO_CONFIG check-env-PACKAGE_VERSION
	mkdir -p build
	PACKAGE_VERSION='$(PACKAGE_VERSION)' \
		envsubst '$${PACKAGE_VERSION}' < ${APKO_CONFIG} > build/apko.yaml
