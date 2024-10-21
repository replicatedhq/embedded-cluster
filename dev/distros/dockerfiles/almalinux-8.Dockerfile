FROM almalinux:8

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

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
