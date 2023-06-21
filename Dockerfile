# TODO
ARG ARCH

FROM docker.io/library/golang:1.20-alpine as build
RUN apk add --no-cache bash curl make openssl
WORKDIR /go/src/github.com/replicatedhq/helmbin
COPY . .
ARG VERSION="dev"
RUN make build

FROM docker.io/library/${ARCH}alpine

RUN apk add --no-cache bash coreutils curl findutils iptables tini

ENV KUBECONFIG=/var/lib/replicated/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
COPY --from=build /go/src/github.com/replicatedhq/helmbin/bin/helmbin /usr/local/bin/helmbin

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]

CMD ["helmbin", "run"]
