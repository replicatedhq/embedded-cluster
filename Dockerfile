# TODO
ARG ARCH

FROM docker.io/library/golang:1.20-alpine as build
RUN apk add make
WORKDIR /go/src/github.com/emosbaugh/helmbin
COPY . .
ARG VERSION="dev"
RUN make build

FROM docker.io/library/${ARCH}alpine

RUN apk add --no-cache bash coreutils findutils iptables curl tini

ENV KUBECONFIG=/var/lib/replicated/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
COPY --from=build /go/src/github.com/emosbaugh/helmbin/bin/helmbin /usr/local/bin/helmbin

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]

CMD ["helmbin", "server"]
