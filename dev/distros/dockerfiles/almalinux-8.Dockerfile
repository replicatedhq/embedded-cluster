FROM almalinux:8

# Only required for systemd-timesyncd
RUN dnf install -y epel-release

# Install necessary packages
RUN dnf install -y \
  sudo \
  systemd \
  openssh-server \
  ca-certificates \
  bash \
  coreutils \
  binutils \
  curl \
  procps-ng \
  ipvsadm \
  kmod \
  iproute \
  systemd-timesyncd \
  expect \
  vim \
  firewalld \
  --allowerasing

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
