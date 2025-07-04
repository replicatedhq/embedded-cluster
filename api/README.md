# Embedded Cluster API Package

This package provides the core API functionality for the Embedded Cluster system. It handles installation, authentication, console access, and health monitoring of the cluster.

## Package Structure

### Root Level
The root directory contains the main API setup files and request handlers.

### Subpackages

#### `/controllers`
Contains the business logic for different API endpoints. Each controller package focuses on a specific domain of functionality or workflow (e.g., authentication, console, install, upgrade, join, etc.) and implements the core business logic for that domain or workflow. Controllers can utilize multiple managers with each manager handling a specific subdomain of functionality.

#### `/internal`
Contains shared utilities and helper packages that provide common functionality used across different parts of the API. This includes both general-purpose utilities and domain-specific helpers.

#### `/internal/managers`
Each manager is responsible for a specific subdomain of functionality and provides a clean interface for controllers to interact with. For example, the Preflight Manager manages system requirement checks and validation.

#### `/internal/statemachine`
The statemachine is used by controllers to capture workflow state and enforce valid transitions.

#### `/types`
Defines the core data structures and types used throughout the API. This includes:
- Request and response types
- Domain models
- Custom error types
- Shared interfaces

#### `/docs`
Contains Swagger-generated API documentation. This includes:
- API endpoint definitions
- Request/response schemas
- Authentication methods
- API operation descriptions

#### `/pkg`
Contains helper packages that can be used by packages external to the API.

#### `/client`
Provides a client library for interacting with the API. The client package implements a clean interface for making API calls and handling responses, making it easy to integrate with the API from other parts of the system.

## Where to Add New Functionality

1. **New API Endpoints**:
   - Add route definitions in the root API setup
   - Create or update corresponding controller in `/controllers`
   - Define request/response types in `/types`

2. **New Business Logic**:
   - Place in appropriate controller under `/controllers` if the logic represents a distinct domain or workflow
   - Place in appropriate manager under `/internal/managers` if the logic represents a distinct subdomain

3. **New Types/Models**:
   - Add to `/types` directory
   - Include validation and serialization methods

4. **New Client Methods**:
   - Add to appropriate file in `/client`
   - Include corresponding tests

5. **New Utilities**:
   - Place in `/pkg/utils` if general purpose
   - Create new subpackage under `/pkg` if domain-specific

## Best Practices

1. **Error Handling**:
   - Use custom error types from `/types`
   - Include proper error wrapping and context
   - Maintain consistent error handling patterns

2. **Testing**:
   - Write unit tests for all new functionality
   - Include integration tests for API endpoints
   - Maintain high test coverage

3. **Documentation**:
   - Document all public types and functions
   - Include examples for complex operations
   - Keep README updated with new functionality

4. **Logging**:
   - Use the logging utilities from the root package
   - Include appropriate log levels and context
   - Follow consistent logging patterns

## Architecture Decisions

1. **Release Metadata Independence**:
   - The EC API should not use the release metadata embedded into the EC binary (CLI)
   - This design choice enables better testability and easier iteration in the development environment
   - API components should be independently configurable and testable

2. **Kubernetes as a Subset of Linux**:
   - The Kubernetes installation target should be a subset of the Linux installation target
   - Linux installations include Kubernetes cluster setup (k0s, addons) plus application management
   - Kubernetes installations focus on application management (deployment, upgrades, lifecycle) on an existing Kubernetes cluster
   - Once Linux installation finishes setting up the Kubernetes cluster, subsequent operations should follow the same workflow as Kubernetes installations

## Integration

The API package is designed to be used as part of the larger Embedded Cluster system. It provides both HTTP endpoints for external access and a client library for internal use.

For integration examples and usage patterns, refer to the integration tests in the `/integration` directory. 

## Generating the Docs

The API documentation is generated using Swagger. To generate or update the docs:

1. Ensure the `swag` tool is installed:
   ```
   make swag
   ```

2. Generate the Swagger documentation:
   ```
   make swagger
   ```

This will scan the codebase for Swagger annotations and generate the API documentation files in the `/docs` directory.

Once the API is running, the Swagger documentation is available at the endpoint:
```
/api/swagger/index.html
```

You can use this interactive documentation to explore the available endpoints, understand request/response formats, and test API operations directly from your browser. 
