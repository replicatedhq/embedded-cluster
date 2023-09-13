FROM golang:1.21.0-bullseye AS builder
RUN apt update -y
RUN apt install -y unzip
WORKDIR /helmvm-builder
COPY go.mod go.sum ./
COPY . .
RUN make builder && mv output/bin/builder builder
RUN make helmvm-linux-amd64 && mv output/bin/helmvm helmvm-linux-amd64
RUN make clean
RUN make helmvm-darwin-amd64 && mv output/bin/helmvm helmvm-darwin-amd64
RUN make clean
RUN make helmvm-darwin-arm64 && mv output/bin/helmvm helmvm-darwin-arm64

FROM alpine:3.18.2
RUN mkdir /helmvm
COPY --from=builder /helmvm-builder/helmvm-linux-amd64 /helmvm/helmvm-linux-amd64
COPY --from=builder /helmvm-builder/helmvm-darwin-amd64 /helmvm/helmvm-darwin-amd64
COPY --from=builder /helmvm-builder/helmvm-darwin-arm64 /helmvm/helmvm-darwin-arm64
COPY --from=builder /helmvm-builder/builder /helmvm/builder
EXPOSE 8080/tcp
CMD ["/helmvm/builder"]
