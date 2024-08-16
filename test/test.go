// You can edit this code!
// Click here and start typing.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
	"github.com/pkg/errors"
)

// parseImageReferences parses the docker image references from strings to references that can be
// later on used during pulling. If the image reference contains both tags and digests this func
// will parse only the digest portion.
func parseImageReferences(images []string) ([]types.ImageReference, error) {
	references := []types.ImageReference{}
	for _, rawref := range images {
		ref, err := reference.Parse(rawref)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse syntactically valid reference %s", rawref)
		}

		_, hastag := ref.(reference.Tagged)
		_, hassha := ref.(reference.Digested)
		if hastag && hassha {
			prefix := strings.Split(rawref, ":")[0]
			suffix := strings.Split(rawref, "@")[1]
			rawref = fmt.Sprintf("%s@%s", prefix, suffix)
		}

		withproto := fmt.Sprintf("docker://%s", rawref)
		parsed, err := alltransports.ParseImageName(withproto)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse image reference %s", rawref)
		}
		references = append(references, parsed)
	}
	return references, nil
}

func createECImagesTar(images []string) error {
	references, err := parseImageReferences(images)
	if err != nil {
		return errors.Wrap(err, "failed to parse image references")
	}

	tmpdir, err := os.MkdirTemp("", "airgap-ec-images-*")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tmpdir)

	policy, err := signature.DefaultPolicy(&types.SystemContext{})
	if err != nil {
		return errors.Wrap(err, "failed to create default image policy")
	}
	policyctx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return errors.Wrap(err, "failed to create policy")
	}

	for _, src := range references {
		fullname := src.DockerReference().String()
		dstpath := fmt.Sprintf("%s:%s", tmpdir, fullname)
		dst, err := layout.ParseReference(dstpath)
		if err != nil {
			return errors.Wrap(err, "failed to parse dst oci reference")
		}

		srcctx := &types.SystemContext{}

		ctx := context.Background()
		opts := &copy.Options{PreserveDigests: true, SourceCtx: srcctx}
		if _, err := copy.Image(ctx, policyctx, dst, src, opts); err != nil {
			return errors.Wrapf(err, "failed to copy image %s", fullname)
		}
	}

	// list images in tmp dir
	files, err := ioutil.ReadDir(tmpdir)
	if err != nil {
		return errors.Wrap(err, "failed to open images tar")
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fmt.Printf("file: %s\n", file.Name())
	}

	return nil
}

func main() {
	images := []string{
		"proxy.replicated.com/anonymous/registry.k8s.io/kube-proxy:2.8.3-r0@sha256:5b76ebd0a362009e31a05ac487c690f5ece0e11f6c4d9261ca63a3f162b57660",
		"registry.k8s.io/pause:3.9",
	}
	if err := createECImagesTar(images); err != nil {
		panic(err)
	}
}

