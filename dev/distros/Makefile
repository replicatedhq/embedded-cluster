SHELL := /bin/bash

.PHONY: list
list:
	@ls dockerfiles/*.Dockerfile | sed 's/^dockerfiles\///' | sed 's/\.Dockerfile$$//'

.PHONY: build-and-push-%
build-and-push-%:
	@$(MAKE) build-$*
	docker push replicated/ec-distro:$*

.PHONY: build-%
build-%:
	docker build -t replicated/ec-distro:$* -f dockerfiles/$*.Dockerfile .
