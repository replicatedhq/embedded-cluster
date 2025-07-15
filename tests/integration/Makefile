SHELL := /bin/bash

DEBUG ?=
PARALLEL ?= 2
RUN ?=
SHORT ?= false

.PHONY: test
test: test-kind

.PHONY: test-kind
test-kind:
	$(MAKE) -C ./kind test

.PHONY: clean
clean:
	$(MAKE) -C ./kind clean
