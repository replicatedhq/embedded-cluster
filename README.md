# Embedded Cluster platform

Replicated Embedded Cluster allows you to distribute a Kubernetes cluster and your application together as a single appliance, making it easy for enterprise users to install, update, and manage the application and the cluster in tandem.
Embedded Cluster is based on the open source Kubernetes distribution k0s.
For more information, see the [k0s documentation](https://docs.k0sproject.io/stable/).

In Embedded Cluster, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc).

Embedded Cluster includes by default the Kots Admin Console, the OpenEBS Storage provisioner, and Velero for backups and disaster recovery.
Additionally, it includes a Registry when deployed in air gap mode.

## Development

### Requirements

- MacOS
- Docker Desktop with sufficient resources allocated:
    - Open Docker Desktop -> Settings -> Resources
    - Increase allocated resources as needed. Higher allocations will significantly enhance the development environment speed and responsiveness.
- Replicated CLI
- Helm CLI
- AWS CLI
- jq

### Running the Development Environment

1. Clone the Embedded Cluster repo:
    ```bash
    git clone https://github.com/replicatedhq/embedded-cluster.git
    cd embedded-cluster
    ```

1. Set the following environment variables:
    ```bash
    export REPLICATED_APP=
    export REPLICATED_API_TOKEN=
    export REPLICATED_API_ORIGIN=
    export APP_CHANNEL=
    export APP_CHANNEL_ID=
    export APP_CHANNEL_SLUG=
    export AWS_ACCESS_KEY_ID=
    export AWS_SECRET_ACCESS_KEY=
    ```

    | Environment Variable | Description | Default Value |
    |----------------------|-------------|---------------|
    | `REPLICATED_APP` | The application slug | `embedded-cluster-smoke-test-staging-app` |
    | `REPLICATED_API_TOKEN` | A vendor portal API token with write access to the application | (required) |
    | `REPLICATED_API_ORIGIN` | The vendor-api URL | `https://api.staging.replicated.com/vendor` |
    | `APP_CHANNEL` | The channel name (it's recommended to create a new channel just for your development environment.) | (required) |
    | `APP_CHANNEL_ID` | The channel ID | (required) |
    | `APP_CHANNEL_SLUG` | The channel slug | (required) |
    | `AWS_ACCESS_KEY_ID` | AWS access key ID with write access to the `dev-embedded-cluster-bin` bucket in Replicated's dev AWS account | (required) |
    | `AWS_SECRET_ACCESS_KEY` | AWS secret access key with write access to the `dev-embedded-cluster-bin` bucket in Replicated's dev AWS account | (required) |

    Note: 
    - To use a different AWS bucket or account, override using the `S3_BUCKET` environment variable.
    - To use the Replicated staging bucket used in CI, set `USES_DEV_BUCKET=0`.

1. Create a release for initial installation:
    ```bash
    make initial-release
    ```

    This step creates a release that is intended to be used for initial installation, not for upgrades.
    It creates a release of the application using the manifests located in the `e2e/kots-release-install` directory.
    It may take a few minutes to complete the first time as nothing is cached yet.

1. Create the first node:
    ```bash
    make create-node0
    ```

    This command sets up the initial node for your cluster and SSHs into it.

    By default, a Debian-based node will be created. If you want to use a different distribution, you can set the `DISTRO` environment variable:

    ```bash
    make create-node0 DISTRO=almalinux-8
    ```

    To view the list of available distributions:

    ```bash
    make list-distros
    ```

1. In the Vendor Portal, create and download a license that is assigned to the channel.
We recommend storing this license in the `local-dev/` directory, as it is gitignored and not otherwise used by the CI.

1. Install Embedded Cluster:
    ```bash
    output/bin/embedded-cluster install --license <license-file>
    ```

1. Once that completes, you can access the admin console at http://localhost:30000

### Interacting with the cluster

You can interact with the cluster using `kubectl` by running the following command:

```bash
output/bin/embedded-cluster shell
```

This will drop you in a new shell, this shell is configured to reach the cluster and includes shell completion. Example output:

```bash
$ output/bin/embedded-cluster shell

    __4___
 _  \ \ \ \   Welcome to embedded-cluster-smoke-test-staging-app debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or CTRL+d) to exit.
  \0\0\0\0\/  Happy hacking.
 ~~~~~~~~~~~
$ export KUBECONFIG="/var/lib/k0s/pki/admin.conf"
$ export PATH="$PATH:/var/lib/embedded-cluster/bin"
$ source <(kubectl completion bash)
$ source /etc/bash_completion
$ 
```

### Creating an upgrade release

To create an upgrade release, run the following command:
```bash
make upgrade-release
```

This step creates a release that is intended to be used for upgrades.
It creates a release of the application using the manifests located in the `e2e/kots-release-upgrade` directory.
The release will show up in the KOTS admin console as an available update.

### Creating additional nodes

To create additional nodes, run the following command:
```bash
make create-node<node-number>
```

For example:
```bash
make create-node1
```

These additional nodes can either be joined to your existing Embedded Cluster installation, or used to set up separate, independent Embedded Cluster instances.

### Deleting nodes

If the node is part of a multi-node Embedded Cluster installation, it's recommended to remove it from the cluster first by running the following commands:
```bash
make ssh-node<node-number>
```
```bash
output/bin/embedded-cluster reset
```

To delete a node, run the following command:
```bash
make delete-node<node-number>
```

For example:
```bash
make delete-node1
```

### Establishing SSH sessions

To SSH into an existing node, run the following command:
```bash
make ssh-node<node-number>
```

For example:
```bash
make ssh-node0
```

### Developing Embedded Cluster Operator

1. To apply your current changes, run the following commands:
    ```bash
    make operator-up
    ```
    ```bash
    make build run
    ```

1. To apply additional changes, stop the current process with Ctrl+C, then run the following command:
    ```bash
    make build run
    ```

1. When finished developing, run the following commands to revert back to the original state:
    ```bash
    exit
    ```
    ```bash
    make operator-down
    ```

### Developing KOTS components

1. Clone the KOTS repo to the same parent directory as Embedded Cluster:
    ```bash
    cd ..
    git clone https://github.com/replicatedhq/kots.git
    cd kots
    ```

1. When developing KOTS components, the connection is made to `node0` by default. To connect to a KOTS instance on a different node, set the `EC_NODE` environment variable to the name of that node:
    ```bash
    export EC_NODE=node1
    ```

#### Developing kotsadm web / API

1. To apply your current changes, run the following commands:
    ```bash
    make kotsadm-up-ec
    ```
    ```bash
    make build run
    ```

    Subsequent changes to the kotsadm web component are reflected in real-time, no manual steps required.

1. To apply additional API changes, stop the current process with Ctrl+C, then run the following command:
    ```bash
    make build run
    ```

1. When finished developing, run the following commands to revert back to the original state:
    ```bash
    exit
    ```
    ```bash
    make kotsadm-down-ec
    ```

#### Developing kurl-proxy web / API

1. To apply your current changes, run the following commands:
    ```bash
    make kurl-proxy-up-ec
    ```
    ```bash
    make build run
    ```

1. To apply additional changes, stop the current process with Ctrl+C, then run the following command:
    ```bash
    make build run
    ```

1. When finished developing, run the following commands to revert back to the original state:
    ```bash
    exit
    ```
    ```bash
    make kurl-proxy-down-ec
    ```
