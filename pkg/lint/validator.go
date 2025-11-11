package lint

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"gopkg.in/yaml.v2"
	k8syaml "sigs.k8s.io/yaml"
)

// Validator validates embedded cluster configuration files
type Validator struct {
	apiClient *APIClient
	verbose   bool
}

// NewValidator creates a new validator instance
func NewValidator(apiToken, apiOrigin, appID string) *Validator {
	return &Validator{
		apiClient: NewAPIClient(apiToken, apiOrigin, appID),
		verbose:   false,
	}
}

// SetVerbose enables or disables verbose mode
func (v *Validator) SetVerbose(verbose bool) {
	v.verbose = verbose
	v.apiClient.SetVerbose(verbose)
}

// ValidationError represents a validation error found during linting
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationWarning represents a validation warning found during linting
type ValidationWarning struct {
	Field   string
	Message string
}

func (w ValidationWarning) String() string {
	return fmt.Sprintf("%s: %s", w.Field, w.Message)
}

// ValidationResult contains both errors and warnings from validation
type ValidationResult struct {
	Errors   []error
	Warnings []ValidationWarning
}

// JSONOutput represents the JSON output format for all linted files
type JSONOutput struct {
	Files []FileResult `json:"files"`
}

// FileResult represents the validation result for a single file in JSON format
type FileResult struct {
	Path     string            `json:"path"`
	Valid    bool              `json:"valid"`
	Errors   []ValidationIssue `json:"errors,omitempty"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
}

// ValidationIssue represents a single validation error or warning in JSON format
type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ToJSON converts a ValidationResult to a FileResult for JSON output
func (r *ValidationResult) ToJSON(path string) FileResult {
	result := FileResult{
		Path:     path,
		Valid:    len(r.Errors) == 0,
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}

	for _, err := range r.Errors {
		if ve, ok := err.(ValidationError); ok {
			result.Errors = append(result.Errors, ValidationIssue(ve))
		} else {
			result.Errors = append(result.Errors, ValidationIssue{
				Field:   "",
				Message: err.Error(),
			})
		}
	}

	for _, warning := range r.Warnings {
		result.Warnings = append(result.Warnings, ValidationIssue(warning))
	}

	return result
}

// ValidateFile validates a single configuration file
func (v *Validator) ValidateFile(path string) (*ValidationResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Validate YAML syntax first (before attempting to parse into structs)
	// This provides better error messages for syntax errors
	if err := v.validateYAMLSyntax(data); err != nil {
		return nil, err
	}

	// Parse the config using k8s yaml which properly handles the embedded types
	var config ecv1beta1.Config
	if err := k8syaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	result := &ValidationResult{
		Errors:   []error{},
		Warnings: []ValidationWarning{},
	}

	// Validate ports in unsupportedOverrides (returns warnings)
	portWarnings := v.validatePorts(config.Spec.UnsupportedOverrides)
	result.Warnings = append(result.Warnings, portWarnings...)

	// Validate custom domains if environment variables are set (returns errors)
	if v.apiClient.isConfigured() {
		if v.verbose {
			v.apiClient.logConfiguration()
		}
		domainErrors, err := v.validateDomainsWithAPI(config.Spec.Domains)
		if err != nil {
			// Log the API error but continue validation
			result.Errors = append(result.Errors, ValidationError{
				Field:   "domains",
				Message: fmt.Sprintf("failed to fetch custom domains from API: %v", err),
			})
		} else {
			result.Errors = append(result.Errors, domainErrors...)
		}
	} else if config.Spec.Domains.ReplicatedAppDomain != "" ||
		config.Spec.Domains.ProxyRegistryDomain != "" ||
		config.Spec.Domains.ReplicatedRegistryDomain != "" {
		// Config has custom domains but API validation is not configured
		missing := v.apiClient.getMissingConfig()
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "INFO: Skipping custom domain validation. Missing environment variable(s):\n")
			for _, m := range missing {
				fmt.Fprintf(os.Stderr, "  - %s\n", m)
			}
			fmt.Fprintf(os.Stderr, "Set all three environment variables to enable custom domain validation.\n")
		}
	}

	// Validate architecture values if provided
	for i, arch := range config.Spec.Architecture {
		val := string(arch)
		switch val {
		case "aarch64", "x86_64":
			// ok
		default:
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("architecture[%d]", i),
				Message: fmt.Sprintf("invalid value %q; allowed values are aarch64 and x86_64", string(arch)),
			})
		}
	}

	return result, nil
}

// validateYAMLSyntax validates that the YAML is syntactically correct
// Adapted from kots-lint/pkg/kots/lint.go:788-846
func (v *Validator) validateYAMLSyntax(data []byte) error {
	reader := bytes.NewReader(data)
	decoder := yaml.NewDecoder(reader)
	decoder.SetStrict(true) // Catches duplicate keys, wrong types, etc.

	for {
		var doc interface{}
		err := decoder.Decode(&doc)

		if err == nil {
			continue // Document valid, check next
		}
		if err == io.EOF {
			break // All documents checked
		}

		// YAML syntax error found - extract line number from error message
		lineNum := extractLineNumber(err.Error())
		if lineNum > 0 {
			return fmt.Errorf("YAML syntax error at line %d: %v", lineNum, err)
		}
		return fmt.Errorf("YAML syntax error: %v", err)
	}

	return nil
}

// extractLineNumber extracts line number from YAML error messages
// YAML errors typically look like: "yaml: line 15: mapping values are not allowed in this context"
func extractLineNumber(errMsg string) int {
	re := regexp.MustCompile(`line (\d+)`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		if line, err := strconv.Atoi(matches[1]); err == nil {
			return line
		}
	}
	return 0 // Unknown line
}

// validatePorts validates that ports in unsupportedOverrides are not already supported
func (v *Validator) validatePorts(overrides ecv1beta1.UnsupportedOverrides) []ValidationWarning {
	var warnings []ValidationWarning

	// Default supported port range is 80-32767
	minSupportedPort := 80
	maxSupportedPort := 32767

	for _, ext := range overrides.BuiltInExtensions {
		if ext.Values == "" {
			continue
		}

		// Parse the YAML values
		var values interface{}
		if err := yaml.Unmarshal([]byte(ext.Values), &values); err != nil {
			warnings = append(warnings, ValidationWarning{
				Field:   fmt.Sprintf("unsupportedOverrides.builtInExtensions[%s]", ext.Name),
				Message: fmt.Sprintf("failed to parse YAML values: %v", err),
			})
			continue
		}

		// Look for nodePort settings
		ports := v.extractNodePorts(values, []string{})
		for _, portInfo := range ports {
			port := portInfo.port
			path := portInfo.path

			if port >= minSupportedPort && port <= maxSupportedPort {
				warnings = append(warnings, ValidationWarning{
					Field: fmt.Sprintf("unsupportedOverrides.builtInExtensions[%s].%s", ext.Name, strings.Join(path, ".")),
					Message: fmt.Sprintf("port %d is already supported (supported range: %d-%d) and should not be in unsupportedOverrides",
						port, minSupportedPort, maxSupportedPort),
				})
			}
		}
	}

	return warnings
}

type portInfo struct {
	port int
	path []string
}

// extractNodePorts recursively extracts nodePort values from the parsed YAML
func (v *Validator) extractNodePorts(data interface{}, path []string) []portInfo {
	var ports []portInfo

	switch val := data.(type) {
	case map[interface{}]interface{}:
		for k, value := range val {
			key, ok := k.(string)
			if !ok {
				continue
			}

			newPath := append(append([]string{}, path...), key)

			// Check if this is a nodePort field
			if key == "nodePort" {
				if port := v.extractPortValue(value); port > 0 {
					ports = append(ports, portInfo{port: port, path: newPath})
				}
			} else {
				// Recursively search in nested structures
				ports = append(ports, v.extractNodePorts(value, newPath)...)
			}
		}
	case map[string]interface{}:
		for k, value := range val {
			newPath := append(append([]string{}, path...), k)

			// Check if this is a nodePort field
			if k == "nodePort" {
				if port := v.extractPortValue(value); port > 0 {
					ports = append(ports, portInfo{port: port, path: newPath})
				}
			} else {
				// Recursively search in nested structures
				ports = append(ports, v.extractNodePorts(value, newPath)...)
			}
		}
	case []interface{}:
		for i, item := range val {
			newPath := append(append([]string{}, path...), fmt.Sprintf("[%d]", i))
			ports = append(ports, v.extractNodePorts(item, newPath)...)
		}
	}

	return ports
}

// extractPortValue extracts a port number from various types
func (v *Validator) extractPortValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		// Try to parse as integer
		if port, err := strconv.Atoi(v); err == nil {
			return port
		}
	}
	return 0
}

// validateDomainsWithAPI validates custom domains by fetching allowed domains from the API
func (v *Validator) validateDomainsWithAPI(domains ecv1beta1.Domains) ([]error, error) {
	// Fetch allowed custom domains from API
	customDomains, err := v.apiClient.GetCustomDomains()
	if err != nil {
		return nil, err
	}

	// Call the pure validation function
	return v.validateDomains(domains, customDomains), nil
}

// validateDomains validates custom domains against a list of allowed domains (pure function)
func (v *Validator) validateDomains(domains ecv1beta1.Domains, allowedDomains []string) []error {
	var errors []error

	// Create a set of allowed domains for easy lookup
	allowedSet := make(map[string]bool)
	for _, domain := range allowedDomains {
		allowedSet[domain] = true
	}

	// Also add default domains as they're always allowed
	allowedSet["replicated.app"] = true
	allowedSet["proxy.replicated.com"] = true
	allowedSet["registry.replicated.com"] = true

	// Check each configured domain
	if domains.ReplicatedAppDomain != "" && !allowedSet[domains.ReplicatedAppDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.replicatedAppDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ReplicatedAppDomain),
		})
	}

	if domains.ProxyRegistryDomain != "" && !allowedSet[domains.ProxyRegistryDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.proxyRegistryDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ProxyRegistryDomain),
		})
	}

	if domains.ReplicatedRegistryDomain != "" && !allowedSet[domains.ReplicatedRegistryDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.replicatedRegistryDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ReplicatedRegistryDomain),
		})
	}

	return errors
}
