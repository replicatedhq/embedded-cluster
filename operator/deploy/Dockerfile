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

COPY --from=build /app/operator/bin/manager /manager

RUN useradd -c 'eco operator manager user' -m -d /home/manager -s /bin/bash -u 1001 manager
USER 1001

ENTRYPOINT ["/manager"]
