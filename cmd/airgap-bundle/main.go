package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/airgapbundle"
	"github.com/replicatedhq/embedded-cluster/utils/pkg/embed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: airgap-bundle <application-identity|separate-config|extract-release|pack-directory|resolve-images|runtime-plan|build-runtime|complete-runtime|verify-runtime|augment|wrap-release>")
	}
	switch args[0] {
	case "application-identity":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		role := fs.String("role", "", "logical release role")
		roots := stringList{}
		fs.Var(&roots, "input", "input file or directory (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		identity, err := airgapbundle.ApplicationDigest(*role, roots...)
		return printJSON(identity, err)
	case "separate-config":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		source := fs.String("source", "", "release source directory")
		application := fs.String("application", "", "application-only output directory")
		config := fs.String("config", "", "EC Config output path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return airgapbundle.SeparateConfig(*source, *application, *config)
	case "extract-release":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		binary := fs.String("binary", "", "release-embedded installer")
		output := fs.String("output", "", "release archive output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		data, err := embed.ExtractReleaseDataFromBinary(*binary)
		if err != nil {
			return err
		}
		return os.WriteFile(*output, data, 0o644)
	case "pack-directory":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		dir := fs.String("dir", "", "directory to archive")
		output := fs.String("output", "", "tar.gz output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return airgapbundle.WriteDeterministicTarGz(*dir, *output)
	case "runtime-plan":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		mode := fs.String("network-mode", "", "online or airgap")
		platform := fs.String("platform", "linux/amd64", "target platform")
		imageArgs, chartArgs := stringList{}, stringList{}
		fs.Var(&imageArgs, "image", "reference,repository,digest")
		fs.Var(&chartArgs, "chart", "name,digest")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		images, err := parseImages(imageArgs)
		if err != nil {
			return err
		}
		charts, err := parseCharts(chartArgs)
		if err != nil {
			return err
		}
		plan, err := airgapbundle.NewRuntimePlan(*mode, *platform, images, charts)
		return printJSON(plan, err)
	case "resolve-images":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		refs := stringList{}
		fs.Var(&refs, "image", "image reference (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		resolved, err := airgapbundle.ResolveImages(context.Background(), refs)
		return printJSON(resolved, err)
	case "complete-runtime":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		planPath := fs.String("plan", "", "runtime plan JSON")
		images := fs.String("images", "", "images archive")
		charts := fs.String("charts", "", "charts archive")
		output := fs.String("output", "", "manifest output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plan, err := readManifest(*planPath)
		if err != nil {
			return err
		}
		if err := plan.Complete(*images, *charts); err != nil {
			return err
		}
		return plan.Write(*output)
	case "build-runtime":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		planPath := fs.String("plan", "", "runtime plan JSON")
		output := fs.String("output-dir", "", "runtime asset output directory")
		charts := stringList{}
		fs.Var(&charts, "chart", "packaged chart path (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plan, err := readManifest(*planPath)
		if err != nil {
			return err
		}
		if err := airgapbundle.BuildRuntimeAssets(context.Background(), &plan, charts, *output); err != nil {
			return err
		}
		return writeJSONFile(*planPath, plan)
	case "verify-runtime":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		dir := fs.String("dir", "", "runtime cache directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		m, err := airgapbundle.ReadRuntimeManifest(*dir + string(os.PathSeparator) + "manifest.json")
		if err != nil {
			return err
		}
		return m.Verify(*dir)
	case "augment":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		app := fs.String("application-bundle", "", "application bundle")
		ecDir := fs.String("ec-dir", "", "production-layout embedded-cluster directory")
		versionLabel := fs.String("version-label", "", "final application version label")
		output := fs.String("output", "", "output bundle")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return airgapbundle.Augment(*app, *ecDir, *versionLabel, *output)
	case "wrap-release":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		slug := fs.String("app-slug", "", "application slug")
		installer := fs.String("installer", "", "release-embedded installer")
		license := fs.String("license", "", "customer license")
		bundle := fs.String("airgap-bundle", "", "augmented .airgap bundle")
		output := fs.String("output", "", "outer tgz output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return airgapbundle.WrapRelease(*slug, *installer, *license, *bundle, *output)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
func parseImages(values []string) ([]airgapbundle.RequestedImage, error) {
	out := make([]airgapbundle.RequestedImage, 0, len(values))
	for _, value := range values {
		p := strings.Split(value, ",")
		if len(p) != 3 {
			return nil, fmt.Errorf("invalid image %q", value)
		}
		out = append(out, airgapbundle.RequestedImage{Reference: p[0], Repository: p[1], Digest: p[2]})
	}
	return out, nil
}
func parseCharts(values []string) ([]airgapbundle.Chart, error) {
	out := make([]airgapbundle.Chart, 0, len(values))
	for _, value := range values {
		p := strings.Split(value, ",")
		if len(p) != 2 {
			return nil, fmt.Errorf("invalid chart %q", value)
		}
		out = append(out, airgapbundle.Chart{Name: p[0], Digest: p[1]})
	}
	return out, nil
}
func printJSON(v any, err error) error {
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(v)
}
func readManifest(path string) (airgapbundle.RuntimeManifest, error) {
	var m airgapbundle.RuntimeManifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func writeJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
