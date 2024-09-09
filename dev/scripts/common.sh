# Get component deployment name
function deployment() {
  jq -r ".\"$1\".deployment" dev/metadata.json
}

# The embedded-cluster container mounts the embedded cluster project at /replicatedhq/embedded-cluster
function ec_render() {
  sed "s|__PROJECT_DIR__|/replicatedhq/embedded-cluster|g" "$1"
}

# Get the embedded cluster node name
function ec_node() {
  echo "${EC_NODE:-node0}"
}

# Executes a command in the embedded cluster container
function ec_exec() {
  docker exec -it -w /replicatedhq/embedded-cluster $(ec_node) $@
}

# Patches a component deployment in the embedded cluster
function ec_patch() {
  ec_render dev/patches/$1-up.yaml > dev/patches/$1-up.yaml.tmp
  ec_exec k0s kubectl patch deployment $(deployment $1) -n embedded-cluster --patch-file dev/patches/$1-up.yaml.tmp
  rm dev/patches/$1-up.yaml.tmp
}

function ec_up() {
  ec_exec k0s kubectl exec -it deployment/$(deployment $1) -n embedded-cluster -- bash
}
