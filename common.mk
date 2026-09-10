SHELL := /bin/bash
MAKEFLAGS += --no-print-directory

ARCH ?= $(shell go env GOARCH)
CURRENT_USER := $(if $(GITHUB_USER),$(GITHUB_USER),$(if $(GITHUB_RUN_ID),$(shell id -u -n)-$(GITHUB_RUN_ID)-$(GITHUB_JOB)-$(GITHUB_RUN_ATTEMPT),$(shell id -u -n)))

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

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

.PHONY: image-output-digest
image-output-digest: check-env-IMAGE
	@digest=$$(cut -s -d'@' -f2 build/digest); \
	if [ -z "$$digest" ]; then \
		echo "error: no image digest found" >&2; \
		exit 1; \
	fi ; \
	echo "$(IMAGE)@$$digest" > build/image
