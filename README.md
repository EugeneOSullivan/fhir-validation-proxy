# FHIR Validation Proxy

[![Go CI](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/eugeneosullivan/fhir-validation-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/eugeneosullivan/fhir-validation-proxy)

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
└── internal/
    ├── auth/             # Authentication middleware
    ├── config/           # Configuration management
    ├── proxy/            # FHIR proxy implementation
    └── validator/        # Core validation logic
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
```

### Configuration File

Create `configs/server.yaml`:
```yaml
server:
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

google_cloud:
  project_id: "${GOOGLE_CLOUD_PROJECT}"
  location: "${GOOGLE_CLOUD_LOCATION}"
  dataset_id: "${GOOGLE_CLOUD_DATASET_ID}"
  fhir_store_id: "${GOOGLE_CLOUD_FHIR_STORE_ID}"

validation:
  strict_mode: true
  profile_validation: true

security:
  require_authentication: false
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
```

## Testing

Run all tests:

```sh
go test ./...
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
    dataQuality:
      - field: Patient.identifier
        validation: nhs-number
      - field: Composition.date
        validation: not-future

message:
  default:
    requiredResources:
      - resourceType: MessageHeader
        minCount: 1
        maxCount: 1
    messageValidation:
      - field: eventCoding
        required: true
      - field: source
        required: true
```

### Google Cloud Healthcare API Integration

The proxy seamlessly integrates with Google Cloud Healthcare API:

1. **Authentication**: Uses Application Default Credentials or service account keys
2. **FHIR Store Targeting**: Automatically constructs Healthcare API URLs
3. **Request Forwarding**: Validates locally, then forwards to Google Cloud
4. **Error Handling**: Translates Google Cloud errors to FHIR OperationOutcomes

## Deployment

### Docker

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o fhir-validation-proxy ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/fhir-validation-proxy .
COPY --from=builder /app/configs ./configs
CMD ["./fhir-validation-proxy"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fhir-validation-proxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: fhir-validation-proxy
  template:
    metadata:
      labels:
        app: fhir-validation-proxy
    spec:
      containers:
      - name: fhir-validation-proxy
        image: fhir-validation-proxy:latest
        ports:
        - containerPort: 8080
        env:
        - name: GOOGLE_CLOUD_PROJECT
          value: "your-project"
        - name: GOOGLE_APPLICATION_CREDENTIALS
          value: "/etc/gcp/service-account.json"
        volumeMounts:
        - name: gcp-key
          mountPath: /etc/gcp
      volumes:
      - name: gcp-key
        secret:
          secretName: gcp-service-account
```

---

## License

MIT (or your license here)
