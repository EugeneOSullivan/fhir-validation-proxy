package api

import (
	"encoding/json"
	"fhir-validation-proxy/internal/validator"
	"io"
	"net/http"
)

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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue": []map[string]interface{}{{
				"severity":    "information",
				"code":        "informational",
				"diagnostics": "Validation successful",
			}},
		})
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue":        issues,
		})
	}
}

func writeOperationOutcome(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{{
			"severity":    "error",
			"code":        "invalid",
			"diagnostics": message,
		}},
	})
}
