SHELL := /bin/bash

DEBUG ?=
RUN ?=

GO_BUILD_TAGS ?= containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,exclude_graphdriver_overlay

.PHONY: test
test: test-openebs test-registry

.PHONY: test-openebs
test-openebs: openebs.test
	DEBUG=$(DEBUG) ./openebs.test \
		-test.v \
		-test.timeout=5m \
		-test.run='$(value RUN)'

.PHONY: test-registry
test-registry: registry.test
	$(MAKE) -C ../../../operator build-and-push-operator-image \
		USE_CHAINGUARD=0 \
		IMAGE_NAME=ttl.sh/replicated/embedded-cluster-operator-image
	DEBUG=$(DEBUG) SKIP_OPERATOR_IMAGE_BUILD=1 ./registry.test \
		-test.v \
		-test.timeout=15m \
		-test.run='$(value RUN)'

.PHONY: clean
clean:
	rm -f *.test

openebs.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./openebs

registry.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./registry
