# this dockerfile is used to build the docker image for the builder
FROM golang:1.21 AS builder
WORKDIR /builder
COPY . .
RUN make builder


FROM ubuntu:22.04
WORKDIR /builder
RUN apt-get update -y
RUN apt-get install -y binutils
RUN mkdir /releases
COPY --from=builder /builder/output/bin/builder .
EXPOSE 8080
CMD ["./builder"]
