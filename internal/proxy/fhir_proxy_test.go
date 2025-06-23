package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fhir-validation-proxy/internal/config"
	"fhir-validation-proxy/internal/validator"
	"github.com/gorilla/mux"
)

func TestFHIRProxy(t *testing.T) {
	// Load test data
	if err := validator.LoadRules("../../configs/rules.yaml"); err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}
	if err := validator.LoadRecipes("../../configs/recipes.yaml"); err != nil {
		t.Fatalf("Failed to load recipes: %v", err)
	}

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			StrictMode: true,
		},
	}

	proxy := NewFHIRProxy(cfg)
	router := mux.NewRouter()
	proxy.SetupRoutes(router)

	t.Run("health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %v", response["status"])
		}
	})

	t.Run("validate endpoint - valid patient", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"active":       true,
			"gender":       "female",
			"birthDate":    "1980-01-01",
			"name": []interface{}{
				map[string]interface{}{"family": "Smith"},
			},
			"address": []interface{}{
				map[string]interface{}{"postalCode": "CF10 1EP"},
			},
		}

		body, _ := json.Marshal(patient)
		req := httptest.NewRequest("POST", "/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["resourceType"] != "OperationOutcome" {
			t.Errorf("Expected OperationOutcome, got %v", response["resourceType"])
		}
	})

	t.Run("validate endpoint - invalid patient", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"active":       false, // Should be true according to rules
			"gender":       "invalid",
		}

		body, _ := json.Marshal(patient)
		req := httptest.NewRequest("POST", "/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["resourceType"] != "OperationOutcome" {
			t.Errorf("Expected OperationOutcome, got %v", response["resourceType"])
		}

		issues, ok := response["issue"].([]interface{})
		if !ok || len(issues) == 0 {
			t.Error("Expected validation issues")
		}
	})

	t.Run("create patient - valid", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Patient",
			"active":       true,
			"gender":       "male",
			"birthDate":    "1990-01-01",
			"name": []interface{}{
				map[string]interface{}{"family": "Doe"},
			},
			"address": []interface{}{
				map[string]interface{}{"postalCode": "SW1A 1AA"},
			},
		}

		body, _ := json.Marshal(patient)
		req := httptest.NewRequest("POST", "/fhir/Patient", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["resourceType"] != "Patient" {
			t.Errorf("Expected Patient, got %v", response["resourceType"])
		}

		if response["id"] == nil {
			t.Error("Expected generated ID")
		}
	})

	t.Run("create patient - resource type mismatch", func(t *testing.T) {
		patient := map[string]interface{}{
			"resourceType": "Observation", // Mismatch with URL
			"status":       "final",
		}

		body, _ := json.Marshal(patient)
		req := httptest.NewRequest("POST", "/fhir/Patient", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}
	})

	t.Run("bundle transaction - valid", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Provenance",
						"id":           "prov1",
						"recorded":     "2023-01-01T00:00:00Z",
						"target": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
					},
				},
			},
		}

		body, _ := json.Marshal(bundle)
		req := httptest.NewRequest("POST", "/fhir", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("bundle transaction - missing required resource", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
				// Missing Provenance resource
			},
		}

		body, _ := json.Marshal(bundle)
		req := httptest.NewRequest("POST", "/fhir", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}
	})

	t.Run("process message - valid", func(t *testing.T) {
		message := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "message",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "MessageHeader",
						"id":           "msg1",
						"eventCoding": map[string]interface{}{
							"system": "http://example.org/events",
							"code":   "patient-created",
						},
						"source": map[string]interface{}{
							"name": "Test System",
						},
						"focus": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
						"target": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
			},
		}

		body, _ := json.Marshal(message)
		req := httptest.NewRequest("POST", "/fhir/$process-message", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Response: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("capability statement", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/fhir/metadata", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["resourceType"] != "CapabilityStatement" {
			t.Errorf("Expected CapabilityStatement, got %v", response["resourceType"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/validate", strings.NewReader("{invalid json"))
		req.Header.Set("Content-Type", "application/fhir+json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/validate", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}