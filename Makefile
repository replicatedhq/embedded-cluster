SHELL := /bin/bash

include common.mk

VERSION ?= $(shell git describe --tags --dirty --match='[0-9]*.[0-9]*.[0-9]*')
CURRENT_USER := $(if $(GITHUB_USER),$(GITHUB_USER),$(shell id -u -n))
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
KOTS_VERSION = v$(shell awk '/^version/{print $$2}' pkg/addons/adminconsole/static/metadata.yaml | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
KOTS_BINARY_URL_OVERRIDE =
# TODO: move this to a manifest file
LOCAL_ARTIFACT_MIRROR_IMAGE ?= proxy.replicated.com/anonymous/replicated/embedded-cluster-local-artifact-mirror:$(VERSION)
# These are used to override the binary urls in dev and e2e tests
METADATA_K0S_BINARY_URL_OVERRIDE =
METADATA_KOTS_BINARY_URL_OVERRIDE =
METADATA_OPERATOR_BINARY_URL_OVERRIDE =
LD_FLAGS = \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/cmd/embedded-cluster/metadata.K0sBinaryURLOverride=$(METADATA_K0S_BINARY_URL_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/cmd/embedded-cluster/metadata.KOTSBinaryURLOverride=$(METADATA_KOTS_BINARY_URL_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/cmd/embedded-cluster/metadata.OperatorBinaryURLOverride=$(METADATA_OPERATOR_BINARY_URL_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleMigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleKurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE)
DISABLE_FIO_BUILD ?= 0

export PATH := $(shell pwd)/bin:$(PATH)

OS ?= linux
ARCH ?= $(shell go env GOARCH)

.DEFAULT_GOAL := default
default: build-ttl.sh

split-hyphen = $(word $2,$(subst -, ,$1))

.PHONY: pkg/goods/bins/k0s
pkg/goods/bins/k0s:
	$(MAKE) output/bins/k0s-$(K0S_VERSION)-$(ARCH)
	mkdir -p pkg/goods/bins
	cp output/bins/k0s-$(K0S_VERSION)-$(ARCH) $@

output/bins/k0s-%:
	mkdir -p output/bins
	if [ "$(K0S_BINARY_SOURCE_OVERRIDE)" != "" ]; then \
	    curl --retry 5 --retry-all-errors -fL -o $@ "$(K0S_BINARY_SOURCE_OVERRIDE)" ; \
	else \
	    curl --retry 5 --retry-all-errors -fL -o $@ "https://github.com/k0sproject/k0s/releases/download/$(call split-hyphen,$*,1)/k0s-$(call split-hyphen,$*,1)-$(call split-hyphen,$*,2)" ; \
	fi
	chmod +x $@
	touch $@

.PHONY: pkg/goods/bins/kubectl-support_bundle
pkg/goods/bins/kubectl-support_bundle:
	$(MAKE) output/bins/kubectl-support_bundle-$(TROUBLESHOOT_VERSION)-$(ARCH)
	mkdir -p pkg/goods/bins
	cp output/bins/kubectl-support_bundle-$(TROUBLESHOOT_VERSION)-$(ARCH) $@

output/bins/kubectl-support_bundle-%:
	mkdir -p output/bins
	mkdir -p output/tmp
	curl --retry 5 --retry-all-errors -fL -o output/tmp/support-bundle.tar.gz "https://github.com/replicatedhq/troubleshoot/releases/download/$(call split-hyphen,$*,1)/support-bundle_$(OS)_$(call split-hyphen,$*,2).tar.gz"
	tar -xzf output/tmp/support-bundle.tar.gz -C output/tmp
	mv output/tmp/support-bundle $@
	rm -rf output/tmp
	touch $@

.PHONY: pkg/goods/bins/kubectl-preflight
pkg/goods/bins/kubectl-preflight:
	$(MAKE) output/bins/kubectl-preflight-$(TROUBLESHOOT_VERSION)-$(ARCH)
	mkdir -p pkg/goods/bins
	cp output/bins/kubectl-preflight-$(TROUBLESHOOT_VERSION)-$(ARCH) $@

output/bins/kubectl-preflight-%:
	mkdir -p output/bins
	mkdir -p output/tmp
	curl --retry 5 --retry-all-errors -fL -o output/tmp/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(call split-hyphen,$*,1)/preflight_$(OS)_$(call split-hyphen,$*,2).tar.gz
	tar -xzf output/tmp/preflight.tar.gz -C output/tmp
	mv output/tmp/preflight $@
	rm -rf output/tmp
	touch $@

.PHONY: pkg/goods/bins/local-artifact-mirror
pkg/goods/bins/local-artifact-mirror:
	mkdir -p pkg/goods/bins
	$(MAKE) -C local-artifact-mirror build GOOS=$(OS) GOARCH=$(ARCH)
	cp local-artifact-mirror/bin/local-artifact-mirror-$(OS)-$(ARCH) $@
	touch $@

pkg/goods/bins/fio: PLATFORM = linux/$(ARCH)
pkg/goods/bins/fio: Makefile fio/Dockerfile
ifneq ($(DISABLE_FIO_BUILD),1)
	mkdir -p pkg/goods/bins
	docker build -t fio --build-arg PLATFORM=$(PLATFORM) fio
	docker rm -f fio && docker run --name fio fio
	docker cp fio:/output/fio $@
	docker rm -f fio
	touch $@
endif

.PHONY: pkg/goods/internal/bins/kubectl-kots
pkg/goods/internal/bins/kubectl-kots:
	$(MAKE) output/bins/kubectl-kots-$(KOTS_VERSION)-$(ARCH)
	mkdir -p pkg/goods/internal/bins
	cp output/bins/kubectl-kots-$(KOTS_VERSION)-$(ARCH) $@

output/bins/kubectl-kots-%:
	mkdir -p output/bins
	mkdir -p output/tmp
	if [ "$(KOTS_BINARY_URL_OVERRIDE)" != "" ]; then \
	    curl --retry 5 --retry-all-errors -fL -o output/tmp/kots.tar.gz "$(KOTS_BINARY_URL_OVERRIDE)" ; \
	else \
	    curl --retry 5 --retry-all-errors -fL -o output/tmp/kots.tar.gz "https://github.com/replicatedhq/kots/releases/download/$(call split-hyphen,$*,1)/kots_$(OS)_$(call split-hyphen,$*,2).tar.gz" ; \
	fi
	tar -xzf output/tmp/kots.tar.gz -C output/tmp
	mv output/tmp/kots $@
	touch $@

.PHONY: output/bin/embedded-cluster-release-builder
output/bin/embedded-cluster-release-builder:
	mkdir -p output/bin
	CGO_ENABLED=0 go build -o output/bin/embedded-cluster-release-builder e2e/embedded-cluster-release-builder/main.go

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
embedded-cluster-linux-amd64: OS = linux
embedded-cluster-linux-amd64: ARCH = amd64
embedded-cluster-linux-amd64: static go.mod embedded-cluster
	mkdir -p ./output/bin
	cp ./build/embedded-cluster-$(OS)-$(ARCH) ./output/bin/$(APP_NAME)

# for testing
.PHONY: embedded-cluster-darwin-arm64
embedded-cluster-darwin-arm64: OS = darwin
embedded-cluster-darwin-arm64: ARCH = arm64
embedded-cluster-darwin-arm64: go.mod embedded-cluster
	mkdir -p ./output/bin
	cp ./build/embedded-cluster-$(OS)-$(ARCH) ./output/bin/$(APP_NAME)

.PHONY: embedded-cluster
embedded-cluster:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build \
		-tags osusergo,netgo \
		-ldflags="-s -w $(LD_FLAGS) -extldflags=-static" \
		-o ./build/embedded-cluster-$(OS)-$(ARCH) \
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
	make embedded-cluster-linux-amd64 \
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
