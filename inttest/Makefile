SHELL := /bin/bash

export DEBUG ?= 1

.PHONY: test
test: test-openebs

.PHONY: test-openebs
test-openebs:
	go test -v --timeout 5m ./openebs
