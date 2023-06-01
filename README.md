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
go build -gcflags "all=-trimpath=/home/ethan/go/src/github.com/emosbaugh" -asmflags "all=-trimpath=/home/ethan/go/src/github.com/emosbaugh" -ldflags " -X main.goos=linux -X main.goarch=amd64 -X main.gitCommit=1a6e487bb4bcb5049c448983758912afbdb9d1c2 -X main.buildDate=2023-05-24T21:42:34Z " -tags='' -o bin/helmbin ./cmd/helmbin
```

## Running

```bash
./bin/helmbin server  --help
Runs a controller+worker node

Usage:
  helmbin run [flags]
  helmbin run [command]

Available Commands:
  controller  Runs a controller node
  worker      Runs a worker node

Flags:
  -c, --config string       k0s config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --data-dir string     Data Directory. DO NOT CHANGE for an existing setup, things will break! (default "/var/lib/replicated")
  -d, --debug               Debug logging (default: false)
      --enable-worker       enable worker (default true)
  -h, --help                help for run
      --no-taints           disable default taints for controller node (default true)
      --token-file string   Path to the file containing join-token.

Use "helmbin run [command] --help" for more information about a command.
```

```bash
$ ./bin/helmbin install --help
Installs and starts a controller+worker as a systemd service

Usage:
  helmbin install [flags]
  helmbin install [command]

Available Commands:
  controller  Installs and starts a controller as a systemd service
  controller  Installs and starts a worker as a systemd service

Flags:
  -c, --config string       k0s config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --data-dir string     Data Directory. DO NOT CHANGE for an existing setup, things will break! (default "/var/lib/replicated")
  -d, --debug               Debug logging (default: false)
      --enable-worker       enable worker (default true)
  -h, --help                help for install
      --no-taints           disable default taints for controller node (default true)
      --start               Start the service after installation (default true)
      --token-file string   Path to the file containing join-token.

Use "helmbin install [command] --help" for more information about a command.
```
