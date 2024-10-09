FROM ubuntu:jammy

# Install necessary packages
RUN apt-get update && apt-get install -y \
  sudo \
  systemd \
  openssh-server \
  ca-certificates \
  bash \
  coreutils \
  curl \
  inotify-tools \
  ipvsadm \
  kmod \
  iproute2 \
  iptables \
  dnsutils \
  chrony \
  expect \
  vim

# Export kube config
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
