package main

import (
	"log"
	"os"

	"github.com/replicatedhq/embedded-cluster/utils/pkg/embed"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) != 4 {
		log.Printf("Usage: %s <binary> <release.tar.gz> <output>", os.Args[0])
		return 1
	}
	if err := embed.EmbedReleaseDataInBinary(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		log.Printf("failed to embed release data: %v", err)
		return 1
	}
	return 0
}
