VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_URL = oci://registry.replicated.com/library
ADMIN_CONSOLE_CHART_NAME = admin-console
ADMIN_CONSOLE_CHART_VERSION = 1.108.10-build.1
ADMIN_CONSOLE_IMAGE_OVERRIDE = ttl.sh/automated-8907544653/kotsadm@sha256:fba192ab34a218ecfb88e5b1d15bfa9233809914f1d21b67dcf79b0f28172737
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_CHART_URL = oci://registry.replicated.com/library
EMBEDDED_OPERATOR_CHART_NAME = embedded-cluster-operator
EMBEDDED_OPERATOR_CHART_VERSION = 0.29.1
EMBEDDED_OPERATOR_UTILS_IMAGE = busybox:1.36.1
EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE =
OPENEBS_CHART_URL = https://openebs.github.io/openebs
OPENEBS_CHART_NAME = openebs/openebs
OPENEBS_CHART_VERSION = 4.0.1
OPENEBS_UTILS_VERSION = 4.0.0
REGISTRY_CHART_URL = https://helm.twun.io
REGISTRY_CHART_NAME = twuni/docker-registry
REGISTRY_CHART_VERSION = 2.2.3
REGISTRY_IMAGE_VERSION = 2.8.3
VELERO_CHART_URL = https://vmware-tanzu.github.io/helm-charts
VELERO_CHART_NAME = vmware-tanzu/velero
VELERO_CHART_VERSION = 6.0.0
VELERO_IMAGE_VERSION = v1.13.2
VELERO_AWS_PLUGIN_IMAGE_VERSION = v1.9.2
KUBECTL_VERSION = v1.30.0
K0S_VERSION = v1.29.4+k0s.0
PREVIOUS_K0S_VERSION ?= v1.28.9+k0s.0
K0S_BINARY_SOURCE_OVERRIDE =
TROUBLESHOOT_VERSION = v0.91.0
KOTS_VERSION = v$(shell echo $(ADMIN_CONSOLE_CHART_VERSION) | sed 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\).*/\1/')
LD_FLAGS = -X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sBinaryURL=$(K0S_BINARY_SOURCE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.TroubleshootVersion=$(TROUBLESHOOT_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.KubectlVersion=$(KUBECTL_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartURL=$(ADMIN_CONSOLE_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartName=$(ADMIN_CONSOLE_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.Version=$(ADMIN_CONSOLE_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.MigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ChartURL=$(EMBEDDED_OPERATOR_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ChartName=$(EMBEDDED_OPERATOR_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.Version=$(EMBEDDED_OPERATOR_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.UtilsImage=$(EMBEDDED_OPERATOR_UTILS_IMAGE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ImageOverride=$(EMBEDDED_CLUSTER_OPERATOR_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.ChartURL=$(OPENEBS_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.ChartName=$(OPENEBS_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.Version=$(OPENEBS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.UtilsVersion=$(OPENEBS_UTILS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.ChartURL=$(REGISTRY_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.ChartName=$(REGISTRY_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.Version=$(REGISTRY_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.ImageVersion=$(REGISTRY_IMAGE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.ChartURL=$(VELERO_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.ChartName=$(VELERO_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.Version=$(VELERO_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.VeleroTag=$(VELERO_IMAGE_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/velero.AwsPluginTag=$(VELERO_AWS_PLUGIN_IMAGE_VERSION)

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
	curl -L -o output/tmp/kots/kots.tar.gz https://github.com/replicatedhq/kots/releases/download/$(KOTS_VERSION)/kots_linux_amd64.tar.gz
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

go.mod: Makefile
	go get github.com/k0sproject/k0s@$(K0S_VERSION)
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
	go test -v $(shell go list ./... | grep -v /e2e)

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
