# this dockerfile builds an image containing the local-artifact-mirror binary
# installed on /usr/local/bin/local-artifact-mirror.
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make pkg/goods/bins/local-artifact-mirror

FROM cgr.dev/chainguard/wolfi-base:latest
COPY --from=builder /src/pkg/goods/bins/local-artifact-mirror /usr/local/bin/local-artifact-mirror
ENTRYPOINT ["/usr/local/bin/local-artifact-mirror"]
