#!/usr/bin/env bash

include Makefile.variables

ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)

SONOBUOY_VERSION ?= 0.56.11
sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_$(OS)_$(ARCH).tar.gz

curl = curl -L --silent

bins = bin/sonobuoy

.PHONY: all
all: $(bins) .footloose-alpine.stamp

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

footloose_alpine_build_cmdline := \
	--build-arg GOLANG_IMAGE=$(golang_buildimage) \
	--build-arg ALPINE_VERSION=$(alpine_patch_version) \
	--build-arg ETCD_VERSION=$(etcd_version) \
	--build-arg HELM_VERSION=$(helm_version) \
	--build-arg KUBE_VERSION=$(kubernetes_version) \
	-t footloose-alpine \
	-f footloose-alpine/Dockerfile \
	footloose-alpine

.footloose-alpine.stamp:
	docker build --build-arg TARGETARCH=$(ARCH) $(footloose_alpine_build_cmdline)
	touch $@

# This is a special target to test the footloose alpine image locally for all supported platforms.
.PHONY: check-footloose-alpine-buildx
check-footloose-alpine-buildx:
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 $(footloose_alpine_build_cmdline)

TIMEOUT ?= 4m

.PHONY: $(smoketests)
$(smoketests): HELMBIN_PATH ?= $(realpath ../bin/helmbin)
$(smoketests): .footloose-alpine.stamp
$(smoketests): TEST_PACKAGE ?= $(subst check-,,$@)
$(smoketests):
	HELMBIN_PATH='$(HELMBIN_PATH)' \
	K0S_PATH='$(HELMBIN_PATH)' \
	go test -count=1 -v -timeout $(TIMEOUT) github.com/emosbaugh/helmbin/inttest/$(TEST_PACKAGE)

.PHONY: clean
clean:
	-[ -x bin/sonobuoy ] && bin/sonobuoy delete
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp
