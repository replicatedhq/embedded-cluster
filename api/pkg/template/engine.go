package template

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"maps"
	"slices"
	"strings"
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
	config      *kotsv1beta1.Config
	license     *kotsv1beta1.License
	releaseData *release.ReleaseData

	// Internal state
	configValues     types.AppConfigValues
	prevConfigValues types.AppConfigValues
	tmpl             *template.Template
	funcMap          template.FuncMap
	cache            map[string]CacheValue
	depsTree         map[string][]string
	stack            []string
	mtx              sync.Mutex
}

type CacheValue struct {
	Value     string
	Processed bool
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
		config:           config,
		configValues:     make(types.AppConfigValues),
		prevConfigValues: make(types.AppConfigValues),
		cache:            make(map[string]CacheValue),
		depsTree:         make(map[string][]string),
		stack:            []string{},
		mtx:              sync.Mutex{},
	}

	for _, opt := range opts {
		opt(engine)
	}

	// Initialize funcMap once
	engine.funcMap = sprig.TxtFuncMap()
	maps.Copy(engine.funcMap, engine.getFuncMap())

	return engine
}

// Parse parses a template and populates the engine's template
func (e *Engine) Parse(templateStr string) error {
	tmpl, err := e.parse(templateStr)
	if err != nil {
		return err
	}

	e.tmpl = tmpl
	return nil
}

// parse parses a template string and returns a prepared template
func (e *Engine) parse(templateStr string) (*template.Template, error) {
	// go's template doesn't support multiple delimiters, so we normalize the template
	normalized := strings.ReplaceAll(templateStr, "repl{{", "{{repl")

	tmpl, err := template.New("template").Delims("{{repl", "}}").Funcs(e.funcMap).Parse(normalized)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// Execute executes a the engine's parsed template
func (e *Engine) Execute(configValues types.AppConfigValues) (string, error) {
	if e.tmpl == nil {
		return "", fmt.Errorf("template not parsed")
	}

	e.mtx.Lock()
	defer e.mtx.Unlock()

	// Store previous config values
	e.prevConfigValues = e.configValues
	e.configValues = configValues

	// Mark all cached values as not yet processed in this execution
	for name, cacheVal := range e.cache {
		cacheVal.Processed = false
		e.cache[name] = cacheVal
	}

	// Reset stack
	e.stack = []string{}

	var buf bytes.Buffer
	if err := e.tmpl.Execute(&buf, nil); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// processTemplate processes a template string using the current execution state
func (e *Engine) processTemplate(templateStr string) (string, error) {
	// Quick optimization: if there are no template delimiters, return as-is
	if !strings.Contains(templateStr, "{{") {
		return templateStr, nil
	}

	tmpl, err := e.parse(templateStr)
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
		"ConfigOption":          e.configOption,
		"ConfigOptionData":      e.configOptionData,
		"ConfigOptionEquals":    e.configOptionEquals,
		"ConfigOptionFilename":  e.configOptionFilename,
		"ConfigOptionNotEquals": e.configOptionNotEquals,

		"LicenseFieldValue": e.licenseFieldValue,
	}
}

func (e *Engine) configOption(name string) (string, error) {
	e.recordDependency(name)

	val, err := e.resolveConfigItem(name, e.getConfigItemValue)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}
	return val, nil
}

func (e *Engine) configOptionData(name string) (string, error) {
	e.recordDependency(name)

	val, err := e.resolveConfigItem(name, e.getConfigItemValue)
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

func (e *Engine) configOptionEquals(name, expected string) (bool, error) {
	e.recordDependency(name)

	val, err := e.resolveConfigItem(name, e.getConfigItemValue)
	if err != nil {
		return false, fmt.Errorf("resolve config item: %w", err)
	}
	return val == expected, nil
}

func (e *Engine) configOptionNotEquals(name, expected string) (bool, error) {
	e.recordDependency(name)

	val, err := e.resolveConfigItem(name, e.getConfigItemValue)
	if err != nil {
		// NOTE: this is parity from KOTS but I would expect this to return true
		return false, fmt.Errorf("resolve config item: %w", err)
	}
	return val != expected, nil
}

func (e *Engine) configOptionFilename(name string) (string, error) {
	e.recordDependency(name)

	val, err := e.resolveConfigItem(name, e.getConfigItemFilename)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}
	return val, nil
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

// recordDependency records that the current item depends on another item
func (e *Engine) recordDependency(dependency string) {
	// Get the current item in the stack
	if len(e.stack) > 0 {
		currentItem := e.stack[len(e.stack)-1]

		// Add dependency if not already present
		if !slices.Contains(e.depsTree[currentItem], dependency) {
			e.depsTree[currentItem] = append(e.depsTree[currentItem], dependency)
		}
	}
}

// shouldInvalidate checks if a cached item should be invalidated
func (e *Engine) shouldInvalidate(itemName string) bool {
	// Check if this item's user value changed
	if e.configValueChanged(itemName) {
		return true
	}

	// Recursively check if any dependencies should be invalidated
	for _, dep := range e.depsTree[itemName] {
		if e.shouldInvalidate(dep) {
			return true
		}
	}

	return false
}

// configValueChanged checks if a config item's user value changed
func (e *Engine) configValueChanged(itemName string) bool {
	prevVal, prevExists := e.prevConfigValues[itemName]
	currentVal, currentExists := e.configValues[itemName]

	if prevExists != currentExists {
		return true
	}

	return prevVal.Value != currentVal.Value
}

// resolveConfigItem resolves a specific config item (internal recursive method)
func (e *Engine) resolveConfigItem(name string, getter func(configItem kotsv1beta1.ConfigItem) (string, error)) (string, error) {
	// Check if we have a cached value
	if cacheVal, exists := e.cache[name]; exists {
		// If already processed in this execution, use it
		if cacheVal.Processed {
			return cacheVal.Value, nil
		}

		// Value is from previous execution - check if still valid
		if !e.shouldInvalidate(name) {
			// Still valid - mark as processed and use it
			cacheVal.Processed = true
			e.cache[name] = cacheVal
			return cacheVal.Value, nil
		}

		// Value is stale - remove from cache
		delete(e.cache, name)
	}

	// Check for circular dependency
	if slices.Contains(e.stack, name) {
		return "", fmt.Errorf("circular dependency detected for %s", name)
	}

	// Track resolution path for dependency discovery
	e.stack = append(e.stack, name)
	defer func() {
		if len(e.stack) > 0 {
			e.stack = e.stack[:len(e.stack)-1]
		}
	}()

	// Find the config item definition
	configItem := e.findConfigItem(name)
	if configItem == nil {
		return "", fmt.Errorf("config item %s not found", name)
	}

	value, err := getter(*configItem)
	if err != nil {
		return "", fmt.Errorf("get config item %s value: %w", name, err)
	}

	// Cache the result and mark as processed
	e.cache[name] = CacheValue{
		Value:     value,
		Processed: true,
	}

	return value, nil
}

func (e *Engine) getConfigItemValue(configItem kotsv1beta1.ConfigItem) (string, error) {
	// Priority: user value > config value > config default
	if userVal, exists := e.configValues[configItem.Name]; exists {
		return userVal.Value, nil
	}

	// Try config value first
	if configItem.Value.String() != "" {
		val, err := e.processTemplate(configItem.Value.String())
		if err != nil {
			return "", fmt.Errorf("process value template: %w", err)
		}
		return val, nil
	}

	// If still empty, try default
	if configItem.Default.String() != "" {
		val, err := e.processTemplate(configItem.Default.String())
		if err != nil {
			return "", fmt.Errorf("process default template: %w", err)
		}
		return val, nil
	}

	// If still empty, return empty string
	return "", nil
}

func (e *Engine) getConfigItemFilename(configItem kotsv1beta1.ConfigItem) (string, error) {
	// Priority: user value
	if userVal, exists := e.configValues[configItem.Name]; exists {
		return userVal.Filename, nil
	}

	// Do not use the config item's filename for KOTS parity

	// If still empty, return empty string
	return "", nil
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
