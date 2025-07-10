SHELL := /bin/bash

include ../common.mk

.PHONY: swagger
swagger: swag
	swag fmt -g api.go
	swag init --parseDependency --v3.1 -g api.go

.PHONY: swag
swag:
	which swag || (go install github.com/swaggo/swag/v2/cmd/swag@latest)

.PHONY: unit-tests
unit-tests:
	go test -race -tags $(GO_BUILD_TAGS) -v $(shell go list ./... | grep -v '/integration')

.PHONY: test-integration
test-integration:
	go test -race -tags $(GO_BUILD_TAGS) -v ./integration/...
