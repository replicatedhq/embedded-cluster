SHELL := /bin/bash

-include ../chainguard.mk

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

export PACKAGE_VERSION ?= $(shell git describe --tags --dirty)
export MELANGE_CONFIG := deploy/melange.tmpl.yaml
export MELANGE_SOURCE_DIR := ../

.PHONY: bin/local-artifact-mirror
bin/local-artifact-mirror:
	mkdir -p bin
	go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/local-artifact-mirror-$(GOOS)-$(GOARCH) ./cmd/local-artifact-mirror

.PHONY: melange-build-local-artifact-mirror
melange-build-local-artifact-mirror:
	$(MAKE) melange-build 

.PHONY: clean
clean:
	rm -rf bin build
