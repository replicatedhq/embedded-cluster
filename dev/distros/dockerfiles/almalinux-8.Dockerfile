FROM almalinux:8

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
