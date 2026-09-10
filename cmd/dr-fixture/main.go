package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/drfixture"
)

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "verify" {
		fmt.Fprintln(os.Stderr, "usage: dr-fixture verify --payload PATH --manifest PATH")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("verify", flag.ExitOnError)
	payload := flags.String("payload", "", "fixture payload path")
	manifest := flags.String("manifest", "", "fixture manifest path")
	var forbiddenEnv stringList
	flags.Var(&forbiddenEnv, "forbid-env", "environment variable whose exact value must not occur (repeatable)")
	_ = flags.Parse(os.Args[2:])
	if *payload == "" || *manifest == "" {
		flags.Usage()
		os.Exit(2)
	}
	verified, err := drfixture.Verify(*payload, *manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture verification failed: %v\n", err)
		os.Exit(1)
	}
	forbidden := make(map[string]string, len(forbiddenEnv))
	for _, name := range forbiddenEnv {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			fmt.Fprintf(os.Stderr, "fixture verification failed: forbidden environment variable %s is unset\n", name)
			os.Exit(1)
		}
		forbidden[name] = value
	}
	if err := drfixture.RejectValues(*payload, forbidden); err != nil {
		fmt.Fprintf(os.Stderr, "fixture verification failed: %v\n", err)
		os.Exit(1)
	}
	if err := drfixture.RejectValues(*manifest, forbidden); err != nil {
		fmt.Fprintf(os.Stderr, "fixture verification failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("verified %s (%s)\n", verified.Payload, verified.PayloadSHA256)
}
