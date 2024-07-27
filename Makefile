VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE =
EMBEDDED_OPERATOR_UTILS_IMAGE ?= replicated/embedded-cluster-utils
EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION ?= $(subst +,-,$(VERSION))
EMBEDDED_OPERATOR_UTILS_IMAGE_LOCATION = proxy.replicated.com/anonymous/$(EMBEDDED_OPERATOR_UTILS_IMAGE):$(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
KUBECTL_VERSION = v1.30.1
K0S_VERSION = v1.29.6+k0s.0
K0S_GO_VERSION = v1.29.6+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.10+k0s.0
K0S_BINARY_SOURCE_OVERRIDE =
PREVIOUS_K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.97.0
KOTS_VERSION = v$(shell awk '/^version/{print $$2}' pkg/addons/adminconsole/static/metadata.yaml | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
KOTS_BINARY_URL_OVERRIDE =
# TODO: move this to a manifest file
LOCAL_ARTIFACT_MIRROR_IMAGE ?= proxy.replicated.com/anonymous/replicated/embedded-cluster-local-artifact-mirror:1.8.0-k8s-1.29-40-g380cf43-dirty@sha256:56c3acdc66404a419d6661328c618dfe8497f49383f05d61cf52ae6101ad7690
LD_FLAGS = \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.KubectlVersion=$(KUBECTL_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/versions.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleMigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleKurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.UtilsImage=$(EMBEDDED_OPERATOR_UTILS_IMAGE_LOCATION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.EmbeddedOperatorImageOverride=$(EMBEDDED_OPERATOR_IMAGE_OVERRIDE)

export PATH := $(shell pwd)/bin:$(PATH)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.DEFAULT_GOAL := default
default: embedded-cluster-linux-amd64

pkg/goods/bins/k0s: Makefile
	mkdir -p pkg/goods/bins
	if [ "$(K0S_BINARY_SOURCE_OVERRIDE)" != "" ]; then \
	    curl -fL -o pkg/goods/bins/k0s "$(K0S_BINARY_SOURCE_OVERRIDE)" ; \
	else \
	    curl -fL -o pkg/goods/bins/k0s "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-amd64" ; \
	fi
	chmod +x pkg/goods/bins/k0s
	touch pkg/goods/bins/k0s

pkg/goods/bins/kubectl: Makefile
	mkdir -p pkg/goods/bins
	curl -fL -o pkg/goods/bins/kubectl "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"
	chmod +x pkg/goods/bins/kubectl
	touch pkg/goods/bins/kubectl

pkg/goods/bins/kubectl-support_bundle: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/support-bundle
	curl -fL -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_linux_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/kubectl-support_bundle
	touch pkg/goods/bins/kubectl-support_bundle

pkg/goods/bins/kubectl-preflight: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/preflight
	curl -fL -o output/tmp/preflight/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/preflight_linux_amd64.tar.gz
	tar -xzf output/tmp/preflight/preflight.tar.gz -C output/tmp/preflight
	mv output/tmp/preflight/preflight pkg/goods/bins/kubectl-preflight
	touch pkg/goods/bins/kubectl-preflight

pkg/goods/bins/local-artifact-mirror: Makefile
	mkdir -p pkg/goods/bins
	$(MAKE) -C lam bin/local-artifact-mirror
	cp lam/bin/local-artifact-mirror-$(GOOS)-$(GOARCH) pkg/goods/bins/local-artifact-mirror

pkg/goods/internal/bins/kubectl-kots: Makefile
	mkdir -p pkg/goods/internal/bins
	mkdir -p output/tmp/kots
	if [ "$(KOTS_BINARY_URL_OVERRIDE)" != "" ]; then \
	    curl -fL -o output/tmp/kots/kots.tar.gz "$(KOTS_BINARY_URL_OVERRIDE)" ; \
	else \
	    curl -fL -o output/tmp/kots/kots.tar.gz https://github.com/replicatedhq/kots/releases/download/$(KOTS_VERSION)/kots_linux_amd64.tar.gz ; \
	fi
	tar -xzf output/tmp/kots/kots.tar.gz -C output/tmp/kots
	mv output/tmp/kots/kots pkg/goods/internal/bins/kubectl-kots
	touch pkg/goods/internal/bins/kubectl-kots

output/tmp/release.tar.gz: e2e/kots-release-install/*
	mkdir -p output/tmp
	tar -czf output/tmp/release.tar.gz -C e2e/kots-release-install .

output/bin/embedded-cluster-release-builder:
	mkdir -p output/bin
	CGO_ENABLED=0 go build -o output/bin/embedded-cluster-release-builder e2e/embedded-cluster-release-builder/main.go

.PHONY: embedded-release
embedded-release: embedded-cluster-linux-amd64 output/tmp/release.tar.gz output/bin/embedded-cluster-release-builder
	./output/bin/embedded-cluster-release-builder output/bin/embedded-cluster output/tmp/release.tar.gz output/bin/embedded-cluster

.PHONY: go.mod
go.mod: Makefile
	go get github.com/k0sproject/k0s@$(K0S_GO_VERSION)
	go mod tidy

.PHONY: static
static: pkg/goods/bins/k0s \
	pkg/goods/bins/kubectl-preflight \
	pkg/goods/bins/kubectl \
	pkg/goods/bins/kubectl-support_bundle \
	pkg/goods/bins/local-artifact-mirror \
	pkg/goods/internal/bins/kubectl-kots

.PHONY: embedded-cluster-linux-amd64
embedded-cluster-linux-amd64: GOOS = linux
embedded-cluster-linux-amd64: GOARCH = amd64
embedded-cluster-linux-amd64: static go.mod embedded-cluster
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

.PHONY: vet
vet: static
	go vet ./...

.PHONY: e2e-tests
e2e-tests: embedded-release
	go test -timeout 45m -parallel 1 -failfast -v ./e2e

.PHONY: e2e-test
e2e-test:
	go test -timeout 45m -v ./e2e -run $(TEST_NAME)$

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

.PHONY: build-utils-image
build-utils-image: export IMAGE ?= $(EMBEDDED_OPERATOR_UTILS_IMAGE):$(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
build-utils-image: export PACKAGE_VERSION ?= $(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
build-utils-image: export APKO_CONFIG = deploy/images/utils/apko.tmpl.yaml
build-utils-image: apko-build

.PHONY: build-and-push-utils-image
build-and-push-utils-image: export IMAGE ?= $(EMBEDDED_OPERATOR_UTILS_IMAGE):$(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
build-and-push-utils-image: export PACKAGE_VERSION ?= $(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
build-and-push-utils-image: export APKO_CONFIG = deploy/images/utils/apko.tmpl.yaml
build-and-push-utils-image: apko-login apko-build-and-publish

.PHONY: buildtools
buildtools:
	mkdir -p pkg/goods/bins pkg/goods/internal/bins
	touch pkg/goods/bins/BUILD pkg/goods/internal/bins/BUILD # compilation will fail if no files are present
	go build -o ./output/bin/buildtools ./cmd/buildtools

.PHONY: cache-files
cache-files: export EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE
cache-files:
	./scripts/cache-files.sh

print-%:
	@echo -n $($*)
