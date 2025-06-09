package api

import (
	"encoding/json"
	"fhir-validation-proxy/internal/validator"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHandler_Valid(t *testing.T) {
	err := validator.LoadProfiles("../configs/profiles")
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}
	err = validator.LoadRules("../configs/rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}
	err = validator.LoadRecipes("../configs/recipes.yaml")
	if err != nil {
		t.Fatalf("Failed to load recipes: %v", err)
	}

	body := `{
	"resourceType": "Patient",
	"meta": {
		"profile": ["https://fhir.nhs.wales/StructureDefinition/DataStandardsWales-Patient"]
	},
	"active": true,
	"gender": "female",
	"birthDate": "1980-01-01",
	"name": [{"family": "Smith"}],
	"address": [{"postalCode": "CF10 1EP"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(body))
	rw := httptest.NewRecorder()

	ValidateHandler(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rw.Code)
	}

	var res map[string]interface{}
	err = json.NewDecoder(rw.Body).Decode(&res)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if res["resourceType"] != "OperationOutcome" {
		t.Errorf("Expected OperationOutcome, got %v", res["resourceType"])
	}

	issues, ok := res["issue"].([]interface{})
	if !ok || len(issues) == 0 {
		t.Errorf("Expected non-empty issue array")
		return
	}

	firstIssue := issues[0].(map[string]interface{})
	if firstIssue["severity"] != "information" {
		t.Errorf("Expected severity=information, got %v", firstIssue["severity"])
	}
}

func TestValidateHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader("{invalid json"))
	rw := httptest.NewRecorder()

	ValidateHandler(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rw.Code)
	}

	var res map[string]interface{}
	_ = json.NewDecoder(rw.Body).Decode(&res)
	if res["resourceType"] != "OperationOutcome" {
		t.Errorf("Expected OperationOutcome for error")
	}
}
