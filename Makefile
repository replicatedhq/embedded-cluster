VERSION ?= $(shell git describe --tags --dirty)
UNAME := $(shell uname)
ARCH := $(shell uname -m)
APP_NAME = embedded-cluster
ADMIN_CONSOLE_CHART_REPO_OVERRIDE =
ADMIN_CONSOLE_CHART_VERSION = 1.111.0
ADMIN_CONSOLE_IMAGE_OVERRIDE = ttl.sh/ethan/kotsadm:24h
ADMIN_CONSOLE_MIGRATIONS_IMAGE_OVERRIDE =
ADMIN_CONSOLE_KURL_PROXY_IMAGE_OVERRIDE = ttl.sh/ethan/kurl-proxy:24h
EMBEDDED_OPERATOR_CHART_URL = oci://ttl.sh/ethan
EMBEDDED_OPERATOR_CHART_NAME = embedded-cluster-operator
EMBEDDED_OPERATOR_CHART_VERSION = 0.0.0-alpha.1
EMBEDDED_OPERATOR_BINARY_URL_OVERRIDE = https://repldev-ethan-test.s3.amazonaws.com/manager
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
KOTS_VERSION = alpha
KOTS_BINARY_URL_OVERRIDE = https://salah-test.s3.us-east-1.amazonaws.com/kots.tar.gz?response-content-disposition=inline&X-Amz-Security-Token=IQoJb3JpZ2luX2VjEDEaCXVzLWVhc3QtMSJHMEUCIQDtUBgqGM6PoWnrOJCVUch%2B%2FYy1FfxStYEC8jOlwD8lxAIgHATMPlYkJOqWlKgEGOKkVUQ4NnJ1HKVne4sUnD9DCYMqpwMI2f%2F%2F%2F%2F%2F%2F%2F%2F%2F%2FARADGgw0MjkxMTQyMTQ1MjYiDMJ5ZBSl%2F52Wfba6Gyr7AlYYzef2IAgyLAoUb1wyJk1X9I0uhuYcotXungxdqxDGOcfFE4hV9d4i8WoZplmkryiGBzUw%2Bjzpku90%2FI4GLNh9nEx54s2LssMYZKhBIrVLtRP%2FL27V0j%2Fz2tkG5pvOan9Xtw76QNWNk0POPSqgrkXZqfralXAvIOL311fKAW75EU%2BqD1r4d87Rg3EgJWuDLSxi6Cfe8TPDqp756SdqDbDb4EDLZdxOu2sO1GsC5bcmEicQ1Nbwh4dt3pUosI1o2KGCJhTaMGexUcIEJEEd1ufMTXw%2BFgKelgDvqWInS9OKD8pfikwv9XjiXzJkDS0q4Xzt%2F1FEkU%2B19EpEdoNWebAdEsuo3rlKO1Yea4Q39ZQEz1r1fX8sH79VrZAwXqVidLw3RLo0VQPWT5Bc8HnDvL%2FEZguSNq8mku1wcWllhMQgu4hEK7LCQNO2Xa6KzKv%2FfTan3%2BS1qxQbK2MYL4TeGue9J9vC65V2hEpbIkgcwxqbnfspq2JDYgFgmOAwkYDxswY6hAIUo3WwdTORwSvxb8RPomy15E5O6BIuv%2BTD9nRuzjTTYVsWwOdQBj%2FlXuqHyfk3zxdXiW8DJhXAQdSsf1hKXMKgcF%2BgwnUnZO7mKve%2FteeRCenbJKDvdb0VSIiKaMiPoPUWt1uF1ISDbMDcS4PBYicigenAh0PJ8pOyOnR%2B9TZCqI%2BDMmfm9rKkOS1ZA9p0bx9daU3w1fMHurqjIQ0UA59qJTkJyDDnL1nfVoLOjCydDwgDB5puiKJK6Xnwy63n3IDHVhfkM40J02iAX%2FUvrDfu2f3bd8QGkK8xZjg5IpyIPDJ3yS%2BMmwO%2BDraxn40%2BZ21k5bngd0V6HZPDxpvlaCofeIY19A%3D%3D&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date=20240626T162311Z&X-Amz-SignedHeaders=host&X-Amz-Expires=43200&X-Amz-Credential=ASIAWH2JTJB7JH7JTP6H%2F20240626%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Signature=6ec25e1182156abc30d97b20fd4843a0c6793d6795bbc0b032861d554bcb529b
LOCAL_ARTIFACT_MIRROR_IMAGE ?= registry.replicated.com/library/embedded-cluster-local-artifact-mirror
LOCAL_ARTIFACT_MIRROR_IMAGE_LOCATION = ${LOCAL_ARTIFACT_MIRROR_IMAGE}:$(subst +,-,$(VERSION))
LD_FLAGS = -X github.com/replicatedhq/embedded-cluster/pkg/defaults.K0sVersion=$(K0S_VERSION) \
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
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.Version=$(OPENEBS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/openebs.UtilsVersion=$(OPENEBS_UTILS_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs.Version=$(SEAWEEDFS_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.Version=$(REGISTRY_CHART_VERSION) \
	-X github.com/replicatedhq/embedded-cluster/pkg/addons/registry.ImageVersion=$(REGISTRY_IMAGE_VERSION) \
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
