# Embedded Cluster platform

This repository houses a cluster installation prototype that utilizes the k0s and k0sctl platforms.
It showcases an alternative approach to deploying clusters and serves as a starting point for further exploration and advancement.
In Embedded Cluster, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc).
Remote hosts are managed using SSH.

Embedded Cluster includes by default the Kots Admin Console and the OpenEBS Storage provisioner, you can very easily embed your own Helm Chart to the binary.

## Building and running

With the repository checked out locally, to compile you just need to run:

```
$ make embedded-cluster-linux-amd64
```

The binary will be located on `output/bin/embedded-cluster`.

## Single node deployment

To create a single node deployment you can upload the Embedded Cluster binary to a Linux x86_64 machine and run:

```
$ ./embedded-cluster install
```

## Multi node deployment

To create a multi node deployment you can run the following command and then follow the instructions:

```
$ ./embedded-cluster install --multi-node
```

In this case, it's not necessary to execute this command exclusively on a Linux x86_64 machine. You have the flexibility to use any architecture for the process.

## Deploying Individual Nodes

Embedded Cluster also facilitates deploying individual nodes through the use of tokens, deviating from the centralized approach.
To follow this path, you need to exclude yourself from the centralized management facilitated via SSH.

### Installing a Multi-Node Setup using Token-Based Deployment

All operations should be executed directly on the Linux servers and require root privileges.
Begin by deploying the first node:

```
server-0# embedded-cluster install
```

After the cluster is online, you can generate a token to enable the addition of other nodes:

```
server-0# embedded-cluster node token create --role controller
INFO[0000] Creating node join token for role controller
WARN[0000] You are opting out of the centralized cluster management.
WARN[0000] Through the centralized management you can manage all your
WARN[0000] cluster nodes from a single location. If you decide to move
WARN[0000] on the centralized management won't be available anymore
? Do you want to use continue ? Yes
INFO[0002] Token created successfully.
INFO[0002] This token is valid for 24h0m0s hours.
INFO[0002] You can now run the following command in a remote node to add it
INFO[0002] to the cluster as a "controller" node:
embedded-cluster node join --role "controller" "<token redacted>"
server-0#
```

Upon generating the token, you will be prompted to continue; press Enter to proceed (you will be opting out of the centralized management).
The role in the command above can be either "controller" or "worker", with the generated token tailored to the selected role.
Copy the command provided and run it on the server you wish to join to the cluster:

```
server-1# embedded-cluster node join --role "controller" "<token redacted>"
```

For this to function, you must ensure that the Embedded Cluster binary is present on all nodes within the cluster.


### Upgrading clusters

If your installation employs centralized management, simply download the newer version of Embedded Cluster and execute:

```
$ embedded-cluster apply
```

For installations without centralized management, download Embedded Cluster, upload it to each server in your cluster, and execute the following command as **root** on each server:

```
# embedded-cluster node upgrade
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

## Miscellaneous

Embedded Cluster stores its data under `$HOME/.embedded-cluster` directory, you may want to create a backup of the directory, specially the `$HOME/.embedded-cluster/etc` directory.  Inside the `$HOME/.embedded-cluster/etc` directory you will find the `k0sctl.yaml` and the `kubeconfig` files, the first is used when installing or upgrading a cluster and the latter is used when accessing the cluster with `kubectl` (a copy of `kubectl` is also kept under `$HOME/.embedded-cluster/bin` directory and you may want to include it into your PATH).

If you want to use an already existing `k0sctl.yaml` configuration during the `install` command you can do so by using the `--config` flag.

Inside `$HOME/.embedded-cluster/bin` you will find a copy of `k0sctl` binary used to bootstrap the cluster, you can use it to manage the cluster as well (e.g. `$HOME/.embedded-cluster/bin/k0sctl kubeconfig --config $HOME/.embedded-cluster/etc/k0sctl.yaml`).

## Experimental features

### Stop and Start nodes for maintenance

Once the cluster is deployed you can easily stop and start nodes using the following:

```
~: embedded-cluster node list
~: embedded-cluster node stop node-0
~: embedded-cluster node start node-0
```
