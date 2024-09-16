# Get component image name
function image() {
  jq -r ".\"$1\".image" dev/metadata.json
}

# Get component dockerfile path
function dockerfile() {
  jq -r ".\"$1\".dockerfile" dev/metadata.json
}

# Get component dockercontext
function dockercontext() {
  jq -r ".\"$1\".dockercontext" dev/metadata.json
}

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

function ec_build_and_load() {
  # Build the image
  if docker images | grep -q "$(image $1)"; then
    echo "$(image $1) image already exists, skipping build..."
  else
    echo "Building $1..."
    docker build -t $(image $1) -f $(dockerfile $1) $(dockercontext $1)
  fi

  # Load the image into the embedded cluster
  if docker exec $(ec_node) k0s ctr images ls | grep -q "$(image $1)"; then
    echo "$(image $1) image already loaded in embedded cluster, skipping import..."
  else
    echo "Loading "$(image $1)" image into embedded cluster..."
    docker save "$(image $1)" | docker exec -i $(ec_node) k0s ctr images import -
  fi
}

function ec_up() {
  ec_exec k0s kubectl exec -it deployment/$(deployment $1) -n embedded-cluster -- bash
}
