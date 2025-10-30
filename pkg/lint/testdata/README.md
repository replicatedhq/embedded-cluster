# Lint Command Test Specifications

This directory contains test YAML specifications for the `embedded-cluster lint` command.

## Test Files

### 01-warning-port-in-range.yaml
- **Expected**: WARNING
- **Issue**: Port 30000 is within the supported range (80-32767)
- **Warning Message**: "port 30000 is already supported"

### 02-warning-multiple-ports.yaml
- **Expected**: 3 WARNINGS
- **Issues**:
  - Port 8080 in adminconsole (in range)
  - Port 443 in openebs (in range)
  - Port 9090 in registry (in range)
  - Port 50000 in openebs is valid (outside range)

### 04-valid-ports-outside-range.yaml
- **Expected**: PASS
- **All ports are outside the supported range**:
  - Port 50000 (above range)
  - Port 79 (below range)
  - Port 32768 (above range)
  - Port 60000 (above range)

### 06-error-custom-domains.yaml
- **Expected**: ERROR when API credentials are provided
- **Issues**: Invalid custom domains
- **Note**: Only validates domains when these environment variables are set:
  - `REPLICATED_API_TOKEN`
  - `REPLICATED_API_ORIGIN`
  - `REPLICATED_APP`

## Running Tests

### Basic test (without API validation)
```bash
./bin/embedded-cluster lint ./pkg/lint/testdata/specs/01-error-port-in-range.yaml
```

### Test with API validation
```bash
REPLICATED_API_TOKEN="your-token" \
REPLICATED_API_ORIGIN="https://api.replicated.com/vendor" \
REPLICATED_APP="your-app-id" \
./bin/embedded-cluster lint ./pkg/lint/testdata/specs/06-error-custom-domains.yaml
```

### Run all tests
```bash
./pkg/lint/testdata/run-tests.sh
```

## Validation Rules

1. **Port Range Validation (WARNING)**: Ports between 80-32767 are already supported and should NOT be in `unsupportedOverrides`. This produces a warning, not an error.
2. **Custom Domain Validation (ERROR)**: When API credentials are provided, domains must exist in the app's configured custom domains. Invalid domains produce errors.