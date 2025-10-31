# Embedded Cluster Lint Package

A hidden lint command for validating Embedded Cluster configuration files.

## Purpose

Validates Embedded Cluster YAML configuration files to catch common issues:
- **YAML syntax errors** (duplicate keys, unclosed quotes, invalid structure)
- **Port configuration issues** (ports in unsupportedOverrides that are already supported)
- **Custom domain validation** (domains must exist in the Replicated app)

## Usage

```bash
# Basic usage
embedded-cluster lint config.yaml

# Multiple files
embedded-cluster lint config1.yaml config2.yaml config3.yaml

# Verbose mode (shows API endpoints and configuration)
embedded-cluster lint -v config.yaml

# With custom domain validation enabled
REPLICATED_API_TOKEN="your-token" \
REPLICATED_API_ORIGIN="https://api.replicated.com/vendor" \
REPLICATED_APP="your-app-id" \
embedded-cluster lint config.yaml
```

## Validation Rules

### 1. YAML Syntax Validation (ERROR)
Validates basic YAML syntax before attempting content validation.

**Catches:**
- Duplicate keys
- Unclosed quotes
- Invalid indentation
- Tabs mixed with spaces
- Malformed YAML structures

**Example:**
```
ERROR: Failed to validate config.yaml: YAML syntax error at line 6: key "version" already set in map
```

### 2. Port Range Validation (WARNING)
Checks if ports specified in `unsupportedOverrides` are within the default supported range (80-32767).

**Why it matters:** Ports in this range don't need to be in unsupportedOverrides - they're already supported by default.

**Example:**
```
WARNING: config.yaml: unsupportedOverrides.builtInExtensions[adminconsole].service.nodePort: port 30000 is already supported (supported range: 80-32767) and should not be in unsupportedOverrides
```

### 3. Custom Domain Validation (ERROR)
When environment variables are provided, validates that custom domains exist in the app's configuration.

**Requires all three environment variables:**
- `REPLICATED_API_TOKEN` - Authentication token for Replicated API
- `REPLICATED_API_ORIGIN` - API endpoint (e.g., `https://api.replicated.com/vendor`)
- `REPLICATED_APP` - Application ID or slug

**If any are missing:** Shows informative message and skips domain validation.

**Example:**
```
ERROR: config.yaml: domains.replicatedAppDomain: custom domain "invalid.example.com" not found in app's configured domains
```

## Exit Codes

- **0**: Validation passed (may have warnings)
- **1**: Validation failed (has errors)

Warnings do NOT cause a non-zero exit code.

## Verbose Mode

Use the `-v` flag to see detailed information:

```bash
embedded-cluster lint -v config.yaml
```

**Shows:**
- Environment configuration (token shown as `<set>`)
- API endpoints being called
- HTTP response status codes
- Custom domains found
- Fallback attempts to alternate endpoints

**Example output:**
```
Environment configuration:
  REPLICATED_API_ORIGIN: https://api.replicated.com/vendor
  REPLICATED_APP: my-app
  REPLICATED_API_TOKEN: <set>
Starting custom domain validation
Fetching channels from: https://api.replicated.com/vendor/v3/app/my-app/channels
Attempting to fetch custom domains from: https://api.replicated.com/vendor/v3/app/my-app/custom-hostnames
Response status: 200 200 OK
```

## Testing

### Run all tests
```bash
go test ./pkg/lint/... -v
```

### Run specific test suites
```bash
# YAML syntax validation
go test ./pkg/lint/... -run TestValidateYAMLSyntax

# Port validation
go test ./pkg/lint/... -run TestValidatePorts

# API client
go test ./pkg/lint/... -run TestAPIClient
```

### Test with example specs
```bash
# Test syntax errors
./bin/embedded-cluster lint ./pkg/lint/testdata/specs/syntax-error-duplicate-key.yaml

# Test port warnings
./bin/embedded-cluster lint ./pkg/lint/testdata/specs/01-warning-port-in-range.yaml

# Test valid configuration
./bin/embedded-cluster lint ./pkg/lint/testdata/specs/04-valid-ports-outside-range.yaml
```

## Package Structure

- `validator.go` - Core validation logic
- `validator_test.go` - Validation tests
- `api_client.go` - Replicated API client for custom domain fetching
- `api_client_test.go` - API client tests
- `testdata/specs/` - Example YAML files for testing

## Development

The lint command is currently **hidden** (not shown in `--help` output). To make it visible, modify `cmd/installer/cli/lint.go` and set `Hidden: false`.

### Adding New Validation Rules

1. Add validation function to `validator.go`
2. Call it from `ValidateFile()`
3. Return warnings or errors as appropriate
4. Add test cases to `validator_test.go`
5. Create example spec files in `testdata/specs/`