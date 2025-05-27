# Embedded Cluster API Package

This package provides the core API functionality for the Embedded Cluster system. It handles installation, authentication, console access, and health monitoring of the cluster.

## Package Structure

### Root Level
The root directory contains the main API setup files and request handlers.

### Subpackages

#### `/controllers`
Contains the business logic for different API endpoints. Each controller package focuses on a specific domain of functionality (e.g., authentication, console, installation) and implements the core business logic for that domain.

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
Contains shared utilities and helper packages that provide common functionality used across different parts of the API. This includes both general-purpose utilities and domain-specific helpers.

#### `/client`
Provides a client library for interacting with the API. The client package implements a clean interface for making API calls and handling responses, making it easy to integrate with the API from other parts of the system.

## Where to Add New Functionality

1. **New API Endpoints**:
   - Add route definitions in the root API setup
   - Create corresponding controller in `/controllers`
   - Define request/response types in `/types`

2. **New Business Logic**:
   - Place in appropriate controller under `/controllers`
   - Share common logic in `/pkg` if used across multiple controllers

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
/api/swagger/
```

You can use this interactive documentation to explore the available endpoints, understand request/response formats, and test API operations directly from your browser. 
