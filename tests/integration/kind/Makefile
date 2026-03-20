SHELL := /bin/bash

DEBUG ?=
RUN ?=
SHORT ?= false

GO_BUILD_TAGS ?= containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,exclude_graphdriver_overlay

.PHONY: test
test: test-openebs test-registry test-velero test-adminconsole

.PHONY: test-openebs
test-openebs: openebs.test
	DEBUG=$(DEBUG) ./openebs.test \
		-test.v \
		-test.timeout=5m \
		-test.run='$(value RUN)' \
		-test.short=$(SHORT)

.PHONY: test-registry
test-registry: registry.test
	$(MAKE) -C ../../../operator build-ttl.sh \
		USE_CHAINGUARD=0
	DEBUG=$(DEBUG) SKIP_OPERATOR_IMAGE_BUILD=1 ./registry.test \
		-test.v \
		-test.timeout=15m \
		-test.run='$(value RUN)' \
		-test.short=$(SHORT)

.PHONY: test-velero
test-velero: velero.test
	DEBUG=$(DEBUG) ./velero.test \
		-test.v \
		-test.timeout=5m \
		-test.run='$(value RUN)' \
		-test.short=$(SHORT)

.PHONY: test-adminconsole
test-adminconsole: adminconsole.test
	DEBUG=$(DEBUG) ./adminconsole.test \
		-test.v \
		-test.timeout=5m \
		-test.run='$(value RUN)' \
		-test.short=$(SHORT)

.PHONY: clean
clean:
	rm -f *.test

openebs.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./openebs

registry.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./registry

velero.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./velero

adminconsole.test:
	go test -c -tags $(GO_BUILD_TAGS) \
		./adminconsole
