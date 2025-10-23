# Embedded Cluster platform

Replicated Embedded Cluster allows you to distribute a Kubernetes cluster and your application together as a single appliance, making it easy for enterprise users to install, update, and manage the application and the cluster in tandem.
Embedded Cluster is based on the open source Kubernetes distribution K0s.
For more information, see the [K0s documentation](https://docs.k0sproject.io/stable/).

In Embedded Cluster, all components and functionalities are consolidated into a single binary, this binary facilitates a streamlined cluster installation process, removing the need for external dependencies (rpms, debs, etc).

Embedded Cluster includes by default the Kots Admin Console, the OpenEBS Storage provisioner, and Velero for backups and disaster recovery.
Additionally, it includes a Registry when deployed in air gap mode, and SeaweedFS for distributed object storage in high availability air gap mode.

## Development

### Requirements

- MacOS
- Docker Desktop with sufficient resources allocated:
    - Open Docker Desktop -> Settings -> Resources
    - Increase allocated resources as needed. Higher allocations will significantly enhance the development environment speed and responsiveness.
- Replicated CLI
- Helm CLI
- AWS CLI
- Dagger
- shasum
- jq
- oras
- crane
- Kubectl (for integration tests)
- Kind (for integration tests)

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

    **Note:** The development environment automatically mounts both data directories to support v2 and v3:
    - **v2 mode:** Uses `/var/lib/embedded-cluster/k0s`
    - **v3 mode:** Uses `/var/lib/{app-slug}/k0s` (determined from `REPLICATED_APP`)
    
    Both directories are mounted automatically, so the embedded cluster binary can use whichever one it needs without any manual configuration.

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
 ((____!___/) Type 'exit' (or Ctrl+D) to exit.
  \0\0\0\0\/
 ~~~~~~~~~~~
$ export KUBECONFIG="/var/lib/embedded-cluster/k0s/pki/admin.conf"
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

### Creating v3 releases

Embedded Cluster supports v3 releases which provide an enhanced manager UI experience for installations and upgrades. V3 releases are enabled by setting the `ENABLE_V3` environment variable.

#### Creating a v3 initial release

To create a v3 initial release, run the following command:
```bash
make initial-release ENABLE_V3=1
```

This creates a release using the manifests located in the `e2e/kots-release-install-v3` directory.

#### Creating a v3 upgrade release

To create a v3 upgrade release, run the following command:
```bash
make upgrade-release ENABLE_V3=1
```

This creates a release using the manifests located in the `e2e/kots-release-upgrade-v3` directory.

#### Installing a v3 release

When using v3 releases, the install and upgrade commands are different from the standard commands:

**Install command:**
```bash
ENABLE_V3=1 EC_DEV_ENV=true output/bin/embedded-cluster install --license <license-file> --target linux
```

**Upgrade command:**
```bash
ENABLE_V3=1 EC_DEV_ENV=true output/bin/embedded-cluster upgrade --license <license-file> --target linux
```

**Required environment variables:**
- `ENABLE_V3=1` - **Required** to enable v3 functionality and manager UI experience
- `EC_DEV_ENV=true` - **Optional** for development mode, enables dynamic asset loading from `./web/dist` instead of embedded assets. This allows you to test web UI changes by simply running `npm run build` in the `web/` directory and refreshing the browser, without needing to rebuild the entire embedded-cluster binary or building a new release.

**Required flags:**
- `--target` - **Required** to specify the target platform. Valid options are `linux` or `kubernetes`
- `--license` - **Required** path to the license file

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

### Building for Previous K0s Versions

To build for a previous K0s version, set the K0S_MINOR_VERSION environment variable and run the build command.

```bash
export K0S_MINOR_VERSION=31
make initial-release
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

    Subsequent changes to the kotsadm web component are reflected in real-time; no manual steps are required.
    However, to add, remove, or upgrade a dependency / package:

    * From a new terminal session, exec into the kotsadm-web container:
        ```bash
        make kotsadm-web-up-ec
        ```

    * Run the desired `yarn` commands. For example:
        ```bash
        yarn add <package>
        ```

    * When finished, exit the container:
        ```bash
        exit
        ```

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

### Using Local KOTS CLI for Development

When developing embedded cluster, you can use your local kots CLI binary instead of the default one. This is particularly useful when you need to test changes to the kots CLI itself.

#### Setting Up Local KOTS Binary

1. **Build your local kots binary** in the kots repository:
   ```bash
   make kots-linux-arm64
   ```

2. **Update `versions.mk`** to set the override:
   ```makefile
   KOTS_BINARY_FILE_OVERRIDE = ../kots/bin/kots
   ```

#### Alternative: Remote Binary Override

1. In the kots repository, build and upload your kots binary to ttl.sh by running:
   ```bash
   make kots-ttl.sh
   ```
   This will print a ttl.sh URL for the uploaded kots binary.

2. In `versions.mk` file, set the override variable to the printed URL:
   ```makefile
   KOTS_BINARY_URL_OVERRIDE = ttl.sh/<user>/kots.tar.gz:24h
   ```

#### How It Works

The build system checks for overrides in this order:
1. `KOTS_BINARY_URL_OVERRIDE` - Downloads from URL (supports HTTP/HTTPS and ttl.sh artifacts)
2. `KOTS_BINARY_FILE_OVERRIDE` - Uses local file directly
3. Default - Downloads from kotsadm Docker image

The system automatically generates a KOTS version string based on your override, ensuring proper versioning for development builds on rebuilds.

## Dependency Versions

The [versions.mk](versions.mk) file serves as the single source of truth for all external dependency versions.

### Automated Version Updates

Dependency versions are automatically kept up-to-date through the [.github/workflows/dependencies.yaml](.github/workflows/dependencies.yaml) GitHub Actions workflow, which will run on a schedule and create a pull request for version updates when appropriate.

## Upgrading K0s Minor Version

To upgrade the K0s minor version, the [.github/workflows/dependencies.yaml](.github/workflows/dependencies.yaml) workflow can be triggered manually with the target minor version as input.

> **Note:** For patch version updates within the same minor version (e.g., 1.33.4 to 1.33.5), the [automated dependency workflow](#automated-version-updates) handles this automatically.

## Releasing

Embedded Cluster maintains support for the current and two previous K0s minor versions, ensuring backward compatibility while supporting the latest features.
All supported versions are released simultaneously from the main branch using a structured tagging approach that combines the application version with the supported K0s version.

### Release Tagging Strategy

Releases follow the format: `{APP_VERSION}+k0s-{K0S_MINOR_VERSION}`

**Examples:**
- `2.10.0+k0s-1.33` - Application version 2.10.0 with K0s 1.33.x support
- `2.10.0+k0s-1.32` - Application version 2.10.0 with K0s 1.32.x support
- `2.10.0+k0s-1.31` - Application version 2.10.0 with K0s 1.31.x support

### Release Process

1. **Prepare the release commit** - Ensure all changes are committed and tested
2. **Create annotated tags** - Tag the same commit with all supported K0s minor versions using annotated tags with descriptive messages:
   ```bash
   # Tag for K0s 1.33.x support
   git tag -a 2.10.0+k0s-1.33 -m "Release 2.10.0+k0s-1.33"
   git push origin 2.10.0+k0s-1.33

   # Tag for K0s 1.32.x support
   git tag -a 2.10.0+k0s-1.32 -m "Release 2.10.0+k0s-1.32"
   git push origin 2.10.0+k0s-1.32

   # Tag for K0s 1.31.x support
   git tag -a 2.10.0+k0s-1.31 -m "Release 2.10.0+k0s-1.31"
   git push origin 2.10.0+k0s-1.31
   ```
