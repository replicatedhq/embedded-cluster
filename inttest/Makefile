SHELL := /bin/bash

DEBUG ?=
PARALLEL ?= 1

.PHONY: test
test: test-openebs

.PHONY: test-openebs
test-openebs:
	DEBUG=$(DEBUG) go test -v -timeout 5m -parallel $(PARALLEL) ./openebs/...
