# Embedded Cluster platform

Replicated Embedded Cluster allows you to distribute a Kubernetes cluster and your application together as a single appliance, making it easy for enterprise users to install, update, and manage the application and the cluster in tandem.
Embedded Cluster is based on the open source Kubernetes distribution k0s.
For more information, see the [k0s documentation](https://docs.k0sproject.io/stable/).

In Embedded Cluster, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc).

Embedded Cluster includes by default the Kots Admin Console, the OpenEBS Storage provisioner, and Velero for backups and disaster recovery.
Additionally, it includes a Registry when deployed in air gap mode.

## Building and running

With the repository checked out locally, to compile you just need to run:

```bash
$ make build-ttl.sh
```

This will build the embedded-cluster binary and push the local-artifact-mirror image to ttl.sh.

The binary will be located on `output/bin/embedded-cluster`.

## Single node deployment

To create a single node deployment you can upload the Embedded Cluster binary to a Linux x86_64 machine and run:

```bash
ubuntu@ip-172-16-10-242:~$ ./embedded-cluster install
```

## Interacting with the cluster

Once the cluster has been deployed you can open a new terminal to interact with it using `kubectl`:

```bash
ubuntu@ip-172-16-10-242:~$ ./embedded-cluster shell

    __4___
 _  \ \ \ \   Welcome to embedded-cluster-smoke-test-staging-app debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or CTRL+d) to exit.
  \0\0\0\0\/  Happy hacking.
 ~~~~~~~~~~~
ubuntu@ip-172-16-10-242:~$ export KUBECONFIG="/var/lib/k0s/pki/admin.conf"
ubuntu@ip-172-16-10-242:~$ export PATH="$PATH:/var/lib/embedded-cluster/bin"
ubuntu@ip-172-16-10-242:~$ source <(kubectl completion bash)
ubuntu@ip-172-16-10-242:~$ source /etc/bash_completion
ubuntu@ip-172-16-10-242:~$ 
```

This will drop you in a new shell, this shell is configured to reach the cluster and includes shell completion:
