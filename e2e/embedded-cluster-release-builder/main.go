package main

import (
	"io"
	"log"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/embed"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) != 4 {
		log.Printf("Usage: %s <binary> <release.tar.gz> <output>", os.Args[0])
		return 1
	}
	builder, err := embed.NewElfBuilder("")
	if err != nil {
		log.Printf("failed to create builder: %v", err)
		return 1
	}
	defer builder.Close()
	src, err := builder.Build(os.Args[1], os.Args[2])
	if err != nil {
		log.Printf("failed to build: %v", err)
		return 1
	}
	defer src.Close()
	dst, err := os.OpenFile(os.Args[3], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Printf("failed to open destination: %v", err)
		return 1
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("failed to copy: %v", err)
		return 1
	}
	return 0
}
