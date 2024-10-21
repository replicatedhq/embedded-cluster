FROM debian:bullseye-slim

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
  chrony \
  expect \
  vim

# Entrypoint for runtime configurations
COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
