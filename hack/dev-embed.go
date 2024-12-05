package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/replicatedhq/embedded-cluster/utils/pkg/embed"
)

func main() {
	os.Exit(run())
}

func run() int {
	binaryPath := flag.String("binary", "", "Path to the binary file")
	releasePath := flag.String("release", "", "Path to the release tar.gz file")
	outputPath := flag.String("output", "", "Path to the output file")
	label := flag.String("label", "", "Release label")
	sequence := flag.Int("sequence", 0, "Release sequence number")
	channel := flag.String("channel", "", "Channel slug")

	flag.Parse()

	if *binaryPath == "" || *releasePath == "" || *outputPath == "" {
		fmt.Printf("Usage: %s --binary <binary> --release <release.tar.gz> --output <output> [--label <label>] [--sequence <sequence>] [--channel <channel>]\n", os.Args[0])
		return 1
	}

	log.Printf("Embedding release with label=%q, sequence=%d, channel=%q", *label, *sequence, *channel)

	if err := embed.EmbedReleaseDataInBinary(*binaryPath, *releasePath, *outputPath); err != nil {
		log.Printf("Failed to embed release data: %v", err)
		return 1
	}

	return 0
}
