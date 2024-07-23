VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
COREDNS_IMAGE = proxy.replicated.com/anonymous/replicated/ec-coredns
COREDNS_VERSION = 1.11.3-r3@sha256:7996a7ee8e1b7fec9a6dc216b01f0047cafbd551562bde44a2c6615ef8f3dbfc
CALICO_NODE_IMAGE = proxy.replicated.com/anonymous/replicated/ec-calico-node
CALICO_NODE_VERSION = 3.26.1-r16@sha256:7212746eda056c0b2833764d065e932fdddee2aced0170fd721197b19d13606e
CALICO_CNI_IMAGE = proxy.replicated.com/anonymous/replicated/ec-calico-cni
CALICO_CNI_VERSION = 3.26.1-r16@sha256:11d5bf25611ffc578e632e23e09767ca5a964f81ff311c47d2e98b686c2d0365
CALICO_KUBE_CONTROLLERS_IMAGE = proxy.replicated.com/anonymous/replicated/ec-calico-kube-controllers
CALICO_KUBE_CONTROLLERS_VERSION = 3.26.1-r16@sha256:74e845a0dbbd2b9ebd988c03ed53b88f2e2657c5defb2ed7796dda2583601111
METRICS_SERVER_IMAGE = proxy.replicated.com/anonymous/replicated/ec-metrics-server
METRICS_SERVER_VERSION = 0.6.4-r9@sha256:bd7d9ada28e299979174b2094d1eec7d653f793730b320dc7e90763c92452268
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE =
EMBEDDED_OPERATOR_UTILS_IMAGE ?= replicated/embedded-cluster-utils
EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION ?= $(subst +,-,$(VERSION))
EMBEDDED_OPERATOR_UTILS_IMAGE_LOCATION = proxy.replicated.com/anonymous/$(EMBEDDED_OPERATOR_UTILS_IMAGE):$(EMBEDDED_OPERATOR_UTILS_IMAGE_VERSION)
EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE =
KUBECTL_VERSION = v1.30.1
K0S_VERSION = v1.29.6+k0s.0
K0S_GO_VERSION = v1.29.6+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.10+k0s.0
K0S_BINARY_SOURCE_OVERRIDE =
PREVIOUS_K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.93.1
KOTS_VERSION = v$(shell awk '/^version/{print $$2}' pkg/addons/adminconsole/static/metadata.yaml | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
KOTS_BINARY_URL_OVERRIDE =
LOCAL_ARTIFACT_MIRROR_IMAGE ?= replicated/embedded-cluster-local-artifact-mirror
LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION ?= $(subst +,-,$(VERSION))
LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION = proxy.replicated.com/anonymous/$(LOCAL_ARTIFACT_MIRROR_IMAGE):$(LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION)
LD_FLAGS = \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.KubectlVersion=$(KUBECTL_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CoreDNSImage=$(COREDNS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CoreDNSVersion=$(COREDNS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoNodeImage=$(CALICO_NODE_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoNodeVersion=$(CALICO_NODE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoCNIImage=$(CALICO_CNI_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoCNIVersion=$(CALICO_CNI_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoKubeControllersImage=$(CALICO_KUBE_CONTROLLERS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoKubeControllersVersion=$(CALICO_KUBE_CONTROLLERS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.MetricsServerImage=$(METRICS_SERVER_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.MetricsServerVersion=$(METRICS_SERVER_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleMigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.AdminConsoleKurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.EmbeddedOperatorImageOverride=$(EMBEDDED_OPERATOR_IMAGE_OVERRIDE)

export PATH := $(shell pwd)/bin:$(PATH)

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
	go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o pkg/goods/bins/local-artifact-mirror ./cmd/local-artifact-mirror

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
embedded-cluster-linux-amd64: static go.mod
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/embedded-cluster

# for testing
.PHONY: embedded-cluster-darwin-arm64
embedded-cluster-darwin-arm64: go.mod
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/embedded-cluster

.PHONY: unit-tests
unit-tests:
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

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image: export IMAGE ?= $(LOCAL_ARTIFACT_MIRROR_IMAGE):$(LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION)
build-local-artifact-mirror-image: export PACKAGE_VERSION ?= $(LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION)
build-local-artifact-mirror-image: export MELANGE_CONFIG = deploy/packages/local-artifact-mirror/melange.tmpl.yaml
build-local-artifact-mirror-image: export APKO_CONFIG = deploy/images/local-artifact-mirror/apko.tmpl.yaml
build-local-artifact-mirror-image: melange-build apko-build

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image: export IMAGE ?= $(LOCAL_ARTIFACT_MIRROR_IMAGE):$(LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION)
build-and-push-local-artifact-mirror-image: export PACKAGE_VERSION ?= $(LOCAL_ARTIFACT_MIRROR_IMAGE_VERSION)
build-and-push-local-artifact-mirror-image: export MELANGE_CONFIG = deploy/packages/local-artifact-mirror/melange.tmpl.yaml
build-and-push-local-artifact-mirror-image: export APKO_CONFIG = deploy/images/local-artifact-mirror/apko.tmpl.yaml
build-and-push-local-artifact-mirror-image: melange-build apko-login apko-build-and-publish

CHAINGUARD_TOOLS_USE_DOCKER = 0
ifeq ($(CHAINGUARD_TOOLS_USE_DOCKER),"1")
MELANGE_CACHE_DIR ?= /go/pkg/mod
APKO_CMD = docker run -v $(shell pwd):/work -w /work -v $(shell pwd)/build/.docker:/root/.docker cgr.dev/chainguard/apko
MELANGE_CMD = docker run --privileged --rm -v $(shell pwd):/work -w /work -v "$(shell go env GOMODCACHE)":${MELANGE_CACHE_DIR} cgr.dev/chainguard/melange
else
MELANGE_CACHE_DIR ?= build/.melange-cache
APKO_CMD = apko
MELANGE_CMD = melange
endif

$(MELANGE_CACHE_DIR):
	mkdir -p $(MELANGE_CACHE_DIR)

.PHONY: apko-build
apko-build: export ARCHS ?= amd64
apko-build: check-env-IMAGE apko-template
	cd build && ${APKO_CMD} \
		build apko.yaml ${IMAGE} apko.tar \
		--arch ${ARCHS}

.PHONY: apko-build-and-publish
apko-build-and-publish: export ARCHS ?= amd64
apko-build-and-publish: check-env-IMAGE apko-template
	cd build && ${APKO_CMD} \
		publish apko.yaml ${IMAGE} \
		--arch ${ARCHS} | tee digest

.PHONY: apko-login
apko-login:
	rm -f build/.docker/config.json
	@ { [ "${PASSWORD}" = "" ] || [ "${USERNAME}" = "" ] ; } || \
	${APKO_CMD} \
		login -u "${USERNAME}" \
		--password "${PASSWORD}" "${REGISTRY}"

.PHONY: melange-build
melange-build: export ARCHS ?= amd64
melange-build: $(MELANGE_CACHE_DIR) melange-template
	${MELANGE_CMD} \
		keygen build/melange.rsa
	${MELANGE_CMD} \
		build build/melange.yaml \
		--arch ${ARCHS} \
		--signing-key build/melange.rsa \
		--cache-dir=$(MELANGE_CACHE_DIR) \
		--source-dir . \
		--out-dir build/packages/

.PHONY: melange-template
melange-template: check-env-MELANGE_CONFIG check-env-PACKAGE_VERSION
	mkdir -p build
	envsubst '$${PACKAGE_VERSION}' < ${MELANGE_CONFIG} > build/melange.yaml

.PHONY: apko-template
apko-template: check-env-APKO_CONFIG check-env-PACKAGE_VERSION
	mkdir -p build
	envsubst '$${PACKAGE_NAME} $${PACKAGE_VERSION}' < ${APKO_CONFIG} > build/apko.yaml

.PHONY: buildtools
buildtools:
	mkdir -p pkg/goods/bins pkg/goods/internal/bins
	touch pkg/goods/bins/BUILD pkg/goods/internal/bins/BUILD # compilation will fail if no files are present
	go build -o ./output/bin/buildtools ./cmd/buildtools

.PHONY: cache-files
cache-files: export EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE
cache-files:
	./scripts/cache-files.sh

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
MELANGE ?= $(LOCALBIN)/melange
APKO ?= $(LOCALBIN)/apko

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

melange: $(MELANGE)
$(MELANGE): $(LOCALBIN)
	go install chainguard.dev/melange@latest && \
		test -s $(GOBIN)/melange && \
		ln -sf $(GOBIN)/melange $(LOCALBIN)/melange

apko: $(APKO)
$(APKO): $(LOCALBIN)
	go install chainguard.dev/apko@latest && \
		test -s $(GOBIN)/apko && \
		ln -sf $(GOBIN)/apko $(LOCALBIN)/apko

print-%:
	@echo -n $($*)

check-env-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi
