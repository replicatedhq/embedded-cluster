SHELL := /bin/bash

.PHONY: list
list:
	@ls dockerfiles/*.Dockerfile | sed 's/^dockerfiles\///' | sed 's/\.Dockerfile$$//'

.PHONY: build-%
build-%:
	docker build -t ec-$* -f dockerfiles/$*.Dockerfile .
