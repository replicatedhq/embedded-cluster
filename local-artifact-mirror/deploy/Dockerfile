FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
COPY kinds/go.mod kinds/go.sum ./kinds/
COPY utils/go.mod kinds/go.sum ./utils/
RUN go mod download

COPY common.mk common.mk
COPY local-artifact-mirror/ local-artifact-mirror/
COPY pkg/ pkg/
COPY cmd/ cmd/
COPY kinds/ kinds/
COPY utils/ utils/

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" make -C local-artifact-mirror build
RUN cp local-artifact-mirror/bin/local-artifact-mirror-$(go env GOOS)-$(go env GOARCH) local-artifact-mirror/bin/local-artifact-mirror

FROM debian:bookworm-slim

COPY --from=build /app/local-artifact-mirror/bin/local-artifact-mirror /usr/bin/local-artifact-mirror
RUN ln -s /usr/bin/local-artifact-mirror /usr/local/bin/local-artifact-mirror

RUN useradd -c 'eco lam user' -m -d /home/lam -s /bin/bash -u 1001 lam
USER 1001

ENTRYPOINT ["/manager"]
