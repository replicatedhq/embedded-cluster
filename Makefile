VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_CHART_VERSION = 1.112.1
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_CHART_URL = oci://registry.replicated.com/library
EMBEDDED_OPERATOR_CHART_NAME = embedded-cluster-operator
EMBEDDED_OPERATOR_CHART_VERSION = 0.40.2
EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE =
EMBEDDED_OPERATOR_UTILS_IMAGE = proxy.replicated.com/anonymous/busybox:1.36.1@sha256:50aa4698fa6262977cff89181b2664b99d8a56dbca847bf62f2ef04854597cf8
EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE =
OPENEBS_CHART_VERSION = 4.1.0
OPENEBS_IMAGE_TAG = 4.1.0@sha256:7a9bb247229564c74b5c4981304bf614f1b0157797a52c6d882d5f1cb057b838
OPENEBS_UTILS_IMAGE_TAG = 4.1.0@sha256:a0f8fd9457388d5728e8e9c41f1cf5dc3b21ef8f00d7b019520101812ae602dc
SEAWEEDFS_CHART_VERSION = 4.0.0
SEAWEEDFS_IMAGE_TAG = 3.69@sha256:88eaab1398cd95b1e49f7210a36af389b4ebc896cccd2ef6f4baff3fa1b245fd
REGISTRY_CHART_VERSION = 2.2.3
REGISTRY_IMAGE_TAG = 2.8.1@sha256:85d4cd48754d4e8e4d4fe933bbde017d969e0e20775507add9faca39064bf9b0
VELERO_CHART_VERSION = 6.3.0
VELERO_IMAGE_TAG = v1.13.2@sha256:9a833acbc2391f50c994e3c1379e038e5a64372f6d645d89b4321b732f8dc8ef
VELERO_AWS_PLUGIN_IMAGE_TAG = v1.10.0@sha256:0b4fe36bbd5c7e484750bf21e25274cecbb72b30b097a72dc3e599430590bdfc
VELERO_KUBECTL_IMAGE_TAG = 1.29@sha256:89bde5352cd988ed9440ff0956a7ceb00415923aea9d34fc0ac82d9b3eee8af7
VELERO_RESTORE_HELPER_IMAGE_TAG = v1.13.2@sha256:83943510812496ad7ba6750f440645b313fd676db460fda7dcbf70f437db775c
KUBECTL_VERSION = v1.30.1
K0S_VERSION = v1.29.5+k0s.0-ec.0
K0S_GO_VERSION = v1.29.5+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.10+k0s.0
K0S_BINARY_SOURCE_OVERRIDE = https://ec-k0s-binaries.s3.amazonaws.com/k0s-v1.29.5%2Bk0s.0-ec.0
PREVIOUS_K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.93.1
KOTS_VERSION = v$(shell echo $(ADMIN_CONSOLE_CHART_VERSION) | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
KOTS_BINARY_URL_OVERRIDE =
LOCAL_ARTIFACT_MIRROR_IMAGE ?= registry.replicated.com/library/embedded-cluster-local-artifact-mirror
LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION = ${LOCAL_ARTIFACT_MIRROR_IMAGE}:$(subst +,-,$(VERSION))
LD_FLAGS = \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.KubectlVersion=$(KUBECTL_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.LocalArtifactMirrorImage=$(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartRepoOverride=$(ADMIN_CONSOLE_CHART_REPO_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.Version=$(ADMIN_CONSOLE_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.MigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KurlProxyImageOverride=$(ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.KotsVersion=$(KOTS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.Version=$(EMBEDDED_OPERATOR_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.UtilsImage=$(EMBEDDED_OPERATOR_UTILS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ImageOverride=$(EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.OpenEBSChartVersion=$(OPENEBS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.OpenEBSImageTag=$(OPENEBS_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.OpenEBSUtilsImageTag=$(OPENEBS_UTILS_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs.SeaweedFSChartVersion=$(SEAWEEDFS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs.SeaweedFSImageTag=$(SEAWEEDFS_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.RegistryChartVersion=$(REGISTRY_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.RegistryImageTag=$(REGISTRY_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroChartVersion=$(VELERO_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroImageTag=$(VELERO_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroAWSPluginImageTag=$(VELERO_AWS_PLUGIN_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroKubectlImageTag=$(VELERO_KUBECTL_IMAGE_TAG) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroRestoreHelperImageTag=$(VELERO_RESTORE_HELPER_IMAGE_TAG)

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

print-%:
	@echo -n $($*)

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image:
	docker build --platform linux/amd64 -t $(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION) -f deploy/local-artifact-mirror/Dockerfile .

.PHONY: push-local-artifact-mirror-image
push-local-artifact-mirror-image:
	docker push $(LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION)

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image: build-local-artifact-mirror-image push-local-artifact-mirror-image

.PHONY: buildtools
buildtools:
	go build -o ./output/bin/buildtools ./cmd/buildtools

.PHONY: cache-files
cache-files: export EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE
cache-files:
	./scripts/cache-files.sh
