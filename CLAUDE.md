# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build and Run
```bash
# Build the application
make build
# or
go build -o fhir-validation-proxy ./cmd/server

# Run the application
make run
# or
./fhir-validation-proxy

# Clean build artifacts
make clean
```

### Testing
```bash
# Run all tests
make test
# or
go test ./...

# Run tests with coverage
make coverage

# Run specific test package
go test ./internal/validator -v

# Run specific test function
go test ./internal/proxy -run TestFHIRProxy/process_message -v
```

### Linting
```bash
# Run linter
make lint

# Run linter with auto-fix
make lint-fix
```

## Architecture Overview

This is a FHIR validation proxy that sits between clients and Google Cloud Healthcare API (or other FHIR servers) to provide enhanced validation and security.

### Core Architecture

The application follows a layered architecture:

1. **Entry Point** (`cmd/server/main.go`): Initializes all components and starts the HTTP server with graceful shutdown
2. **Proxy Layer** (`internal/proxy/`): Handles FHIR R4 API routing and request/response transformation  
3. **Validation Engine** (`internal/validator/`): Core validation logic for resources, bundles, and messages
4. **Authentication** (`internal/auth/`): Google Cloud IAM integration and request authentication
5. **Configuration** (`internal/config/`): YAML-based configuration with environment variable overrides

### Key Components

**FHIRProxy** (`internal/proxy/fhir_proxy.go`):
- Routes FHIR operations: `/{resourceType}`, `/{resourceType}/{id}`, `/$process-message`, `/metadata`
- Route ordering is critical - system operations must come before resource operations to avoid conflicts
- Validates requests before forwarding to upstream FHIR server
- Supports both Google Cloud Healthcare API and generic FHIR servers

**Validation Engine** (`internal/validator/`):
- `validator.go`: Main validation orchestrator that handles different resource types and bundle types
- `rules.go`: Field-level validation rules (min/max counts, patterns, allowed values)
- `recipes.go`: Bundle validation recipes with enhanced features (resource counts, forbidden resources, conditional rules)
- `profiles.go`: FHIR StructureDefinition profile validation

**Configuration System** (`internal/config/config.go`):
- Hierarchical config: defaults → YAML file → environment variables
- Google Cloud Healthcare API URL construction: automatically builds Healthcare API URLs from project/location/dataset/store
- Environment variables follow the pattern: `GOOGLE_CLOUD_*`, `VALIDATION_*`, `REQUIRE_AUTHENTICATION`

### Validation Flow

1. **Request Reception**: Proxy receives FHIR request on appropriate endpoint
2. **Resource Extraction**: Extracts FHIR resource(s) from request body
3. **Validation Orchestration**: 
   - Applies field rules from `rules.yaml`
   - For bundles: applies appropriate recipe (transaction vs message)
   - Validates resource counts, references, forbidden resources
4. **Response Handling**: Returns OperationOutcome for validation errors or forwards to upstream server

### Bundle Recipe System

Bundle recipes (`configs/recipes.yaml`) support advanced validation:
- **Resource Requirements**: Min/max counts per resource type
- **Reference Validation**: Ensures required references exist between resources  
- **Forbidden Resources**: Blocks specific resource types in bundles
- **Conditional Rules**: "when X then require Y" logic
- **Message Validation**: Special rules for MessageHeader fields in FHIR messages

Recipe keys use pattern `{bundleType}:{recipeName}` (e.g., `transaction:default`, `message:default`).

### Google Cloud Integration

The proxy integrates with Google Cloud Healthcare API through:
- **Authentication**: Uses Application Default Credentials or service account files
- **URL Construction**: Builds Healthcare API URLs from config: `projects/{project}/locations/{location}/datasets/{dataset}/fhirStores/{store}/fhir`
- **Request Forwarding**: Validates locally then proxies to Healthcare API
- **Fallback**: Can work with any FHIR server via `FHIR_SERVER_URL` or `base_url` config

### Testing Strategy

Tests are organized by component:
- `*_test.go`: Unit tests for individual functions and components
- `enhanced_validation_test.go`: Integration tests for complex validation scenarios
- Tests require loading configuration files, so they reference `../../configs/` paths
- Authentication tests gracefully skip when Google Cloud credentials are unavailable

### Configuration Loading

Configuration loading follows this priority:
1. Default values in code
2. YAML file (`configs/server.yaml`) - optional
3. Environment variables - highest priority

Critical environment variables:
- `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_DATASET_ID`, `GOOGLE_CLOUD_FHIR_STORE_ID`
- `GOOGLE_APPLICATION_CREDENTIALS` for authentication
- `FHIR_SERVER_URL` for non-Google Cloud FHIR servers
- `REQUIRE_AUTHENTICATION=true` to enable authentication

### Route Handling Nuances

Route registration order matters in `SetupRoutes()`:
1. System operations (`/$process-message`, `/metadata`) must be registered first
2. Bundle operations (`/fhir` root endpoint)  
3. Resource operations (`/{resourceType}` patterns) last

This prevents system operations from being caught by the generic `{resourceType}` pattern.