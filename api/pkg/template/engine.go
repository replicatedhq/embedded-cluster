package template

import (
	"bytes"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

// Mode defines the operating mode of the template engine
type Mode string

const (
	// ModeConfig is for processing and templating the KOTS config itself
	ModeConfig Mode = "config"
	// ModeGeneric is for processing and templating generic manifests
	ModeGeneric Mode = "generic"
)

type Engine struct {
	mode        Mode
	config      *kotsv1beta1.Config
	license     *kotsv1beta1.License
	releaseData *release.ReleaseData

	// Internal state
	configValues     types.AppConfigValues
	prevConfigValues types.AppConfigValues
	tmpl             *template.Template
	funcMap          template.FuncMap
	cache            map[string]resolvedConfigItem
	depsTree         map[string][]string
	stack            []string
	mtx              sync.Mutex
}

type EngineOption func(*Engine)

func WithMode(mode Mode) EngineOption {
	return func(e *Engine) {
		e.mode = mode
	}
}

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
		mode:             ModeGeneric, // default to generic mode
		config:           config,
		configValues:     make(types.AppConfigValues),
		prevConfigValues: make(types.AppConfigValues),
		cache:            make(map[string]resolvedConfigItem),
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
	if e.mode != ModeGeneric {
		return fmt.Errorf("Parse is only available in generic mode, current mode: %s", e.mode)
	}

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

// Execute executes the template engine using the provided config values.
// In ModeConfig, it processes and templates the KOTS config itself, returning the templated config.
// In ModeGeneric, it executes the engine's parsed template and returns the templated result.
func (e *Engine) Execute(configValues types.AppConfigValues) (string, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	// Store previous config values
	e.prevConfigValues = e.configValues
	e.configValues = configValues

	// Mark all cached values as not yet processed in this execution
	for key, cacheVal := range e.cache {
		cacheVal.Processed = false
		e.cache[key] = cacheVal
	}

	// Reset stack
	e.stack = []string{}

	if e.mode == ModeConfig {
		// Template each config item individually first to enable caching of expensive operations like certificate generation.
		// This allows us to track which config items use these operations and determine when they need to be re-run.
		// Templating the entire config at once would make it difficult to associate operations with specific items.
		cfg, err := e.templateConfigItems()
		if err != nil {
			return "", fmt.Errorf("template config items: %w", err)
		}

		// Marshal the updated config
		marshaled, err := kyaml.Marshal(cfg)
		if err != nil {
			return "", fmt.Errorf("marshal config: %w", err)
		}

		// Now template the entire config
		return e.processTemplate(string(marshaled))
	}

	// We're in generic mode, check if a template was parsed
	if e.tmpl == nil {
		return "", fmt.Errorf("template not parsed")
	}

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
		"LicenseDockerCfg":  e.licenseDockerCfg,

		"HTTPProxy":  e.httpProxy,
		"HTTPSProxy": e.httpsProxy,
		"NoProxy":    e.noProxy,

		"Now":          e.now,
		"NowFmt":       e.nowFormat,
		"ToLower":      strings.ToLower,
		"ToUpper":      strings.ToUpper,
		"TrimSpace":    strings.TrimSpace,
		"Trim":         e.trim,
		"UrlEncode":    url.QueryEscape,
		"Base64Encode": e.base64Encode,
		"Base64Decode": e.base64Decode,
		"Split":        strings.Split,
		"RandomBytes":  e.randomBytes,
		"RandomString": e.randomString,
		"Add":          e.add,
		"Sub":          e.sub,
		"Mult":         e.mult,
		"Div":          e.div,
		"ParseBool":    e.parseBool,
		"ParseFloat":   e.parseFloat,
		"ParseInt":     e.parseInt,
		"ParseUint":    e.parseUint,
		"HumanSize":    e.humanSize,
		"YamlEscape":   e.yamlEscape,
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
