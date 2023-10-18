# HelmVM platform

This repository houses a cluster installation prototype that utilizes the k0s and k0sctl platforms. It showcases an alternative approach to deploying clusters and serves as a starting point for further exploration and advancement. In HelmVM, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc). Remote hosts are managed using SSH.

HelmVM includes by default the Kots Admin Console and the OpenEBS Storage provisioner, you can very easily embed your own Helm Chart to the binary.

## Building and running

With the repository checked out locally, to compile you just need to run:

```
$ make helmvm-linux-amd64
```

The binary will be located on `output/bin/helmvm`.
You can also build binaries for other architectures with the following targets: `helmvm-darwin-amd64` and  `helmvm-darwin-arm64` are available.

## Single node deployment

To create a single node deployment you can upload the HelmVM binary to a Linux x86_64 machine and run:

```
$ ./helmvm install
```

## Multi node deployment

To create a multi node deployment you can run the following command and then follow the instructions:

```
$ ./helmvm install --multi-node
```

In this case, it's not necessary to execute this command exclusively on a Linux x86_64 machine. You have the flexibility to use any architecture for the process.

## Deploying Individual Nodes

HelmVM also facilitates deploying individual nodes through the use of tokens, deviating from the centralized approach.
To follow this path, you need to exclude yourself from the centralized management facilitated via SSH.

### Installing a Multi-Node Setup using Token-Based Deployment

All operations should be executed directly on the Linux servers and require root privileges.
Begin by deploying the first node:

```
server-0# helmvm install
```

After the cluster is online, you can generate a token to enable the addition of other nodes:

```
server-0# helmvm node token create --role controller
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
helmvm node join --role "controller" "<token redacted>"
server-0# 
```

Upon generating the token, you will be prompted to continue; press Enter to proceed (you will be opting out of the centralized management).
The role in the command above can be either "controller" or "worker", with the generated token tailored to the selected role.
Copy the command provided and run it on the server you wish to join to the cluster:

```
server-1# helmvm node join --role "controller" "<token redacted>"
```

For this to function, you must ensure that the HelmVM binary is present on all nodes within the cluster.


### Upgrading clusters

If your installation employs centralized management, simply download the newer version of HelmVM and execute:

```
$ helmvm apply
```

For installations without centralized management, download HelmVM, upload it to each server in your cluster, and execute the following command as **root** on each server:

```
# helmvm node upgrade
```

## Interacting with the cluster

Once the cluster has been deployed you can open a new terminal to interact with it using `kubectl`:

```
$ ./helmvm shell
```

This will drop you in a new shell, this shell is configured to reach the cluster:

```
ubuntu@ip-172-16-10-242:~$ ./helmvm shell

    __4___
 _  \ \ \ \   Welcome to helmvm debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or CTRL+d) to exit.
  \0\0\0\0\/  Happy hacking.
 ~~~~~~~~~~~
ubuntu@ip-172-16-10-242:~/helmvm/etc$ export KUBECONFIG="/home/ubuntu/helmvm/etc/kubeconfig"
ubuntu@ip-172-16-10-242:~/helmvm/etc$ export PATH="$PATH:/home/ubuntu/helmvm/bin"
ubuntu@ip-172-16-10-242:~/helmvm/etc$
```

## Embedding your own Helm Chart

HelmVM allows you to embed your own Helm Charts so they are installed by default when the cluster is installed or updated. For sake of documenting this let's create a hypothetical scenario: you have a software called `rocks` that is packaged as a Helm Chart and is ready to be installed in any Kubernetes Cluster.

Your Helm Chart is in a file called `rocks-1.0.0.tgz` and you already have a copy of HelmVM binary in your $PATH. To embed your Chart you can run:

```
$ helmvm embed --chart rocks-1.0.0.tgz --output rocks
```
This command will create a binary called `rocks` in the current directory, this command is a copy of HelmVM binary with your Helm Chart embedded into it. You can then use the `rocks` binary to install a cluster that automatically deploys your `rocks-1.0.0.tgz` Helm Chart.

If you want to provide a customised `values.yaml` during the Helm Chart installation you can also embed it into the binary. You can do that with the following command:

```
$ helmvm embed \
        --chart rocks-1.0.0.tgz \
        --values values.yaml \
        --output rocks
```
Now every time someone installs or upgrades a cluster using the `rocks` binary the Helm Chart will be installed with the custom values.

You can embed as many Helm Charts and `values.yaml` as you want:

```
$ helmvm embed \
        --chart rocks-1.0.0.tgz \
        --values values.yaml \
        --chart mongodb-13.16.1.tgz \
        --values mongo-values.yaml `
        --output rocks
```

## Miscellaneous

HelmVM stores its data under `$HOME/helmvm` directory, you may want to create a backup of the directory, specially the `$HOME/helmvm/etc` directory.  Inside the `$HOME/helmvm/etc` directory you will find the `k0sctl.yaml` and the `kubeconfig` files, the first is used when installing or upgrading a cluster and the latter is used when accessing the cluster with `kubectl` (a copy of `kubectl` is also kept under `$HOME/helmvm/bin` directory and you may want to include it into your PATH).

If you want to use an already existing `k0sctl.yaml` configuration during the `install` command you can do so by using the `--config` flag.

Inside `$HOME/helmvm/bin` you will find a copy of `k0sctl` binary used to bootstrap the cluster, you can use it to manage the cluster as well (e.g. `$HOME/helmvm/bin/k0sctl kubeconfig --config $HOME/helmvm/etc/k0sctl.yaml`).

## Experimental features

### Stop and Start nodes for maintenance

Once the cluster is deployed you can easily stop and start nodes using the following:

```
~: helmvm node list
~: helmvm node stop node-0
~: helmvm node start node-0
```
