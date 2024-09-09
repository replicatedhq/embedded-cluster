FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
COPY kinds/go.mod kinds/go.sum ./kinds/
COPY utils/go.mod kinds/go.sum ./utils/
RUN go mod download

COPY common.mk common.mk
COPY operator/ operator/
COPY kinds/ kinds/
COPY utils/ utils/

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" make -C operator build

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates make \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /replicatedhq/embedded-cluster/operator

COPY --from=build /app/operator/bin/manager /manager

RUN groupadd -r manager && useradd -r -u 1000 -g manager manager
USER 1000

ENTRYPOINT ["/manager"]
