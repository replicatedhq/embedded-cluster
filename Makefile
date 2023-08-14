BUILDER_NAME=builder
APP_NAME=helmvm
ADMIN_CONSOLE_CHART_VERSION=1.100.1
KUBECTL_VERSION=v1.27.3
K0SCTL_VERSION=v0.15.2
TERRAFORM_VERSION=1.5.4
OPENEBS_VERSION=3.7.0
K0S_VERSION=v1.27.2+k0s.0
LD_FLAGS=-X github.com/replicatedhq/helmvm/pkg/defaults.K0sVersion=$(K0S_VERSION)

default: helmvm-linux-amd64

output/bin/yq:
	curl -L -o output/bin/yq https://github.com/mikefarah/yq/releases/download/v4.34.1/yq_linux_amd64
	chmod +x output/bin/yq

output/bin/helm:
	mkdir -p output/bin
	curl -fsSL "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3" | \
		PATH=$(PATH):output/bin HELM_INSTALL_DIR=output/bin USE_SUDO=false bash

pkg/goods/images/list.txt: pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz
	mkdir -p pkg/goods/images
	mkdir -p output/tmp/adminconsole
	tar -zxf "pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz" -C output/tmp/adminconsole
	output/bin/yq -r ".images[]" output/tmp/adminconsole/admin-console/values.yaml > pkg/goods/images/list.txt
	rm -rf output/tmp/adminconsole
	mkdir -p output/tmp/openebs
	tar -zxf "pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz" -C output/tmp/openebs
	output/bin/yq -r '(.localprovisioner.image + ":" + .localprovisioner.imageTag)' output/tmp/openebs/openebs/values.yaml >> pkg/goods/images/list.txt
	output/bin/yq -r '(.ndm.image + ":" + .ndm.imageTag)' output/tmp/openebs/openebs/values.yaml >> pkg/goods/images/list.txt
	output/bin/yq -r '(.ndmOperator.image + ":" + .ndmOperator.imageTag)' output/tmp/openebs/openebs/values.yaml >> pkg/goods/images/list.txt
	output/bin/yq -r '(.helper.image + ":" + .helper.imageTag)' output/tmp/openebs/openebs/values.yaml >> pkg/goods/images/list.txt
	rm -rf output/tmp/openebs

pkg/goods/bins/k0sctl/k0s-${K0S_VERSION}:
	mkdir -p pkg/goods/bins/k0sctl
	curl -L -o pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION) "https://github.com/k0sproject/k0s/releases/download/$(K0S_VERSION)/k0s-$(K0S_VERSION)-amd64"
	chmod +x pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION)

pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz: output/bin/helm
	output/bin/helm pull oci://registry.replicated.com/library/admin-console --version=$(ADMIN_CONSOLE_CHART_VERSION)
	mv admin-console-$(ADMIN_CONSOLE_CHART_VERSION).tgz pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz

pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz: output/bin/helm
	curl -L -o pkg/addons/openebs/charts/openebs-$(OPENEBS_VERSION).tgz https://github.com/openebs/charts/releases/download/openebs-$(OPENEBS_VERSION)/openebs-$(OPENEBS_VERSION).tgz

pkg/goods/bins/helmvm/terraform-linux-amd64:
	mkdir -p output/tmp/terraform
	curl -L -o output/tmp/terraform/terraform.zip https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_linux_amd64.zip
	unzip -o output/tmp/terraform/terraform.zip -d output/tmp/terraform
	mv output/tmp/terraform/terraform pkg/goods/bins/helmvm/terraform-linux-amd64

pkg/goods/bins/helmvm/terraform-darwin-amd64:
	mkdir -p output/tmp/terraform
	curl -L -o output/tmp/terraform/terraform.zip https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_darwin_amd64.zip
	unzip -o output/tmp/terraform/terraform.zip -d output/tmp/terraform
	mv output/tmp/terraform/terraform pkg/goods/bins/helmvm/terraform-darwin-amd64

pkg/goods/bins/helmvm/terraform-darwin-arm64:
	mkdir -p output/tmp/terraform
	curl -L -o output/tmp/terraform/terraform.zip https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_darwin_arm64.zip
	unzip -o output/tmp/terraform/terraform.zip -d output/tmp/terraform
	mv output/tmp/terraform/terraform pkg/goods/bins/helmvm/terraform-darwin-arm64

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

.PHONY: static
static: pkg/addons/adminconsole/charts/adminconsole-$(ADMIN_CONSOLE_CHART_VERSION).tgz \
	output/bin/yq pkg/goods/bins/k0sctl/k0s-$(K0S_VERSION) \
	pkg/goods/images/list.txt

.PHONY: static-darwin-arm64
static-darwin-arm64: pkg/goods/bins/helmvm/kubectl-darwin-arm64 pkg/goods/bins/helmvm/k0sctl-darwin-arm64 pkg/goods/bins/helmvm/terraform-darwin-arm64

.PHONY: static-darwin-amd64
static-darwin-amd64: pkg/goods/bins/helmvm/kubectl-darwin-amd64 pkg/goods/bins/helmvm/k0sctl-darwin-amd64 pkg/goods/bins/helmvm/terraform-darwin-amd64

.PHONY: static-linux-amd64
static-linux-amd64: pkg/goods/bins/helmvm/kubectl-linux-amd64 pkg/goods/bins/helmvm/k0sctl-linux-amd64 pkg/goods/bins/helmvm/terraform-linux-amd64

.PHONY: helmvm-linux-amd64
helmvm-linux-amd64: static static-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: helmvm-darwin-amd64
helmvm-darwin-amd64: static static-darwin-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: helmvm-darwin-arm64
helmvm-darwin-arm64: static static-darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LD_FLAGS)" -o ./output/bin/$(APP_NAME) ./cmd/helmvm

.PHONY: builder
builder: static static-linux-amd64
	CGO_ENABLED=0 go build -o ./output/bin/$(BUILDER_NAME) ./cmd/builder

.PHONY: integration-tests
integration-tests: helmvm-linux-amd64
	mkdir -p output/tmp
	ssh-keygen -t dsa -N "" -C "Integration Test Keys" -f output/tmp/id_rsa
	go test -timeout 30m -v ./integration

.PHONY: unit-tests
unit-tests:
	go test -v $(shell go list ./... | grep -v /integration)

.PHONY: vet
vet: static-linux-amd64 static
	go vet ./...

.PHONY: clean
clean:
	rm -rf output
	rm -rf pkg/addons/adminconsole/charts/*.tgz
	rm -rf pkg/addons/openebs/charts/*.tgz
	rm -rf pkg/goods/bins
	rm -rf pkg/goods/images
	rm -rf bundle
