SHELL := /bin/bash

.PHONY: build-%
build-%:
	docker build -t ec-$* -f dockerfiles/$*.Dockerfile .
