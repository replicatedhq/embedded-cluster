package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestOperatorRecipeVersion(t *testing.T) {
	for _, minor := range []int{33, 34, 35, 36} {
		data, err := os.ReadFile(fmt.Sprintf("../securebuild/package/embedded-cluster-operator-k8s1.%d/melange.yaml", minor))
		if err != nil {
			t.Fatal(err)
		}
		var recipe struct {
			Pipeline []struct {
				Name string
				Runs string
			}
		}
		if err := yaml.Unmarshal(data, &recipe); err != nil {
			t.Fatal(err)
		}
		var script string
		for _, step := range recipe.Pipeline {
			if step.Name == "build-go-binary" {
				script = step.Runs
			}
		}
		if script == "" {
			t.Fatal("operator build step missing")
		}
		// Simulate Melange's substitutions, then execute the recipe's shell with
		// make and cp stubbed so we can check the version passed to the build.
		script = strings.NewReplacer(
			"${{package.version}}", "2.19.9",
			"${{vars.k8s-version}}", fmt.Sprintf("k8s-1.%d", minor),
			"${{targets.destdir}}", "/unused",
		).Replace(script)
		for _, local := range []bool{false, true} {
			t.Run(fmt.Sprintf("k8s1.%d/local=%t", minor, local), func(t *testing.T) {
				version := fmt.Sprintf("2.19.9+k8s-1.%d", minor)
				setup := "unset VERSION\n"
				if local {
					version = fmt.Sprintf("v2.19.10+k8s-1.%d-rc1", minor)
					setup = fmt.Sprintf("export VERSION=%q\n", version)
				}
				cmd := exec.Command("bash", "-euc", setup+"make() { printf '%s\\n' \"$@\"; }; cp() { :; };\n"+script)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("recipe shell failed: %v\n%s", err, out)
				}
				if !strings.Contains(string(out), "VERSION="+version+"\n") {
					t.Fatalf("expected binary version %q, got %s", version, out)
				}
			})
		}
	}
}
