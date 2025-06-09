package api

import (
	"encoding/json"
	"fhir-validation-proxy/internal/validator"
	"io"
	"net/http"
)

// ValidateHandler handles FHIR resource validation requests.
func ValidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOperationOutcome(w, http.StatusMethodNotAllowed, "Only POST allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var resource map[string]interface{}
	err = json.Unmarshal(body, &resource)
	if err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result := validator.Validate(resource)

	w.Header().Set("Content-Type", "application/fhir+json")
	if result.Valid {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue": []map[string]interface{}{{
				"severity":    "information",
				"code":        "informational",
				"diagnostics": "Validation successful",
			}},
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		issues := []map[string]interface{}{}
		for _, msg := range result.Errors {
			issues = append(issues, map[string]interface{}{
				"severity":    "error",
				"code":        "invalid",
				"diagnostics": msg,
			})
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue":        issues,
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

func writeOperationOutcome(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{{
			"severity":    "error",
			"code":        "invalid",
			"diagnostics": message,
		}},
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
