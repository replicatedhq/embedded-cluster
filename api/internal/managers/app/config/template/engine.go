package template

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"maps"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type Engine struct {
	config       *kotsv1beta1.Config
	configValues types.AppConfigValues
	license      *kotsv1beta1.License
	releaseData  *release.ReleaseData

	// Simple cycle detection and result caching
	visited  map[string]bool
	resolved map[string]string
	mtx      sync.Mutex
}

type EngineOption func(*Engine)

func WithLicense(license *kotsv1beta1.License) EngineOption {
	return func(e *Engine) {
		e.license = license
	}
}

func WithReleaseData(releaseData *release.ReleaseData) EngineOption {
	return func(e *Engine) {
		e.releaseData = releaseData
	}
}

func NewEngine(config *kotsv1beta1.Config, opts ...EngineOption) *Engine {
	engine := &Engine{
		config:   config,
		visited:  make(map[string]bool),
		resolved: make(map[string]string),
		mtx:      sync.Mutex{},
	}

	for _, opt := range opts {
		opt(engine)
	}

	return engine
}

// Parse parses a template string and returns a prepared template
func (e *Engine) Parse(templateStr string) (*template.Template, error) {
	if templateStr == "" {
		return nil, nil
	}

	// Combine sprig functions with our custom functions
	funcMap := sprig.TxtFuncMap()
	maps.Copy(funcMap, e.getFuncMap())

	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// Execute executes a parsed template
func (e *Engine) Execute(tmpl *template.Template, configValues types.AppConfigValues) (string, error) {
	if tmpl == nil {
		return "", nil
	}

	e.mtx.Lock()
	defer e.mtx.Unlock()

	// Reset execution state
	e.visited = make(map[string]bool)
	e.resolved = make(map[string]string)
	e.configValues = configValues

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ProcessTemplate processes any template string (convenience method)
func (e *Engine) ProcessTemplate(templateStr string, configValues types.AppConfigValues) (string, error) {
	tmpl, err := e.Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	result, err := e.Execute(tmpl, configValues)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return result, nil
}

// processTemplate processes a template string using the current execution state
func (e *Engine) processTemplate(templateStr string) (string, error) {
	tmpl, err := e.Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func (e *Engine) getFuncMap() template.FuncMap {
	return template.FuncMap{
		"ConfigOption":       e.configOption,
		"ConfigOptionEquals": e.configOptionEquals,
		"ConfigOptionData":   e.configOptionData,
		"LicenseFieldValue":  e.licenseFieldValue,
	}
}

func (e *Engine) configOption(name string) (string, error) {
	val, err := e.ResolveConfigItem(name)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}
	return val, nil
}

func (e *Engine) configOptionEquals(name, expected string) (bool, error) {
	val, err := e.ResolveConfigItem(name)
	if err != nil {
		return false, fmt.Errorf("resolve config item: %w", err)
	}
	return val == expected, nil
}

func (e *Engine) configOptionData(name string) (string, error) {
	val, err := e.ResolveConfigItem(name)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}

	// Base64 decode for file content
	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return "", fmt.Errorf("decode base64 value: %w", err)
	}
	return string(decoded), nil
}

func (e *Engine) licenseFieldValue(name string) string {
	if e.license == nil {
		return ""
	}

	// Update docs at https://github.com/replicatedhq/kots.io/blob/main/content/reference/template-functions/license-context.md
	// when adding new values
	switch name {
	case "isSnapshotSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSnapshotSupported)
	case "IsDisasterRecoverySupported":
		return fmt.Sprintf("%t", e.license.Spec.IsDisasterRecoverySupported)
	case "isGitOpsSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGitOpsSupported)
	case "isSupportBundleUploadSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsSupportBundleUploadSupported)
	case "isEmbeddedClusterMultiNodeEnabled":
		return fmt.Sprintf("%t", e.license.Spec.IsEmbeddedClusterMultiNodeEnabled)
	case "isIdentityServiceSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsIdentityServiceSupported)
	case "isGeoaxisSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsGeoaxisSupported)
	case "isAirgapSupported":
		return fmt.Sprintf("%t", e.license.Spec.IsAirgapSupported)
	case "licenseType":
		return e.license.Spec.LicenseType
	case "licenseSequence":
		return fmt.Sprintf("%d", e.license.Spec.LicenseSequence)
	case "signature":
		return string(e.license.Spec.Signature)
	case "appSlug":
		return e.license.Spec.AppSlug
	case "channelID":
		return e.license.Spec.ChannelID
	case "channelName":
		return e.license.Spec.ChannelName
	case "isSemverRequired":
		return fmt.Sprintf("%t", e.license.Spec.IsSemverRequired)
	case "customerName":
		return e.license.Spec.CustomerName
	case "licenseID", "licenseId":
		return e.license.Spec.LicenseID
	case "endpoint":
		if e.releaseData == nil {
			return ""
		}
		ecDomains := utils.GetDomains(e.releaseData)
		return netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain)
	default:
		entitlement, ok := e.license.Spec.Entitlements[name]
		if ok {
			return fmt.Sprintf("%v", entitlement.Value.Value())
		}
		return ""
	}
}

// ResolveConfigItem resolves a specific config item (internal recursive method)
func (e *Engine) ResolveConfigItem(name string) (string, error) {
	// Check if already resolved
	if value, exists := e.resolved[name]; exists {
		return value, nil
	}

	// Check for circular dependency
	if e.visited[name] {
		return "", fmt.Errorf("circular dependency detected for %s", name)
	}

	// Mark as visited for cycle detection
	e.visited[name] = true

	// Find the config item definition
	configItem := e.findConfigItem(name)
	if configItem == nil {
		return "", fmt.Errorf("config item %s not found", name)
	}

	var effectiveValue string

	// Priority: user value > config value > config default
	if userVal, exists := e.configValues[name]; exists {
		effectiveValue = userVal.Value
	} else if configItem.Value.String() != "" {
		// Process config value template
		val, err := e.processTemplate(configItem.Value.String())
		if err != nil {
			return "", fmt.Errorf("error processing value template for %s: %w", name, err)
		}
		effectiveValue = val
	} else if configItem.Default.String() != "" {
		// Process config default template
		val, err := e.processTemplate(configItem.Default.String())
		if err != nil {
			return "", fmt.Errorf("error processing default template for %s: %w", name, err)
		}
		effectiveValue = val
	}

	// Cache the result
	e.resolved[name] = effectiveValue
	return effectiveValue, nil
}

func (e *Engine) findConfigItem(name string) *kotsv1beta1.ConfigItem {
	for _, group := range e.config.Spec.Groups {
		for _, item := range group.Items {
			if item.Name == name {
				return &item
			}
		}
	}
	return nil
}
