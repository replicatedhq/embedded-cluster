package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestLocalSecureBuildRecipes(t *testing.T) {
	for _, family := range []string{"embedded-cluster-operator", "local-artifact-mirror"} {
		for _, minor := range []int{33, 34, 35, 36} {
			name := fmt.Sprintf("%s-k8s1.%d", family, minor)
			t.Run(name, func(t *testing.T) {
				version := fmt.Sprintf("v2.19.10+k8s-1.%d-rc1", minor)
				packageData, err := os.ReadFile("../securebuild/package/" + name + "/melange.yaml")
				if err != nil {
					t.Fatal(err)
				}
				localPackage, err := adaptLocalPackage(string(packageData), version)
				if err != nil {
					t.Fatal(err)
				}
				var recipe struct {
					Package struct {
						Name    string
						Version string
						Epoch   int
					}
					Environment struct {
						Environment map[string]string
					}
					Pipeline []struct {
						Uses string
						Runs string
					}
				}
				if err := yaml.Unmarshal([]byte(localPackage), &recipe); err != nil {
					t.Fatal(err)
				}
				if recipe.Package.Name != name+"-head" || recipe.Package.Version != "1000.0.0" || recipe.Package.Epoch != 0 {
					t.Fatalf("local image pins must match the recipe's head package: %+v", recipe.Package)
				}
				if recipe.Environment.Environment["EC_VERSION"] != version || recipe.Environment.Environment["VERSION"] != version {
					t.Fatal("local build lost the requested binary version")
				}
				var runs string
				for _, step := range recipe.Pipeline {
					if step.Uses == "git-checkout" {
						t.Fatal("local build would replace the working source with a remote checkout")
					}
					runs += step.Runs
				}
				if !strings.Contains(runs, "build-deps build") {
					t.Fatal("local build lost the recipe's build step")
				}
				if family == "embedded-cluster-operator" && !strings.Contains(runs, `VERSION="${EC_VERSION}"`) {
					t.Fatal("operator would embed the sentinel APK version instead of the requested version")
				}

				imageData, err := os.ReadFile("../securebuild/image/apko-" + name + ".yaml")
				if err != nil {
					t.Fatal(err)
				}
				localImage, err := adaptLocalImage(string(imageData), name)
				if err != nil {
					t.Fatal(err)
				}
				var image struct {
					Contents struct {
						Packages     []string
						Repositories []string
						Keyring      []string
					}
				}
				if err := yaml.Unmarshal([]byte(localImage), &image); err != nil {
					t.Fatal(err)
				}
				if image.Contents.Repositories[0] != "./packages" || image.Contents.Keyring[0] != "./melange.rsa.pub" {
					t.Fatal("local package repository or signing key is missing")
				}
				for _, pkg := range image.Contents.Packages {
					if strings.HasPrefix(pkg, name) && !strings.Contains(pkg, "-head") {
						t.Fatalf("image could select a released package instead of the local build: %s", pkg)
					}
				}
				if family == "local-artifact-mirror" && minor >= 34 && !strings.Contains(localImage, name+"-head-compat=1000.0.0-r0") {
					t.Fatal("local compat package is not pinned")
				}
			})
		}
	}
}
