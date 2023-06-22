# Helmbin

Helmbin is a command-line tool that enables users to render and run Helm Charts within a Virtual Machine.

## Introduction

Helmbin embeds Kubernetes and Helm charts in a single installable binary. It is designed to simplify the process of rendering and deploying Helm Charts by providing a lightweight and isolated environment that can then be installed in a Virtual Machine. With Helmbin, you can quickly deploy Helm-based applications without the need for complex setup or dependencies.

## Installation

To install Helmbin, follow these steps:

1. Download the latest release from the [Helmbin releases page](https://github.com/replicatedhq/helmbin/releases).
2. Extract the downloaded archive to a directory of your choice.
3. Add the directory containing the Helmbin binary to your system's `PATH` variable.

## Contributing

### Building

If you want to build the binary on your own you can, after cloning this repository, run:

```bash
$ make build
```

The compiled binary will be then placed under the `bin/` subdirectory.

### Testing your changes

Before submitting a PR you may want to run a few local checks, namely `linter` and `tests`. You can do it by running the following commands in your cloned repository directory:

```bash
$ make lint
$ make test
```

## Running

By default `Helmbin` deploys [Kots](https://kots.io/) Helm Chart, support for additional Helm Charts are coming. Once you have Helmbin installed you can run it in either foreground or as a `systemd` unit.

### Running on foreground

To run it in foreground you can use:

```bash
$ sudo helmbin run
```

Be aware that all logs are going to be printed in the current terminal and Helmbin tends to be very verbose. After a few seconds you should be able to access [Kots](https://kots.io/) by accessing the Virtual Machine IP Address on port `8800` (e.g. http://192.168.0.1:8800/)

### Running as a systemd unit

Systemd is a system initialization and service management framework for Linux that provides a more efficient and centralized approach to managing and controlling system processes. To install and run Helmbin as a systemd unit you can run:

```bash
$ sudo helmbin install
```

This will create a system unit service called `k0scontroller`. Helmbin can then be stopped and started on demand with the following commands:

```bash
$ sudo helmbin stop
$ sudo helmbin start
```

You can enable Helmbin startup during boot by running:

```bash
$ sudo systemctl enable k0scontroller.service
```

To visualize Helmbin logs you can use `journalctl`:

```bash
$ journalctl -u k0scontroller.service -f
```

### Visualizing Kubernetes objects

Helmbin deploys a lightweight Kubernetes compatible cluster behind the scenes. To access Kubernetes entities deployed in this cluster you can use the `helmbin kubectl` command as shown below:

```bash
$ sudo helmbin kubectl get pods -A
NAMESPACE     NAME                                           READY   STATUS    RESTARTS      AGE
default       kotsadm-b46879d6b-sphz6                        1/1     Running   0             21h
default       kotsadm-minio-0                                1/1     Running   0             21h
default       kotsadm-rqlite-0                               1/1     Running   0             21h
kube-system   calico-kube-controllers-6d48c8cf5c-44944       1/1     Running   0             21h
kube-system   calico-node-n7xpw                              1/1     Running   0             21h
kube-system   coredns-878bb57ff-bhqh5                        1/1     Running   0             21h
kube-system   konnectivity-agent-2txhh                       1/1     Running   0             21h
kube-system   kube-proxy-2g4rf                               1/1     Running   0             21h
kube-system   metrics-server-7f86dff975-lsnsc                1/1     Running   0             21h
openebs       openebs-localpv-provisioner-5cfcbff6fb-dnzrm   1/1     Running   0             21h
openebs       openebs-ndm-2q2d9                              1/1     Running   0             21h
openebs       openebs-ndm-operator-68d4455cf9-gg7qt          1/1     Running   0             21h
$
```
