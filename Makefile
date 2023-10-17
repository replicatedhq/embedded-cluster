VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = helmvm
ADMIN_CONSOLE_CHART_VERSION = 1.103.2
KUBECTL_VERSION = v1.28.2
K0SCTL_VERSION = v0.16.0
OPENEBS_VERSION = 3.9.0
K0S_VERSION = v1.28.2+k0s.0
TROUBLESHOOT_VERSION = v0.76.1
LD_FLAGS = -X github.com/replicatedhq/helmvm/pkg/defaults.K0sVersion=$(K0S_VERSION) -X github.com/replicatedhq/helmvm/pkg/defaults.Version=$(VERSION)

default: helmvm-linux-amd64

output/bin/helm:
	mkdir -p output/bin
	curl -fsSL "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3" | \
		PATH=$(PATH):output/bin HELM_INSTALL_DIR=output/bin USE_SUDO=false bash

pkg/goods/bins/k0sctl/k0s-${K0S_VERSION}:
	mkdir -p pkg/goods/bins/k0sctl
	curl -L -o pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION) "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-amd64"
	chmod +x pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION)

pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz: output/bin/helm
	output/bin/helm pull oci://registry.replicated.com/library/admin-console --version=$(ADMIN_CONSOLE_CHART_VERSION)
	mv admin-console-$(ADMIN_CONSOLE_CHART_VERSION).tgz pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz

pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz: output/bin/helm
	curl -L -o pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz https://github.com/openebs/charts/releases/download/openebs-$(OPENEBS_VERSION)/openebs-$(OPENEBS_VERSION).tgz

pkg/goods/bins/helmvm/kubectl-linux-amd64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/kubectl-linux-amd64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"
	chmod +x pkg/goods/bins/helmvm/kubectl-linux-amd64

pkg/goods/bins/helmvm/kubectl-darwin-amd64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/kubectl-darwin-amd64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl"
	chmod +x pkg/goods/bins/helmvm/kubectl-darwin-amd64

pkg/goods/bins/helmvm/kubectl-darwin-arm64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/kubectl-darwin-arm64 "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/darwin/arm64/kubectl"
	chmod +x pkg/goods/bins/helmvm/kubectl-darwin-arm64

pkg/goods/bins/helmvm/k0sctl-linux-amd64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/k0sctl-linux-amd64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-linux-x64"
	chmod +x pkg/goods/bins/helmvm/k0sctl-linux-amd64

pkg/goods/bins/helmvm/k0sctl-darwin-amd64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/k0sctl-darwin-amd64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-darwin-x64"
	chmod +x pkg/goods/bins/helmvm/k0sctl-darwin-amd64

pkg/goods/bins/helmvm/k0sctl-darwin-arm64:
	mkdir -p pkg/goods/bins/helmvm
	curl -L -o pkg/goods/bins/helmvm/k0sctl-darwin-arm64 "https://github.com/k0sproject/k0sctl/releases/download/$(K0SCTL_VERSION)/k0sctl-darwin-arm64"
	chmod +x pkg/goods/bins/helmvm/k0sctl-darwin-arm64

pkg/goods/bins/helmvm/kubectl-support_bundle-linux-amd64:
	mkdir -p pkg/goods/bins/helmvm
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_linux_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/helmvm/kubectl-support_bundle-linux-amd64

pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-amd64:
	mkdir -p pkg/goods/bins/helmvm
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_darwin_amd64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-amd64

pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-arm64:
	mkdir -p pkg/goods/bins/helmvm
	mkdir -p output/tmp/support-bundle
	curl -L -o output/tmp/support-bundle/support-bundle.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/support-bundle_darwin_arm64.tar.gz
	tar -xzf output/tmp/support-bundle/support-bundle.tar.gz -C output/tmp/support-bundle
	mv output/tmp/support-bundle/support-bundle pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-arm64

pkg/goods/bins/helmvm/kubectl-preflight:
	mkdir -p pkg/goods/bins/helmvm
	mkdir -p output/tmp/preflight
	curl -L -o output/tmp/preflight/preflight.tar.gz https://github.com/replicatedhq/troubleshoot/releases/download/$(TROUBLESHOOT_VERSION)/preflight_linux_amd64.tar.gz
	tar -xzf output/tmp/preflight/preflight.tar.gz -C output/tmp/preflight
	mv output/tmp/preflight/preflight pkg/goods/bins/helmvm/kubectl-preflight

.PHONY: static
static: pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz \
	pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz \
	pkg/goods/bins/helmvm/kubectl-preflight \
	pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION)

.PHONY: static-darwin-arm64
static-darwin-arm64: pkg/goods/bins/helmvm/kubectl-darwin-arm64 pkg/goods/bins/helmvm/k0sctl-darwin-arm64 pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-arm64

.PHONY: static-darwin-amd64
static-darwin-amd64: pkg/goods/bins/helmvm/kubectl-darwin-amd64 pkg/goods/bins/helmvm/k0sctl-darwin-amd64 pkg/goods/bins/helmvm/kubectl-support_bundle-darwin-amd64

.PHONY: static-linux-amd64
static-linux-amd64: pkg/goods/bins/helmvm/kubectl-linux-amd64 pkg/goods/bins/helmvm/k0sctl-linux-amd64 pkg/goods/bins/helmvm/kubectl-support_bundle-linux-amd64

.PHONY: helmvm-linux-amd64
helmvm-linux-amd64: static static-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: helmvm-darwin-amd64
helmvm-darwin-amd64: static static-darwin-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: helmvm-darwin-arm64
helmvm-darwin-arm64: static static-darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: unit-tests
unit-tests:
	go test -v $(shell go list ./... | grep -v /e2e)

.PHONY: vet
vet: static-linux-amd64 static
	go vet ./...

.PHONY: e2e-tests
e2e-tests: helmvm-linux-amd64
	mkdir -p output/tmp
	rm -rf output/tmp/id_rsa*
	ssh-keygen -t rsa -N "" -C "Integration Test Key" -f output/tmp/id_rsa
	go test -timeout 45m -parallel 1 -failfast -v ./e2e

.PHONY: e2e-test
e2e-test: helmvm-linux-amd64
	mkdir -p output/tmp
	rm -rf output/tmp/id_rsa*
	ssh-keygen -t rsa -N "" -C "Integration Test Key" -f output/tmp/id_rsa
	go test -timeout 45m -v ./e2e -run $(TEST_NAME)$

.PHONY: create-e2e-workflows
create-e2e-workflows:
	./e2e/hack/create-e2e-workflows.sh

.PHONY: clean
clean:
	rm -rf output
	rm -rf pkg/addons/adminconsole/charts/*.tgz
	rm -rf pkg/addons/openebs/charts/*.tgz
	rm -rf pkg/goods/bins
