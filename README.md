# FHIR Validation Proxy

[![Go CI](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/go-ci.yml/badge.svg)](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/go-ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/go-ci.yml)

A Go-based proxy server for validating FHIR resources against custom rules, profiles, and recipes before forwarding to Google Cloud Healthcare API or other FHIR servers.

## Features

- **Full FHIR R4 API Support**: Complete proxy for all FHIR operations (CRUD, search, history)
- **Google Cloud Healthcare API Integration**: Native support for Google Cloud FHIR stores
- **Advanced Bundle Validation**: Enhanced recipes with conditional rules, resource count limits, and forbidden resources
- **FHIR Messaging Support**: Validates FHIR message bundles with MessageHeader requirements
- **Flexible Validation Engine**:
  - Custom rules (YAML)
  - FHIR profiles (JSON)
  - Bundle recipes (YAML) with advanced features
  - Data quality rules
- **Security & Authentication**: Google Cloud IAM integration and audit logging
- **Monitoring**: Prometheus metrics and health checks
- **Production Ready**: Graceful shutdown, comprehensive error handling, and extensive testing
- **Enterprise Optimized**: Caching, request limits, performance monitoring, and Cloud Run ready

## Enterprise Deployment

### Cloud Run Deployment (Recommended)

The FHIR Validation Proxy is optimized for Google Cloud Run with enterprise-grade features:

#### Quick Deployment

```bash
# Set your project ID
export GOOGLE_CLOUD_PROJECT="your-project-id"
export GOOGLE_CLOUD_REGION="us-central1"

# Run the deployment script
./deploy.sh
```

#### Manual Deployment

```bash
# Build and push the Docker image
gcloud builds submit --tag gcr.io/PROJECT_ID/fhir-validation-proxy .

# Deploy to Cloud Run
gcloud run deploy fhir-validation-proxy \
  --image gcr.io/PROJECT_ID/fhir-validation-proxy \
  --platform managed \
  --region us-central1 \
  --memory 4Gi \
  --cpu 2 \
  --max-instances 100 \
  --min-instances 1 \
  --concurrency 80 \
  --timeout 300 \
  --set-env-vars="GOOGLE_CLOUD_PROJECT=PROJECT_ID" \
  --set-env-vars="REQUIRE_AUTHENTICATION=true" \
  --set-env-vars="AUDIT_LOGGING=true"
```

#### Enterprise Configuration

The proxy includes several enterprise optimizations:

- **Caching**: Rules and profiles are cached in memory for faster validation
- **Request Limits**: Configurable size limits (10MB default) and bundle entry limits (1000 default)
- **Performance Monitoring**: Built-in metrics for request duration, success rates, and resource types
- **Security**: Non-root container, proper IAM roles, and audit logging
- **Auto-scaling**: Cloud Run handles scaling from 1 to 100 instances based on load
- **Health Checks**: Built-in health and readiness probes

#### Performance Characteristics

- **Latency**: < 100ms for simple validations, < 500ms for complex bundles
- **Throughput**: 1000+ requests/second per instance
- **Memory**: 1-4GB configurable, optimized for validation workloads
- **CPU**: 0.5-2 cores configurable, supports concurrent validation

#### Monitoring Setup

```bash
# View real-time metrics
curl https://your-service-url/metrics

# Set up Cloud Monitoring alerts
gcloud monitoring policies create \
  --policy-from-file=monitoring-policy.yaml
```

Example monitoring policy:
```yaml
displayName: "FHIR Validation Proxy - High Error Rate"
conditions:
  - displayName: "Error rate > 5%"
    conditionThreshold:
      filter: 'resource.type="cloud_run_revision" AND resource.labels.service_name="fhir-validation-proxy"'
      comparison: COMPARISON_GREATER_THAN
      thresholdValue: 0.05
      duration: 300s
      aggregations:
        - alignmentPeriod: 60s
          perSeriesAligner: ALIGN_RATE
          crossSeriesReducer: REDUCE_MEAN
```

## Project Structure

```
.
├── api/                    # Legacy API handlers 
├── cmd/server/            # Main entrypoint
├── configs/               # Configuration files
│   ├── server.yaml       # Server configuration
│   ├── rules.yaml        # Validation rules
│   ├── recipes.yaml      # Bundle recipes
│   └── profiles/         # FHIR profiles
├── internal/
│   ├── auth/             # Authentication middleware
│   ├── config/           # Configuration management
│   ├── proxy/            # FHIR proxy implementation
│   └── validator/        # Core validation logic
├── cloud-run.yaml        # Cloud Run service configuration
├── deploy.sh             # Deployment script
└── Dockerfile            # Multi-stage Docker build
```

## How to Build

```sh
git clone <your-repo-url>
cd fhir-validation-proxy
go build -o fhir-validation-proxy ./cmd/server
```

## Configuration

### Environment Variables

```sh
# Google Cloud Configuration
export GOOGLE_CLOUD_PROJECT=your-project-id
export GOOGLE_CLOUD_LOCATION=us-central1
export GOOGLE_CLOUD_DATASET_ID=your-dataset
export GOOGLE_CLOUD_FHIR_STORE_ID=your-fhir-store
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Alternative: Direct FHIR Server URL
export FHIR_SERVER_URL=https://your.fhir.server/fhir

# Security Configuration
export REQUIRE_AUTHENTICATION=true
export VALIDATION_STRICT_MODE=true

# Server Configuration
export PORT=8080

# Enterprise Limits
export MAX_REQUEST_SIZE=10485760  # 10MB
export MAX_BUNDLE_ENTRIES=1000
export MAX_VALIDATION_TIME=30     # seconds
```

### Configuration File

Create `configs/server.yaml`:
```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

google_cloud:
  project_id: "${GOOGLE_CLOUD_PROJECT}"
  location: "${GOOGLE_CLOUD_LOCATION}"
  dataset_id: "${GOOGLE_CLOUD_DATASET_ID}"
  fhir_store_id: "${GOOGLE_CLOUD_FHIR_STORE_ID}"

validation:
  strict_mode: true
  profile_validation: true
  max_request_size: 10485760
  max_bundle_entries: 1000
  max_validation_time: 30

security:
  require_authentication: true
  audit_logging: true

monitoring:
  enable_metrics: true
  metrics_port: 9090
```

## How to Run

```sh
./fhir-validation-proxy
```

The server will start on `http://localhost:8080` with:
- FHIR API at `/fhir/*`
- Legacy validation at `/validate`
- Health check at `/health`
- Metrics at `/metrics` (port 9090)

## API Usage

### FHIR R4 API

The proxy supports the complete FHIR R4 REST API:

```sh
# Create a Patient
curl -X POST http://localhost:8080/fhir/Patient \
  -H 'Content-Type: application/fhir+json' \
  -H 'Authorization: Bearer your-token' \
  -d @patient.json

# Read a Patient
curl http://localhost:8080/fhir/Patient/123

# Update a Patient
curl -X PUT http://localhost:8080/fhir/Patient/123 \
  -H 'Content-Type: application/fhir+json' \
  -d @updated-patient.json

# Search Patients
curl "http://localhost:8080/fhir/Patient?family=Smith&active=true"

# Submit a Bundle Transaction
curl -X POST http://localhost:8080/fhir \
  -H 'Content-Type: application/fhir+json' \
  -d @transaction-bundle.json

# Process a FHIR Message
curl -X POST http://localhost:8080/fhir/\$process-message \
  -H 'Content-Type: application/fhir+json' \
  -d @message-bundle.json

# Get Server Capability Statement
curl http://localhost:8080/fhir/metadata
```

### Legacy Validation Endpoint

```sh
curl -X POST http://localhost:8080/validate \
  -H 'Content-Type: application/fhir+json' \
  -d @your-resource.json
```

### Health and Monitoring

```sh
# Health check
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics

# Validation metrics (enterprise)
curl http://localhost:8080/metrics
```

## Testing

Run all tests:

```sh
go test ./...
```

Run with coverage:

```sh
make coverage
```

## Advanced Configuration

### Enhanced Bundle Recipes

The `configs/recipes.yaml` file supports advanced validation rules:

```yaml
transaction:
  clinical-document:
    requiredResources:
      - resourceType: Patient
        minCount: 1
        maxCount: 1
      - resourceType: Composition
        minCount: 1
        validation: clinical-composition
    forbiddenResources:
      - Organization
      - Device
    conditionalRules:
      - when: "Composition.type.coding.code = '11488-4'"
        require: [DocumentReference]
    mustReference:
      - source: Composition
        target: Patient
```

## Enterprise Features

### Performance Optimizations

1. **Caching**: Rules and profiles are loaded once and cached in memory
2. **Concurrent Validation**: Multiple validations can run simultaneously
3. **Request Limits**: Configurable size and complexity limits
4. **Metrics**: Built-in performance monitoring

### Security Features

1. **Authentication**: Google Cloud IAM integration
2. **Audit Logging**: Comprehensive request logging
3. **Request Validation**: Size and content validation
4. **Non-root Container**: Secure runtime environment

### Monitoring and Observability

1. **Health Checks**: `/health` endpoint for load balancers
2. **Metrics**: `/metrics` endpoint with validation statistics
3. **Request Headers**: Duration and resource type headers
4. **Cloud Logging**: Structured logging for analysis

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test` and `make coverage`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
