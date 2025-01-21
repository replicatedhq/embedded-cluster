SHELL := /bin/bash

DEBUG ?=
PARALLEL ?= 2
RUN ?=

.PHONY: test
test: test-kind test-manager

.PHONY: test-kind
test-kind:
	$(MAKE) -C ./kind test

.PHONY: test-manager
test-manager:
	$(MAKE) -C ./manager clean build test
