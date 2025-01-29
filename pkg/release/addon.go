package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

const metadataPreface = `#
# this file is automatically generated by buildtools. manual edits are not recommended.
# to regenerate this file, run the following commands:
#
# $ make buildtools
# $ output/bin/buildtools update addon <addon name>
#
`
const valuesPreface = `#
# this file is automatically generated by the buildtools utility. Manual edits to this file
# are not recommended and may be overwritten. to regenerate this file, please execute the
# following commands:
#
# $ make buildtools
# $ output/bin/buildtools update addon <addon name>
#
# should you need to modify any configurations or settings within this file, please update
# the values.tpl.yaml file located in the same directory. after making the necessary changes,
# regenerate this file using the aforementioned commands to ensure all modifications are
# correctly applied.
#
`

type AddonMetadata struct {
	Version       string                `yaml:"version"`
	Location      string                `yaml:"location"`
	Images        map[string]AddonImage `yaml:"images"`
	ReplaceImages bool                  `yaml:"-"`
	GOARCH        string                `yaml:"-"`
}

type AddonImage struct {
	Repo string            `yaml:"repo"`
	Tag  map[string]string `yaml:"tag"`
}

func (i AddonImage) String() string {
	if strings.HasPrefix(i.Tag[runtime.GOARCH], "latest@") {
		// The image appears in containerd images without the "latest" tag and causes an
		// ImagePullBackOff error
		return fmt.Sprintf("%s@%s", i.Repo, strings.TrimPrefix(i.Tag[runtime.GOARCH], "latest@"))
	}
	return fmt.Sprintf("%s:%s", i.Repo, i.Tag[runtime.GOARCH])
}

var funcMap = template.FuncMap{
	"TrimPrefix": func(prefix, s string) string {
		return strings.TrimPrefix(s, prefix)
	},
	"ImageString": func(i AddonImage) string {
		return i.String()
	},
}

func RenderHelmValues(rawvalues []byte, meta AddonMetadata) (map[string]interface{}, error) {
	meta.ReplaceImages = true
	meta.GOARCH = runtime.GOARCH
	tmpl, err := template.New("helmvalues").Funcs(funcMap).Parse(string(rawvalues))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, meta)
	if err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	helmValues := make(map[string]interface{})
	if err := yaml.Unmarshal(buf.Bytes(), &helmValues); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return helmValues, nil
}

func GetValuesWithOriginalImages(addon string) (map[string]interface{}, error) {
	tplpath := filepath.Join("pkg", "addons", addon, "static", "values.tpl.yaml")
	tpl, err := os.ReadFile(tplpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read values template: %w", err)
	}
	tmpl, err := template.New(fmt.Sprintf("builder-%s", addon)).Funcs(funcMap).Parse(string(tpl))
	if err != nil {
		return nil, fmt.Errorf("failed to parse values template: %w", err)
	}
	buf := bytes.NewBufferString(valuesPreface)
	if err := tmpl.Execute(buf, AddonMetadata{}); err != nil {
		return nil, fmt.Errorf("failed to execute values template: %w", err)
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal values: %w", err)
	}
	return values, nil
}

func (a *AddonMetadata) Save(addon string) error {
	buf := bytes.NewBufferString(metadataPreface)
	if err := yaml.NewEncoder(buf).Encode(a); err != nil {
		return fmt.Errorf("failed to encode addon metadata: %w", err)
	}
	fpath := filepath.Join("pkg", "addons", addon, "static", "metadata.yaml")
	if err := os.WriteFile(fpath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write addon metadata: %w", err)
	}
	fpath = filepath.Join("pkg", "addons2", addon, "static", "metadata.yaml")
	if err := os.WriteFile(fpath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write addon metadata: %w", err)
	}
	return nil
}
