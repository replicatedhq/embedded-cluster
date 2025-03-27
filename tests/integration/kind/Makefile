SHELL := /bin/bash

DEBUG ?=
PARALLEL ?= 2
RUN ?=

.PHONY: test
test: test-openebs test-registry

.PHONY: test-openebs
test-openebs: openebs.test
	DEBUG=$(DEBUG) ./openebs.test \
		-test.v \
		-test.timeout=5m \
		-test.parallel=$(PARALLEL) \
		-test.run='$(value RUN)'

.PHONY: test-registry
test-registry: registry.test
	DEBUG=$(DEBUG) ./registry.test \
		-test.v \
		-test.timeout=15m \
		-test.parallel=$(PARALLEL) \
		-test.run='$(value RUN)'

.PHONY: clean
clean:
	rm -f openebs.test
	rm -f registry.test

openebs.test:
	go test -c -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay" \
		./openebs

registry.test:
	go test -c -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay" \
		./registry
