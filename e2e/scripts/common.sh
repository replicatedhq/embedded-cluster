wait_for_healthy_node() {
    ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
    counter=0
    while [ "$ready" -lt "1" ]; do
        if [ "$counter" -gt 36 ]; then
            return 1
        fi
        sleep 5
        counter=$((counter+1))
        echo "Waiting for node to be ready"
        ready=$(kubectl get nodes | grep -v NotReady | grep -c Ready || true)
        kubectl get nodes || true
    done
    return 0
}
