SHELL := /bin/bash

include common.mk

VERSION ?= $(shell git describe --tags --dirty --match='[0-9]*.[0-9]*.[0-9]*')
CURRENT_USER := $(if $(GITHUB_USER),$(GITHUB_USER),$(shell id -u -n))
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE =
K0S_VERSION = v1.29.7+k0s.0
K0S_GO_VERSION = v1.29.7+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.10+k0s.0
K0S_BINARY_SOURCE_OVERRIDE =
PREVIOUS_K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.100.0
# revert this
KOTS_VERSION = v1.115.1
KOTS_BINARY_URL_OVERRIDE =
# TODO: move this to a manifest file
LOCAL_ARTIFACT_MIRROR_IMAGE ?= proxy.replicated.com/anonymous/replicated/embedded-cluster-local-artifact-mirror:$(VERSION)
LD_FLAGS = \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleMigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleKurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE)
DISABLE_FIO_BUILD ?= 0

export PATH := $(shell pwd)/bin:$(PATH)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.DEFAULT_GOAL := default
default: build-ttl.sh

pkg/goods/bins/k0s: Makefile
	mkdir -p pkg/goods/bins
	if [ "$(K0S_BINARY_SOURCE_OVERRIDE)" != "" ]; then \
	    curl --retry 5 --retry-all-errors -fL -o pkg/goods/bins/k0s "$(K0S_BINARY_SOURCE_OVERRIDE)" ; \
	else \
	    curl --retry 5 --retry-all-errors -fL -o pkg/goods/bins/k0s "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-$(GOARCH)" ; \
	fi
	chmod +x pkg/goods/bins/k0s
	touch pkg/goods/bins/k0s

pkg/goods/bins/kubectl-support_bundle: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/support-bundle
	curl --retry 5 --retry-all-errors -fL -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_linux_$(GOARCH).tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/kubectl-support_bundle
	touch pkg/goods/bins/kubectl-support_bundle

pkg/goods/bins/kubectl-preflight: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/preflight
	curl --retry 5 --retry-all-errors -fL -o output/tmp/preflight/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/preflight_linux_$(GOARCH).tar.gz
	tar -xzf output/tmp/preflight/preflight.tar.gz -C output/tmp/preflight
	mv output/tmp/preflight/preflight pkg/goods/bins/kubectl-preflight
	touch pkg/goods/bins/kubectl-preflight

pkg/goods/bins/local-artifact-mirror: Makefile
	mkdir -p pkg/goods/bins
	$(MAKE) -C local-artifact-mirror build GOOS=linux GOARCH=$(GOARCH)
	cp local-artifact-mirror/bin/local-artifact-mirror-linux-$(GOARCH) pkg/goods/bins/local-artifact-mirror

pkg/goods/bins/fio: PLATFORM = linux/$(GOARCH)
pkg/goods/bins/fio: Makefile
ifneq ($(DISABLE_FIO_BUILD),1)
	mkdir -p pkg/goods/bins
	docker build -t fio --build-arg PLATFORM=$(PLATFORM) fio
	docker rm -f fio && docker run --name fio fio
	docker cp fio:/output/fio pkg/goods/bins/fio
	touch pkg/goods/bins/fio
endif

pkg/goods/internal/bins/kubectl-kots: Makefile
	mkdir -p pkg/goods/internal/bins
	mkdir -p output/tmp/kots
	if [ "$(KOTS_BINARY_URL_OVERRIDE)" != "" ]; then \
	    curl --retry 5 --retry-all-errors -fL -o output/tmp/kots/kots.tar.gz "$(KOTS_BINARY_URL_OVERRIDE)" ; \
	else \
	    curl --retry 5 --retry-all-errors -fL -o output/tmp/kots/kots.tar.gz https://github.com/replicatedhq/kots/releases/download/$(KOTS_VERSION)/kots_linux_$(GOARCH).tar.gz ; \
	fi
	tar -xzf output/tmp/kots/kots.tar.gz -C output/tmp/kots
	mv output/tmp/kots/kots pkg/goods/internal/bins/kubectl-kots
	touch pkg/goods/internal/bins/kubectl-kots

output/tmp/release.tar.gz: e2e/kots-release-install/*
	mkdir -p output/tmp/kots-release-install
	cp -r e2e/kots-release-install/* output/tmp/kots-release-install/
	@sed -i '' "s|__version_string__|${VERSION}|g" output/tmp/kots-release-install/cluster-config.yaml
	tar -czf output/tmp/release.tar.gz -C output/tmp/kots-release-install .
	rm -rf output/tmp/kots-release-install

output/bin/embedded-cluster-release-builder:
	mkdir -p output/bin
	CGO_ENABLED=0 go build -o output/bin/embedded-cluster-release-builder e2e/embedded-cluster-release-builder/main.go

.PHONY: embedded-release
embedded-release: GOARCH = amd64
embedded-release: embedded-cluster-linux-$(GOARCH) output/tmp/release.tar.gz output/bin/embedded-cluster-release-builder
	./output/bin/embedded-cluster-release-builder output/bin/embedded-cluster output/tmp/release.tar.gz output/bin/embedded-cluster

.PHONY: embedded-release-arm64
embedded-release-arm64: GOARCH = arm64
embedded-release-arm64: embedded-release

.PHONY: go.mod
go.mod: Makefile
	go get github.com/k0sproject/k0s@$(K0S_GO_VERSION)
	go mod tidy

.PHONY: static
static: pkg/goods/bins/k0s \
	pkg/goods/bins/kubectl-preflight \
	pkg/goods/bins/kubectl-support_bundle \
	pkg/goods/bins/local-artifact-mirror \
	pkg/goods/bins/fio \
	pkg/goods/internal/bins/kubectl-kots

.PHONY: embedded-cluster-linux-amd64
embedded-cluster-linux-amd64: GOOS = linux
embedded-cluster-linux-amd64: GOARCH = amd64
embedded-cluster-linux-amd64: static go.mod embedded-cluster
	mkdir -p ./output/bin
	cp ./build/embedded-cluster-$(GOOS)-$(GOARCH) ./output/bin/$(APP_NAME)

.PHONY: embedded-cluster-linux-arm64
embedded-cluster-linux-arm64: GOOS = linux
embedded-cluster-linux-arm64: GOARCH = arm64
embedded-cluster-linux-arm64: static go.mod embedded-cluster
	mkdir -p ./output/bin
	cp ./build/embedded-cluster-$(GOOS)-$(GOARCH) ./output/bin/$(APP_NAME)

# for testing
.PHONY: embedded-cluster-darwin-arm64
embedded-cluster-darwin-arm64: GOOS = darwin
embedded-cluster-darwin-arm64: GOARCH = arm64
embedded-cluster-darwin-arm64: go.mod embedded-cluster
	mkdir -p ./output/bin
	cp ./build/embedded-cluster-$(GOOS)-$(GOARCH) ./output/bin/$(APP_NAME)

.PHONY: embedded-cluster
embedded-cluster:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -ldflags "$(LD_FLAGS)" -o ./build/embedded-cluster-$(GOOS)-$(GOARCH) \
		./cmd/embedded-cluster

.PHONY: unit-tests
unit-tests:
	mkdir -p pkg/goods/bins pkg/goods/internal/bins
	touch pkg/goods/bins/BUILD pkg/goods/internal/bins/BUILD # compilation will fail if no files are present
	go test -v ./pkg/... ./cmd/...
	$(MAKE) -C operator test

.PHONY: vet
vet: static
	go vet ./...

.PHONY: e2e-tests
e2e-tests: embedded-release
	go test -timeout 45m -parallel 1 -failfast -v ./e2e

.PHONY: e2e-test
e2e-test:
	go test -timeout 45m -v ./e2e -run $(TEST_NAME)$

.PHONY: build-ttl.sh
build-ttl.sh:
	$(MAKE) -C local-artifact-mirror build-ttl.sh \
		IMAGE_NAME=$(CURRENT_USER)/embedded-cluster-local-artifact-mirror
	make embedded-cluster-linux-$(GOARCH) \
		LOCAL_ARTIFACT_MIRROR_IMAGE=proxy.replicated.com/anonymous/$(shell cat local-artifact-mirror/build/image)

.PHONY: clean
clean:
	rm -rf output
	rm -rf pkg/goods/bins
	rm -rf pkg/goods/internal/bins
	rm -rf build
	rm -rf bin

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml ./...

.PHONY: lint-and-fix
lint-and-fix:
	golangci-lint run --fix -c .golangci.yml ./...

.PHONY: scan
scan:
	trivy fs \
		--scanners vuln \
		--exit-code=1 \
		--severity="HIGH,CRITICAL" \
		--ignore-unfixed \
		./

.PHONY: buildtools
buildtools:
	mkdir -p pkg/goods/bins pkg/goods/internal/bins
	touch pkg/goods/bins/BUILD pkg/goods/internal/bins/BUILD # compilation will fail if no files are present
	go build -o ./output/bin/buildtools ./cmd/buildtools

.PHONY: cache-files
cache-files:
	./scripts/cache-files.sh

print-%:
	@echo -n $($*)

bootloose-debian:
	docker build -t bootloose-debian -f dev/dockerfiles/bootloose-debian.Dockerfile dev/dockerfiles

node%:
	@docker run -d \
		--name node$* \
		--hostname node$* \
		--privileged \
		--cgroupns=host \
		-v $(shell pwd):/replicatedhq/embedded-cluster \
		-v $(shell dirname $(shell pwd))/kots:/replicatedhq/kots \
		$(if $(filter node0,node$*),-p 30000:30000) \
		bootloose-debian \
		/sbin/init

	@$(MAKE) ssh-node$*

ssh-node%:
	@docker exec -it -w /replicatedhq/embedded-cluster node$* bash
