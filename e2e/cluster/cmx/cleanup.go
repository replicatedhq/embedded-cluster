package cmx

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	globalSignalChan chan os.Signal
	globalClusters   []*Cluster
	globalMutex      sync.RWMutex
	globalSignalOnce sync.Once
)

func initGlobalSignalHandler() {
	globalSignalOnce.Do(func() {
		globalSignalChan = make(chan os.Signal, 1)
		signal.Notify(globalSignalChan, syscall.SIGINT, syscall.SIGTERM)
		go handleGlobalSignals()
	})
}

func handleGlobalSignals() {
	for sig := range globalSignalChan {
		if sig != nil {
			globalMutex.RLock()
			clusters := make([]*Cluster, len(globalClusters))
			copy(clusters, globalClusters)
			globalMutex.RUnlock()

			for _, cluster := range clusters {
				cluster.Destroy()
			}
			os.Exit(1)
		}
	}
}

func registerClusterForGlobalCleanup(cluster *Cluster) {
	initGlobalSignalHandler()
	globalMutex.Lock()
	defer globalMutex.Unlock()
	globalClusters = append(globalClusters, cluster)
}

func unregisterClusterForGlobalCleanup(cluster *Cluster) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	for i, c := range globalClusters {
		if c == cluster {
			globalClusters = append(globalClusters[:i], globalClusters[i+1:]...)
			break
		}
	}
}
