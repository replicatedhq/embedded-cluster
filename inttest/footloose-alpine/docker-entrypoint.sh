#!/bin/sh

# kubelet uses /sys/class/dmi/id/product_name to check if is running on a GCE VM and then attempts to query the metadata server
# this fails and kubelet fails to register the node
fix_product_name() {
  if [ -f /sys/class/dmi/id/product_name ]; then
    mkdir -p /k0s
    echo 'k0svm' > /k0s/product_name
    mount -o ro,bind /k0s/product_name /sys/class/dmi/id/product_name
  fi
}

fix_product_name

exec $@
