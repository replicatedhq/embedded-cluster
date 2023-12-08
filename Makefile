VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_URL = oci://registry.replicated.com/library
ADMIN_CONSOLE_CHART_NAME = admin-console
ADMIN_CONSOLE_CHART_VERSION = 1.104.5
ADMIN_CONSOLE_IMAGE_OVERRIDE =
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
EMBEDDED_OPERATOR_CHART_URL = oci://registry.replicated.com/library
EMBEDDED_OPERATOR_CHART_NAME = embedded-cluster-operator
EMBEDDED_OPERATOR_CHART_VERSION = 0.6.0
OPENEBS_CHART_URL = https://openebs.github.io/charts
OPENEBS_CHART_NAME = openebs/openebs
OPENEBS_CHART_VERSION = 3.9.0
KUBECTL_VERSION = v1.28.4
K0SCTL_VERSION = v0.16.0
K0S_VERSION = v1.28.4+k0s.0+ec.0
K0S_BINARY_SOURCE_OVERRIDE = "https://tf-embedded-cluster-binaries.s3.amazonaws.com/v1.28.4%2Bk0s.0%2Bec.0"
TROUBLESHOOT_VERSION = v0.78.1
LD_FLAGS = -X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.Version=$(VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sBinaryURL=$(K0S_BINARY_SOURCE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartURL=$(ADMIN_CONSOLE_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ChartName=$(ADMIN_CONSOLE_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.Version=$(ADMIN_CONSOLE_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.ImageOverride=$(ADMIN_CONSOLE_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole.MigrationsImageOverride=$(ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ChartURL=$(EMBEDDED_OPERATOR_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.ChartName=$(EMBEDDED_OPERATOR_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator.Version=$(EMBEDDED_OPERATOR_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.ChartURL=$(OPENEBS_CHART_URL) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.ChartName=$(OPENEBS_CHART_NAME) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.Version=$(OPENEBS_CHART_VERSION)

default: embedded-cluster-linux-amd64

pkg/goods/bins/k0sctl/k0s-${K0S_VERSION}:
	mkdir -p pkg/goods/bins/k0sctl
	if [ "$(K0S_BINARY_SOURCE_OVERRIDE)" != "" ]; then \
	    curl -L -o pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION) "$(K0S_BINARY_SOURCE_OVERRIDE)" ; \
	else \
	    curl -L -o pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION) "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-amd64" ; \
	fi
	chmod +x pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION)

pkg/goods/bins/embedded-cluster/kubectl-linux-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/kubectl-linux-amd64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"
	chmod +x pkg/goods/bins/embedded-cluster/kubectl-linux-amd64

pkg/goods/bins/embedded-cluster/kubectl-darwin-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/kubectl-darwin-amd64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl"
	chmod +x pkg/goods/bins/embedded-cluster/kubectl-darwin-amd64

pkg/goods/bins/embedded-cluster/kubectl-darwin-arm64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/kubectl-darwin-arm64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/darwin/arm64/kubectl"
	chmod +x pkg/goods/bins/embedded-cluster/kubectl-darwin-arm64

pkg/goods/bins/embedded-cluster/k0sctl-linux-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/k0sctl-linux-amd64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-linux-x64"
	chmod +x pkg/goods/bins/embedded-cluster/k0sctl-linux-amd64

pkg/goods/bins/embedded-cluster/k0sctl-darwin-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/k0sctl-darwin-amd64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-darwin-x64"
	chmod +x pkg/goods/bins/embedded-cluster/k0sctl-darwin-amd64

pkg/goods/bins/embedded-cluster/k0sctl-darwin-arm64:
	mkdir -p pkg/goods/bins/embedded-cluster
	curl -L -o pkg/goods/bins/embedded-cluster/k0sctl-darwin-arm64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-darwin-arm64"
	chmod +x pkg/goods/bins/embedded-cluster/k0sctl-darwin-arm64

pkg/goods/bins/embedded-cluster/kubectl-support_bundle-linux-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_linux_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/embedded-cluster/kubectl-support_bundle-linux-amd64

pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-amd64:
	mkdir -p pkg/goods/bins/embedded-cluster
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_darwin_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-amd64

pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-arm64:
	mkdir -p pkg/goods/bins/embedded-cluster
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_darwin_arm64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-arm64

pkg/goods/bins/embedded-cluster/kubectl-preflight:
	mkdir -p pkg/goods/bins/embedded-cluster
	mkdir -p output/tmp/preflight
	curl -L -o output/tmp/preflight/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/preflight_linux_amd64.tar.gz
	tar -xzf output/tmp/preflight/preflight.tar.gz -C output/tmp/preflight
	mv output/tmp/preflight/preflight pkg/goods/bins/embedded-cluster/kubectl-preflight

release.tar.gz:
	mkdir -p output/tmp
	tar -czf release.tar.gz -C e2e/kots-release .

.PHONY: embedded-release
embedded-release: embedded-cluster-linux-amd64 release.tar.gz
	objcopy --input-target binary --output-target binary --rename-section .data=sec_bundle release.tar.gz release.o
	@if ! objcopy --update-section sec_bundle=release.o output/bin/embedded-cluster ; then \
		objcopy --add-section sec_bundle=release.o output/bin/embedded-cluster ; \
	fi

.PHONY: static
static: pkg/goods/bins/embedded-cluster/kubectl-preflight \
	pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION)

.PHONY: static-darwin-arm64
static-darwin-arm64: pkg/goods/bins/embedded-cluster/kubectl-darwin-arm64 pkg/goods/bins/embedded-cluster/k0sctl-darwin-arm64 pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-arm64

.PHONY: static-darwin-amd64
static-darwin-amd64: pkg/goods/bins/embedded-cluster/kubectl-darwin-amd64 pkg/goods/bins/embedded-cluster/k0sctl-darwin-amd64 pkg/goods/bins/embedded-cluster/kubectl-support_bundle-darwin-amd64

.PHONY: static-linux-amd64
static-linux-amd64: pkg/goods/bins/embedded-cluster/kubectl-linux-amd64 pkg/goods/bins/embedded-cluster/k0sctl-linux-amd64 pkg/goods/bins/embedded-cluster/kubectl-support_bundle-linux-amd64

.PHONY: embedded-cluster-linux-amd64
embedded-cluster-linux-amd64: static static-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/embedded-cluster

.PHONY: embedded-cluster-darwin-amd64
embedded-cluster-darwin-amd64: static static-darwin-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/embedded-cluster

.PHONY: embedded-cluster-darwin-arm64
embedded-cluster-darwin-arm64: static static-darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/embedded-cluster

.PHONY: unit-tests
unit-tests:
	go test -v $(shell go list ./... | grep -v /e2e)

.PHONY: vet
vet: static-linux-amd64 static
	go vet ./...

.PHONY: e2e-tests
e2e-tests: embedded-release
	mkdir -p output/tmp
	rm -rf output/tmp/id_rsa*
	ssh-keygen -t rsa -N "" -C "Integration Test Key" -f output/tmp/id_rsa
	go test -timeout 45m -parallel 1 -failfast -v ./e2e

.PHONY: e2e-test
e2e-test: embedded-release
	mkdir -p output/tmp
	rm -rf output/tmp/id_rsa*
	ssh-keygen -t rsa -N "" -C "Integration Test Key" -f output/tmp/id_rsa
	go test -timeout 45m -v ./e2e -run $(TEST_NAME)$

.PHONY: clean
clean:
	rm -rf output
	rm -rf pkg/addons/adminconsole/charts/*.tgz
	rm -rf pkg/addons/openebs/charts/*.tgz
	rm -rf pkg/goods/bins
