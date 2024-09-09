# adapted from https://github.com/k0sproject/k0s/blob/2b7adcd574b0f20ccd1a2da515cf8c81d57b2241/inttest/bootloose-alpine/Dockerfile
FROM almalinux:latest

# Install necessary packages
RUN dnf install -y \
  sudo \
  systemd \
  openssh-server \
  bash \
  coreutils \
  curl \
  procps-ng \
  ipvsadm \
  kmod \
  iproute \
  chrony \
  vim \
  --allowerasing

# Export kube config
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

# Default command to run systemd
CMD ["/sbin/init"]
