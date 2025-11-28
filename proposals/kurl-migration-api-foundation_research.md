# kURL Migration API Foundation Research

## Executive Summary
This research examines the existing codebase to understand patterns and implementation approaches for building the kURL to Embedded Cluster V3 Migration API foundation (story sc-130971). The API will provide REST endpoints to enable Admin Console UI integration for migrating from kURL to Embedded Cluster.

## Current Codebase Analysis

### API Architecture Pattern
The codebase follows a consistent layered architecture:
- **Routes** (`api/routes.go`): Handles HTTP route registration using Gorilla Mux
- **Handlers** (`api/internal/handlers/`): HTTP request/response handling, validation, and Swagger documentation
- **Controllers** (`api/controllers/`): Business logic orchestration
- **Managers** (`api/internal/managers/`): Core business operations and external system interactions
- **Stores** (`api/internal/store/`): Data persistence layer

### Existing Migration Implementation
The migration API skeleton is already partially implemented:

#### Routes (api/routes.go)
- Routes are registered under `/linux/migration/` prefix
- Three endpoints defined: `/config`, `/start`, `/status`
- Routes are mode-agnostic (available in both install and upgrade modes)

#### Handler (api/internal/handlers/migration/handler.go)
- Implements GET `/config`, POST `/start`, GET `/status` endpoints
- Uses consistent error handling patterns with `utils.JSONError`
- Includes comprehensive Swagger documentation
- Validates transfer mode (copy/move) in the handler

#### Controller (api/controllers/migration/controller.go)
- Implements business logic for migration operations
- Uses functional options pattern for configuration
- Manages migration state through store abstraction
- Launches background goroutine for async migration execution
- Skeleton implementation of `Run()` method for orchestration

#### Manager (api/internal/managers/migration/manager.go)
- Provides core migration operations
- Implements config extraction and merging logic
- Transfer mode validation
- Skeleton implementation for phase execution

#### Store (api/internal/store/migration/store.go)
- In-memory implementation for migration state
- Thread-safe with RWMutex
- Tracks migration ID, status, phase, progress, and error state
- Deep copy for safe data retrieval

#### Types (api/types/migration.go)
- Defines migration-specific types and constants
- Error definitions with proper HTTP status codes
- Request/response structures with validation tags
- State and phase enums

### Key Patterns Observed

1. **Dependency Injection**: All components use functional options pattern for configuration
2. **Interface-Based Design**: Controllers, managers, and stores are defined as interfaces
3. **Error Handling**: Consistent use of typed errors with HTTP status codes
4. **Testing**: Comprehensive unit tests with mocks for all dependencies
5. **Documentation**: Swagger annotations on all public endpoints
6. **Logging**: Structured logging with Logrus throughout
7. **Thread Safety**: Proper mutex usage in stores for concurrent access

### Integration Points

1. **Linux Installation Manager**: Used to get EC defaults and manage installation
2. **Kubernetes Client**: Will be used to query kURL cluster configuration
3. **Authentication**: Routes protected by auth middleware
4. **Metrics Reporter**: Integration point for telemetry (not yet implemented)

### Configuration Management

The API uses a three-level configuration precedence:
1. **User Config**: Highest priority, provided via API
2. **kURL Config**: Extracted from running cluster
3. **EC Defaults**: Base configuration

Merging follows a field-by-field override pattern where non-zero values take precedence.

### Testing Strategy

The codebase demonstrates comprehensive testing:
- Unit tests for controllers with mocked dependencies
- Table-driven tests for multiple scenarios
- Error case coverage including HTTP status codes
- Mock generation using Testify

## Key Implementation Considerations

### Network Configuration
- Must calculate non-overlapping CIDRs between kURL and EC
- Pod CIDR, Service CIDR, and Global CIDR must be distinct
- Network interface selection may need validation

### State Management
- Currently uses in-memory store (needs persistence in PR 7)
- Must handle concurrent access safely
- Resume capability after interruption required

### Async Execution
- Migration runs in background goroutine
- Status endpoint provides real-time progress
- Error handling must update state appropriately

### Data Transfer Modes
- **Copy Mode**: Safe but requires 2x disk space
- **Move Mode**: Efficient but destructive
- Mode validation at API layer

## Gaps to Address

1. **Persistent Store**: Current memory store needs file-based persistence (PR 7)
2. **kURL Config Extraction**: GetKurlConfig() not yet implemented
3. **EC Defaults**: Integration with RuntimeConfig needed
4. **Phase Execution**: ExecutePhase() skeleton needs implementation (PR 8)
5. **CIDR Calculation**: Logic to ensure non-overlapping networks
6. **Metrics Reporting**: Telemetry integration not implemented
7. **Password Hash**: kURL password hash export for auth compatibility

## API Design Requirements

### GET /api/migration/config
- Returns merged configuration (kURL + EC defaults)
- No authentication of kURL cluster needed
- Provides values, defaults, and resolved config

### POST /api/migration/start
- Accepts transfer mode and optional user config
- Validates configuration
- Generates migration UUID
- Launches async migration
- Returns migration ID immediately

### GET /api/migration/status
- Returns current state, phase, progress
- Provides user-friendly messages
- Includes error details if failed
- 404 if no active migration

## Security Considerations

1. **Authentication**: All endpoints require bearer token auth
2. **Password Compatibility**: kURL password hash must be preserved
3. **Network Isolation**: Migration must not disrupt running workloads
4. **Rollback Safety**: Must preserve ability to rollback if migration fails

## Performance Considerations

1. **Async Execution**: Migration runs in background to avoid timeout
2. **Progress Tracking**: Regular status updates for UI
3. **Resource Usage**: Must monitor disk space for copy mode
4. **Service Availability**: Minimize downtime during migration

## Recommendations

1. **Use Existing Patterns**: Follow handler→controller→manager architecture
2. **Implement Store Persistence Early**: File-based store critical for reliability
3. **Add Comprehensive Validation**: Config validation before migration start
4. **Implement Idempotency**: Allow safe retry of failed migrations
5. **Add Telemetry**: Report migration metrics for observability
6. **Test Error Scenarios**: Ensure graceful handling of all failure modes

## Conclusion

The codebase provides a solid foundation with clear patterns and partial implementation. The main work involves:
1. Completing the skeleton implementations
2. Adding persistent state storage
3. Implementing kURL config extraction
4. Adding CIDR calculation logic
5. Comprehensive testing of all scenarios

The existing architecture supports all requirements and provides good separation of concerns for maintainability.