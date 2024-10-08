FROM quay.io/centos/centos:stream9

# Install necessary packages
RUN dnf install -y \
  sudo \
  systemd \
  openssh-server \
  ca-certificates \
  bash \
  coreutils \
  curl \
  procps-ng \
  ipvsadm \
  kmod \
  iproute \
  chrony \
  expect \
  vim \
  --allowerasing

# Export kube config
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
