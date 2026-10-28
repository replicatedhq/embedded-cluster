package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/drfixture"
)

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "build":
		err = runBuild(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "dr-fixture %s failed: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dr-fixture <build|verify> [flags]")
}

func runVerify(args []string) error {
	flags := flag.NewFlagSet("verify", flag.ExitOnError)
	payload := flags.String("payload", "", "fixture payload path")
	manifest := flags.String("manifest", "", "fixture manifest path")
	var forbiddenEnv stringList
	flags.Var(&forbiddenEnv, "forbid-env", "environment variable whose exact value must not occur (repeatable)")
	_ = flags.Parse(args)
	if *payload == "" || *manifest == "" {
		return fmt.Errorf("--payload and --manifest are required")
	}
	verified, err := drfixture.Verify(*payload, *manifest)
	if err != nil {
		return err
	}
	forbidden := make(map[string]string, len(forbiddenEnv))
	for _, name := range forbiddenEnv {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			return fmt.Errorf("forbidden environment variable %s is unset", name)
		}
		forbidden[name] = value
	}
	if err := drfixture.RejectValues(*payload, forbidden); err != nil {
		return err
	}
	if err := drfixture.RejectValues(*manifest, forbidden); err != nil {
		return err
	}
	fmt.Printf("verified %s (%s)\n", verified.Payload, verified.PayloadSHA256)
	return nil
}

func runBuild(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	payload := flags.String("payload", "", "exported fixture payload")
	output := flags.String("manifest", "", "output manifest path")
	ecVersion := flags.String("ec-version", "", "released EC version")
	ecCommit := flags.String("ec-commit", "", "released EC source commit")
	k0sVersion := flags.String("k0s-version", "", "k0s version")
	kotsCommit := flags.String("kots-commit", "", "KOTS source commit")
	veleroVersion := flags.String("velero-version", "", "Velero version")
	application := flags.String("application-version", "", "application version")
	bundleDigest := flags.String("bundle-sha256", "", "bundle digest")
	region := flags.String("s3-region", "", "fixture S3 region")
	bucket := flags.String("s3-bucket", "", "fixture S3 bucket")
	prefix := flags.String("s3-prefix", "", "fixture S3 prefix")
	accessKey := flags.String("s3-access-key", "", "fixture S3 access key")
	secretKey := flags.String("s3-secret-key", "", "fixture S3 secret key")
	generation := flags.String("generation-command", "", "reproducible generation command")
	var kotsDigests stringList
	flags.Var(&kotsDigests, "kots-artifact-digest", "KOTS artifact digest (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *payload == "" || *output == "" {
		return fmt.Errorf("--payload and --manifest are required")
	}
	built, err := drfixture.Build(*payload, drfixture.Manifest{
		CreatedAt: time.Now().UTC().Format(time.RFC3339), ECVersion: *ecVersion, ECCommit: *ecCommit,
		K0sVersion: *k0sVersion, KOTSCommit: *kotsCommit, VeleroVersion: *veleroVersion,
		Application: *application, BundleSHA256: *bundleDigest, S3Region: *region, S3Bucket: *bucket,
		S3Prefix: *prefix, S3AccessKey: *accessKey, S3SecretKey: *secretKey,
		Generation: *generation, KOTSDigests: []string(kotsDigests),
	})
	if err != nil {
		return err
	}
	return drfixture.WriteManifest(*output, *built)
}
