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

# Disable getty service as it's flaky and doesn't apply in containers
RUN systemctl mask getty@tty1.service

# Export kube config
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

# Default command to run systemd
CMD ["/sbin/init"]
