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
  install     Installs and starts the server as a systemd service
  kubectl     kubectl controls the Kubernetes cluster manager
  run         Runs the server
  start       Starts the systemd service
  stop        Stops the systemd service
  version     Prints version information

Flags:
  -d, --debug                Debug logging (default: false)
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
Runs the server

Usage:
  helmbin server [flags]

Aliases:
  server, controller

Flags:
      --data-dir string   Path to the data directory. (default "/var/lib/replicated")
  -h, --help              help for server

Global Flags:
  -d, --debug                Debug logging (default: false)
```

```bash
$ ./bin/helmbin install --help
Installs the server as a systemd service

Usage:
  helmbin install [flags]

Flags:
      --data-dir string   Path to the data directory. (default "/var/lib/replicated")
  -h, --help              help for install

Global Flags:
  -d, --debug                Debug logging (default: false)
```
