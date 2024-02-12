# Embedded Cluster platform

This repository houses a cluster installation prototype that utilizes the k0s platform.
It showcases an alternative approach to deploying clusters and serves as a starting point for further exploration and advancement.
In Embedded Cluster, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc).

Embedded Cluster includes by default the Kots Admin Console and the OpenEBS Storage provisioner.

## Building and running

With the repository checked out locally, to compile you just need to run:

```
$ make embedded-cluster
```

The binary will be located on `output/bin/embedded-cluster`.

## Single node deployment

To create a single node deployment you can upload the Embedded Cluster binary to a Linux x86_64 machine and run:

```
$ ./embedded-cluster install
```

## Interacting with the cluster

Once the cluster has been deployed you can open a new terminal to interact with it using `kubectl`:

```
$ ./embedded-cluster shell
```

This will drop you in a new shell, this shell is configured to reach the cluster and includes shell completion:

```
ubuntu@ip-172-16-10-242:~$ ./embedded-cluster shell

    __4___
 _  \ \ \ \   Welcome to embedded-cluster debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or CTRL+d) to exit.
  \0\0\0\0\/  Happy hacking.
 ~~~~~~~~~~~
ubuntu@ip-172-16-10-242:~/.embedded-cluster/etc$ export KUBECONFIG="/home/ubuntu/.embedded-cluster/etc/kubeconfig"
ubuntu@ip-172-16-10-242:~/.embedded-cluster/etc$ export PATH="$PATH:/home/ubuntu/.embedded-cluster/bin"
ubuntu@ip-172-16-10-242:~/.embedded-cluster/etc$ source <(kubectl completion $(basename "/bin/bash"))
ubuntu@ip-172-16-10-242:~/.embedded-cluster/etc$
```
