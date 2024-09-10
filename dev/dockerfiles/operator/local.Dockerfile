FROM golang:1.23 AS build

WORKDIR /replicatedhq/embedded-cluster/operator

COPY go.mod go.sum ./
COPY kinds/go.mod kinds/go.sum ./kinds/
COPY utils/go.mod utils/go.sum ./utils/
RUN go mod download
