package lint

import (
	"fmt"
	"io"
	"os"
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
		domainErrors, err := v.validateDomains(config.Spec.Domains)
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

	return result, nil
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

			newPath := append(path, key)

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
			newPath := append(path, k)

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
			newPath := append(path, fmt.Sprintf("[%d]", i))
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

// validateDomains validates custom domains against the Replicated API
func (v *Validator) validateDomains(domains ecv1beta1.Domains) ([]error, error) {
	var errors []error

	// Fetch allowed custom domains from API
	customDomains, err := v.apiClient.GetCustomDomains()
	if err != nil {
		return nil, err
	}

	// Create a set of allowed domains for easy lookup
	allowedDomains := make(map[string]bool)
	for _, domain := range customDomains {
		allowedDomains[domain] = true
	}

	// Also add default domains as they're always allowed
	allowedDomains["replicated.app"] = true
	allowedDomains["proxy.replicated.com"] = true
	allowedDomains["registry.replicated.com"] = true

	// Check each configured domain
	if domains.ReplicatedAppDomain != "" && !allowedDomains[domains.ReplicatedAppDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.replicatedAppDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ReplicatedAppDomain),
		})
	}

	if domains.ProxyRegistryDomain != "" && !allowedDomains[domains.ProxyRegistryDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.proxyRegistryDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ProxyRegistryDomain),
		})
	}

	if domains.ReplicatedRegistryDomain != "" && !allowedDomains[domains.ReplicatedRegistryDomain] {
		errors = append(errors, ValidationError{
			Field:   "domains.replicatedRegistryDomain",
			Message: fmt.Sprintf("custom domain %q not found in app's configured domains", domains.ReplicatedRegistryDomain),
		})
	}

	return errors, nil
}