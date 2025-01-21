SHELL := /bin/bash

DEBUG ?=
PARALLEL ?= 2
RUN ?=

.PHONY: test
test: test-openebs

.PHONY: test-openebs
test-openebs:
	DEBUG=$(DEBUG) go test -v -tags exclude_graphdriver_btrfs \
		-timeout=5m \
		-parallel=$(PARALLEL) \
		-run='$(value RUN)' \
		./openebs/...
