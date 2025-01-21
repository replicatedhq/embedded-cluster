SHELL := /bin/bash

DEBUG ?=
RUN ?=

CONTAINER_NAME = integration-test-manager

.PHONY: test
test: DISTRO = debian-bookworm
test:
	@-docker rm -f $(CONTAINER_NAME) 2>/dev/null ; true
	@docker run -d \
		--name $(CONTAINER_NAME) \
		--privileged \
		--restart=unless-stopped \
		-v $(shell pwd)/manager.test:/wrk/manager.test \
		-v $(shell pwd)/../../../pkg/goods/bins/manager:/wrk/bin/manager \
		-e DATA_DIR=/wrk \
		replicated/ec-distro:$(DISTRO)
	@sleep 5 # wait for container to be ready
	docker exec -it \
		-e DEBUG=$(DEBUG) \
		$(CONTAINER_NAME) \
		/wrk/manager.test \
		-test.v \
		-test.timeout=5m \
		-test.run='$(value RUN)'

.PHONY: clean
clean:
	rm -f manager.test
	-docker rm -f $(CONTAINER_NAME) 2>/dev/null ; true

.PHONY: build
build: manager.test ../../../pkg/goods/bins/manager

manager.test:
	GOOS=linux go test -c .

../../../pkg/goods/bins/manager:
	$(MAKE) -C ../../../ pkg/goods/bins/manager
