VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
COREDNS_IMAGE = proxy.replicated.com/anonymous/ttl.sh/ec/coredns
COREDNS_VERSION = 1.11.3
CALICO_NODE_IMAGE = proxy.replicated.com/anonymous/ttl.sh/ec/calico-node
CALICO_NODE_VERSION = 3.28.0-r7
METRICS_SERVER_IMAGE = proxy.replicated.com/anonymous/ttl.sh/ec/metrics-server
METRICS_SERVER_VERSION = 0.6.4
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_CHART_VERSION = 1.111.0
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_CHART_URL = oci://registry.replicated.com/library
EMBEDDED_OPERATOR_CHART_NAME = embedded-cluster-operator
EMBEDDED_OPERATOR_CHART_VERSION = 0.36.5
EMBEDDED_OPERATOR_UTILS_IMAGE = busybox:1.36.1
EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE =
OPENEBS_CHART_VERSION = 4.1.0
OPENEBS_UTILS_VERSION = 4.1.0
SEAWEEDFS_CHART_VERSION = 4.0.0
REGISTRY_CHART_VERSION = 2.2.3
REGISTRY_IMAGE_VERSION = 2.8.3
VELERO_CHART_VERSION = 6.3.0
VELERO_IMAGE_VERSION = v1.13.2
VELERO_AWS_PLUGIN_IMAGE_VERSION = v1.9.2
KUBECTL_VERSION = v1.30.1
K0S_VERSION = v1.29.5+k0s.0-ec.0
K0S_GO_VERSION = v1.29.5+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.8+k0s.0
K0S_BINARY_SOURCE_OVERRIDE = https://ec-k0s-binaries.s3.amazonaws.com/k0s-v1.29.5%2Bk0s.0-ec.0
PREVIOUS_K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.93.1
KOTS_VERSION = v$(shell echo $(ADMIN_CONSOLE_CHART_VERSION) | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
KOTS_BINARY_URL_OVERRIDE =
LOCAL_ARTIFACT_MIRROR_IMAGE ?= registry.replicated.com/library/embedded-cluster-local-artifact-mirror
LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION = ${LOCAL_ARTIFACT_MIRROR_IMAGE}:$(subst +,-,$(VERSION))
LD_FLAGS = -X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.KubectlVersion=$(KUBECTL_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CoreDNSImage=$(COREDNS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CoreDNSVersion=$(COREDNS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoNodeImage=$(CALICO_NODE_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.CalicoNodeVersion=$(CALICO_NODE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.MetricsServerImage=$(METRICS_SERVER_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/config/images.MetricsServerVersion=$(METRICS_SERVER_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.Version=$(ADMIN_CONSOLE_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.MigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.Version=$(EMBEDDED_OPERATOR_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.UtilsImage=$(EMBEDDED_OPERATOR_UTILS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ImageOverride=$(EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.Version=$(OPENEBS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.UtilsVersion=$(OPENEBS_UTILS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs.Version=$(SEAWEEDFS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.Version=$(REGISTRY_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.ImageVersion=$(REGISTRY_IMAGE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.Version=$(VELERO_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroTag=$(VELERO_IMAGE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.AwsPluginTag=$(VELERO_AWS_PLUGIN_IMAGE_VERSION)

export PATH := $(shell pwd)/bin:$(PATH)

.DEFAULT_GOAL := default
default: embedded-cluster-linux-amd64

pkg/goods/bins/k0s: Makefile
	mkdir -p pkg/goods/bins
	if [ "$(K0S_BINARY_SOURCE_OVERRIDE)" != "" ]; then \
	    curl -L -o pkg/goods/bins/k0s "$(K0S_BINARY_SOURCE_OVERRIDE)" ; \
	else \
	    curl -L -o pkg/goods/bins/k0s "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-amd64" ; \
	fi
	chmod +x pkg/goods/bins/k0s
	touch pkg/goods/bins/k0s

pkg/goods/bins/kubectl: Makefile
	mkdir -p pkg/goods/bins
	curl -L -o pkg/goods/bins/kubectl "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"
	chmod +x pkg/goods/bins/kubectl
	touch pkg/goods/bins/kubectl

pkg/goods/bins/kubectl-support_bundle: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_linux_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/kubectl-support_bundle
	touch pkg/goods/bins/kubectl-support_bundle

pkg/goods/bins/kubectl-preflight: Makefile
	mkdir -p pkg/goods/bins
	mkdir -p output/tmp/preflight
	curl -L -o output/tmp/preflight/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/preflight_linux_amd64.tar.gz
	tar -xzf output/tmp/preflight/preflight.tar.gz -C output/tmp/preflight
	mv output/tmp/preflight/preflight pkg/goods/bins/kubectl-preflight
	touch pkg/goods/bins/kubectl-preflight

pkg/goods/bins/local-artifact-mirror: Makefile
	mkdir -p pkg/goods/bins
	CGO_ENABLED=0 go build -o pkg/goods/bins/local-artifact-mirror ./cmd/local-artifact-mirror

pkg/goods/internal/bins/kubectl-kots: Makefile
	mkdir -p pkg/goods/internal/bins
	mkdir -p output/tmp/kots
	if [ "$(KOTS_BINARY_URL_OVERRIDE)" != "" ]; then \
	    curl -L -o output/tmp/kots/kots.tar.gz "$(KOTS_BINARY_URL_OVERRIDE)" ; \
	else \
	    curl -L -o output/tmp/kots/kots.tar.gz https://github.com/replicatedhq/kots/releases/download/$(KOTS_VERSION)/kots_linux_amd64.tar.gz ; \
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

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image:
	docker build -t $(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION) -f Dockerfile .

.PHONY: push-local-artifact-mirror-image
push-local-artifact-mirror-image:
	docker push $(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION)

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image: build-local-artifact-mirror-image push-local-artifact-mirror-image

CHAINGUARD_TOOLS_USE_DOCKER = 0
ifeq ($(CHAINGUARD_TOOLS_USE_DOCKER),"1")
MELANGE_CACHE_DIR = /go/pkg/mod
APKO_CMD = docker run -v "${PWD}":/work -w /work -v "${PWD}"/build/.docker:/root/.docker cgr.dev/chainguard/apko
MELANGE_CMD = docker run --privileged --rm -v "${PWD}":/work -w /work -v "$(shell go env GOMODCACHE)":${MELANGE_CACHE_DIR} cgr.dev/chainguard/melange
else
MELANGE_CACHE_DIR = $(shell go env GOMODCACHE)
APKO_CMD = apko
MELANGE_CMD = melange
endif

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
melange-build: melange-template
	mkdir -p build
	for f in pkg cmd go.mod go.sum Makefile ; do \
		rm -rf "build/$$f" && cp -r $$f build/ ; \
	done
	${MELANGE_CMD} \
		keygen build/melange.rsa
	${MELANGE_CMD} \
		build build/melange.yaml \
		--arch ${ARCHS} \
		--signing-key build/melange.rsa \
		--cache-dir=$(MELANGE_CACHE_DIR) \
		--out-dir build/packages/

.PHONY: melange-template
melange-template: check-env-MELANGE_CONFIG check-env-VERSION
	mkdir -p build
	envsubst '$${VERSION}' < ${MELANGE_CONFIG} > build/melange.yaml

.PHONY: apko-template
apko-template: check-env-APKO_CONFIG check-env-VERSION
	mkdir -p build
	envsubst '$${VERSION}' < ${APKO_CONFIG} > build/apko.yaml

.PHONY: buildtools
buildtools:
	go build -o ./output/bin/buildtools ./cmd/buildtools

.PHONY: cache-files
cache-files: export EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE
cache-files:
	./scripts/cache-files.sh

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

bin/apko:
	mkdir -p bin
	go install chainguard.dev/apko@latest && \
		test -s $(GOBIN)/apko && \
		ln -sf $(GOBIN)/apko bin/apko

bin/melange:
	mkdir -p bin
	go install chainguard.dev/melange@latest && \
		test -s $(GOBIN)/melange && \
		ln -sf $(GOBIN)/melange bin/melange

print-%:
	@echo -n $($*)

check-env-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi