# adapted from https://github.com/k0sproject/k0s/blob/2b7adcd574b0f20ccd1a2da515cf8c81d57b2241/inttest/bootloose-alpine/Dockerfile
FROM ubuntu:jammy

# Install necessary packages
RUN apt-get update && apt-get install -y \
  sudo \
  systemd \
  openssh-server \
  bash \
  coreutils \
  curl \
  inotify-tools \
  ipvsadm \
  kmod \
  iproute2 \
  systemd-timesyncd \
  vim

# Override timesyncd config to allow it to run in containers
COPY ./timesyncd-override.conf /etc/systemd/system/systemd-timesyncd.service.d/override.conf

# Export kube config
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

CMD ["/sbin/init"]
