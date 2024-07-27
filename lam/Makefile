SHELL := /bin/bash

-include ../chainguard.mk

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

export PACKAGE_VERSION ?= $(shell git describe --tags --dirty)
export IMAGE_NAME ?= replicated/embedded-cluster-local-artifact-mirror
export IMAGE_TAG ?= $(shell echo "$PACKAGE_VERSION" | sed 's/+/-/')
export IMAGE ?= $(IMAGE_NAME):$(IMAGE_TAG)

export MELANGE_CONFIG := deploy/melange.tmpl.yaml
export MELANGE_SOURCE_DIR := ../
export APKO_CONFIG := deploy/apko.tmpl.yaml

.PHONY: bin/local-artifact-mirror
bin/local-artifact-mirror:
	mkdir -p bin
	go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/local-artifact-mirror-$(GOOS)-$(GOARCH) ./cmd/local-artifact-mirror

.PHONY: build-local-artifact-mirror-package
build-local-artifact-mirror-package: export MELANGE_CONFIG = deploy/melange.tmpl.yaml
build-local-artifact-mirror-package:
	$(MAKE) melange-build 

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image: build-local-artifact-mirror-package apko-build

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image: build-local-artifact-mirror-package apko-build-and-publish

.PHONY: clean
clean:
	rm -rf bin build
