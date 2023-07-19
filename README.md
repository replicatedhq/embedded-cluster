# helmbin

Embeds Kubernetes and Helm charts as a single installable binary.

```bash
$ ./bin/helmbin 
An embeddable Kubernetes distribution

Usage:
  helmbin [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  install     Installs and starts a controller+worker as a systemd service
  kubectl     kubectl controls the Kubernetes cluster manager
  run         Runs a controller+worker node
  start       Starts the systemd service
  stop        Stops the systemd service
  version     Prints version information

Flags:
  -d, --debug                Enables debug logging
  -h, --help                 help for helmbin

Use "helmbin [command] --help" for more information about a command.
```

## Building

```bash
$ make build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags osusergo -asmflags "all=-trimpath=/Users/ethan/go/src/github.com/replicatedhq" -gcflags "all=-trimpath=/Users/ethan/go/src/github.com/replicatedhq" -ldflags='-X main.goos=linux -X main.goarch=amd64 -X main.gitCommit=5196840140ccbb3fdf9394eec7d8bea9169aae84 -X main.buildDate=2023-07-12T19:33:16Z -extldflags=-static' -o bin/helmbin ./cmd/helmbin
```

## Running

```bash
./bin/helmbin run  --help
Runs a controller+worker node

Usage:
  helmbin run [flags]
  helmbin run [command]

Available Commands:
  controller  Runs a controller node
  worker      Runs a worker node

Flags:
  -c, --config string            k0s config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --data-dir string          Data Directory. DO NOT CHANGE for an existing setup, things will break! (default "/var/lib/replicated")
  -d, --debug                    Debug logging (default: false)
      --enable-worker            enable worker (default true)
  -h, --help                     help for run
  -l, --logging stringToString   Logging Levels for the different components (default [kube-proxy=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1])
      --no-taints                disable default taints for controller node (default true)
      --token-file string        Path to the file containing join-token.

Use "helmbin run [command] --help" for more information about a command.
```

```bash
Install helmbin on a brand-new system. Must be run as root (or with sudo)

Usage:
  helmbin install [flags]
  helmbin install [command]

Examples:
With the install command you can setup a single node cluster by running:

	helmbin install


Available Commands:
  controller  Install helmbin controller on a brand-new system. Must be run as root (or with sudo)
  worker      Install helmbin worker on a brand-new system. Must be run as root (or with sudo)

Flags:
      --api-server string                              HACK: api-server for the windows worker node
      --cidr-range string                              HACK: cidr range for the windows worker node (default "10.96.0.0/12")
      --cluster-dns string                             HACK: cluster dns for the windows worker node (default "10.96.0.10")
  -c, --config string                                  config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --cri-socket string                              container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --data-dir string                                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                                          Debug logging (default: false)
      --debugListenOn string                           Http listenOn for Debug pprof handler (default ":6060")
      --disable-components strings                     disable components (valid items: autopilot,control-api,coredns,csr-approver,endpoint-reconciler,helm,konnectivity-server,kube-controller-manager,kube-proxy,kube-scheduler,metrics-server,network-provider,node-role,system-rbac,worker-config)
      --enable-cloud-provider                          Whether or not to enable cloud provider support in kubelet
      --enable-dynamic-config                          enable cluster-wide dynamic config based on custom resource
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --enable-metrics-scraper                         enable scraping metrics from the controller components (kube-scheduler, kube-controller-manager)
  -e, --env stringArray                                set environment variable
      --force                                          force init script creation
  -h, --help                                           help for install
      --iptables-mode string                           iptables mode (valid values: nft, legacy, auto). default: auto
      --k0s-cloud-provider-port int                    the port that k0s-cloud-provider binds on (default 10258)
      --k0s-cloud-provider-update-frequency duration   the frequency of k0s-cloud-provider node updates (default 2m0s)
      --kube-controller-manager-extra-args string      extra args for kube-controller-manager
      --kubelet-extra-args string                      extra args for kubelet
      --labels strings                                 Node labels, list of key=value pairs
  -l, --logging stringToString                         Logging Levels for the different components (default [etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1])
      --profile string                                 worker profile to use on the node (default "default")
      --status-socket string                           Full file path to the socket file. (default "status.sock")
      --taints strings                                 Node taints, list of key=value:effect strings
      --token-file string                              Path to the file containing join-token.
  -v, --verbose                                        Verbose logging (default: false)

Use "helmbin install [command] --help" for more information about a command.
```
