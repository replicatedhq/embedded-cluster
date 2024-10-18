SHELL := /bin/bash

.PHONY: test
test: test-registry

.PHONY: test-registry
test-registry:
	go test -v --timeout 1m ./registry
