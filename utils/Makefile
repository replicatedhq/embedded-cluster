SHELL := /bin/bash

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: unit-tests
unit-tests:
	go test -v ./...
