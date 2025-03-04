FROM ubuntu:jammy

# Install necessary packages
RUN apt-get update && apt-get install -y \
  sudo \
  systemd \
  openssh-server \
  ca-certificates \
  bash \
  coreutils \
  binutils \
  curl \
  inotify-tools \
  ipvsadm \
  kmod \
  iproute2 \
  systemd-timesyncd \
  expect \
  vim

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
