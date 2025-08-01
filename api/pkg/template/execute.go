package template

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"text/template"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

// execOptions holds options passed to the template engine at execution time.
type execOptions struct {
	proxySpec *ecv1beta1.ProxySpec // Proxy spec for the engine, if applicable
}

// ExecOption is a function that sets configuration for the engine at execution time.
type ExecOption func(*execOptions)

// WithProxySpec is an ExecOption that sets the proxy spec for the engine.
func WithProxySpec(proxySpec *ecv1beta1.ProxySpec) ExecOption {
	return func(opts *execOptions) {
		opts.proxySpec = proxySpec
	}
}

// Execute executes the template engine using the provided config values and execution options.
// In ModeConfig, it processes and templates the KOTS config itself, returning the templated config.
// In ModeGeneric, it executes the engine's parsed template and returns the templated result.
func (e *Engine) Execute(configValues types.AppConfigValues, opts ...ExecOption) (string, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	// Set execution options
	execOptions := execOptions{}
	for _, opt := range opts {
		opt(&execOptions)
	}
	if err := e.validateExecOptions(execOptions); err != nil {
		return "", fmt.Errorf("validate execution options: %w", err)
	}
	e.proxySpec = execOptions.proxySpec

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

func (e *Engine) validateExecOptions(opts execOptions) error {
	if e.mode == ModeGeneric && opts.proxySpec == nil {
		return errors.New("installation with proxy spec must be provided in generic mode. This is required to process the proxy-related template functions")
	}
	if e.mode == ModeConfig && opts.proxySpec != nil {
		return errors.New("installation with proxy spec should not be provided in config mode. Proxy-related template functions are not available in this mode")
	}
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
