SHELL := /bin/bash

-include ../chainguard.mk

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

export PACKAGE_VERSION ?= $(shell git describe --tags --dirty)
export IMAGE_NAME ?= replicated/embedded-cluster-local-artifact-mirror
export IMAGE_TAG ?= $(shell echo "$(PACKAGE_VERSION)" | sed 's/+/-/')
export IMAGE ?= $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: build
build:
	mkdir -p bin
	go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/local-artifact-mirror-$(GOOS)-$(GOARCH) ../cmd/local-artifact-mirror

export MELANGE_CONFIG := deploy/melange.tmpl.yaml

.PHONY: melange-build-local-artifact-mirror
melange-build-local-artifact-mirror:
	$(MAKE) melange-build MELANGE_SOURCE_DIR=../

export APKO_CONFIG := deploy/apko.tmpl.yaml

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image: melange-build-local-artifact-mirror apko-build

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image: melange-build-local-artifact-mirror apko-build-and-publish

.PHONY: build-ttl.sh
build-ttl.sh:
	$(MAKE) build-and-push-local-artifact-mirror-image \
		IMAGE=ttl.sh/$(IMAGE)

.PHONY: clean
clean:
	rm -rf bin build
