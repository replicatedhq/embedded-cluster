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
