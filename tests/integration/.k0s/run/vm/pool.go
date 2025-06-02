package vm

import (
	"log"

	"github.com/ory/dockertest/v3"
)

var (
	pool *dockertest.Pool
)

func init() {
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}
	err = pool.Client.Ping()
	if err != nil {
		log.Fatalf("Could not connect to Docker: %s", err)
	}
}
