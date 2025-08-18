SHELL := /bin/bash

DEBUG ?=
RUN ?=

K0S_VERSION ?= v1.30.14+k0s.0

GO_BUILD_TAGS ?= containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,exclude_graphdriver_overlay

.PHONY: test
test: export TEST_K0S_BINARY=bin/k0s-$(K0S_VERSION)
test: k0s.test bin/k0s-$(K0S_VERSION)
	DEBUG=$(DEBUG) go run ./run/main.go $(RUN) TestInstall_basic
	DEBUG=$(DEBUG) go run ./run/main.go $(RUN) TestInstall_workerProfile

.PHONY: clean
clean:
	@rm -f *.test

k0s.test:
	GOOS=linux go test -c -tags $(GO_BUILD_TAGS) .

bin:
	mkdir -p bin

bin/k0s-%: bin
	curl --retry 5 --retry-all-errors -fL -o $@ \
		"https://github.com/k0sproject/k0s/releases/download/$*/k0s-$*-$(shell go env GOARCH)"
	chmod +x $@
	touch $@
