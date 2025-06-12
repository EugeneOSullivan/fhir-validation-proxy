# FHIR Validation Proxy

[![Go CI](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/test.yml/badge.svg)](https://github.com/eugeneosullivan/fhir-validation-proxy/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/eugeneosullivan/fhir-validation-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/eugeneosullivan/fhir-validation-proxy)

A Go-based proxy service for validating FHIR resources against custom rules, profiles, and recipes before forwarding to a FHIR server.

## Features

- Validates FHIR resources (e.g., Patient) using:
  - Custom rules (YAML)
  - FHIR profiles (JSON)
  - Bundle recipes (YAML)
- Returns OperationOutcome for validation errors
- Forwards valid resources to a configured FHIR server
- Easily extensible with new rules and profiles

## Project Structure

```
.
├── api/           # API handlers and tests
├── cmd/           # Entrypoint (main.go)
├── configs/       # Rules, profiles, recipes
├── internal/
│   └── validator/ # Core validation logic
```

## How to Build

```sh
git clone <your-repo-url>
cd fhir-validation-proxy
go build -o fhir-validation-proxy ./cmd/server
```

## How to Run

1. **Set up configuration:**
   - Place your FHIR profiles in `configs/profiles/`
   - Edit `configs/rules.yaml` and `configs/recipes.yaml` as needed

2. **(Optional) Set FHIR server URL:**
   ```sh
   export FHIR_SERVER_URL=https://your.fhir.server/endpoint
   ```

3. **Run the server:**
   ```sh
   ./fhir-validation-proxy
   ```

   The server will start on `http://localhost:8080`.

## API Usage

- **POST /validate**
  - Accepts a FHIR resource (JSON)
  - Returns an OperationOutcome if validation fails, or forwards to the FHIR server if valid

Example:

```sh
curl -X POST http://localhost:8080/validate \
  -H 'Content-Type: application/fhir+json' \
  -d @your-resource.json
```

## Testing

Run all tests:

```sh
go test ./...
```

## Extending

- **Add new rules:** Edit `configs/rules.yaml`
- **Add new profiles:** Place JSON files in `configs/profiles/`
- **Add new recipes:** Edit `configs/recipes.yaml`

## Roadmap / Suggestions

- Add OpenAPI documentation
- Add Dockerfile and CI/CD
- Improve error handling and logging
- Add more comprehensive integration tests

---

## License

MIT (or your license here)
